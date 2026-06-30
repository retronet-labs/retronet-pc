package machine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/retronet-labs/retronet-8086/cpu"
	"github.com/retronet-labs/retronet-pc/disk"
)

// hddBootSector costruisce un settore di boot da 512 byte che stampa un messaggio
// con INT 10h e poi cicla. Equivale a bootok.rom, con la firma 0x55AA.
func hddBootSector(msg string) []byte {
	b := make([]byte, 512)
	copy(b, []byte{
		0x31, 0xC0, // xor ax,ax
		0x8E, 0xD8, // mov ds,ax
		0xBE, 0x16, 0x7C, // mov si,0x7C16 (stringa a offset 0x16)
		0xB4, 0x0E, // mov ah,0x0E
		0xAC,       // lodsb
		0x3C, 0x00, // cmp al,0
		0x74, 0x05, // je +5 -> 0x13
		0xCD, 0x10, // int 0x10
		0xE9, 0xF6, 0xFF, // jmp -0x0A -> lodsb
		0xE9, 0xFD, 0xFF, // jmp -3 (ciclo)
	})
	copy(b[0x16:], append([]byte(msg), 0))
	b[510], b[511] = 0x55, 0xAA
	return b
}

// TestBootHardDiskXTIDE verifica l'intera catena del disco fisso: il POST del BIOS
// scandisce l'option ROM XTIDE a 0xC8000 -> XTIDE Universal BIOS aggancia INT 13h e
// rileva il disco -> INT 19h avvia dal disco (0x80) leggendo LBA 0 a 0x7C00 -> il
// settore di boot stampa il messaggio. Senza floppy l'avvio va al disco fisso.
//
// Asset non versionati: BIOS GLaBIOS e option ROM XTIDE (ide_xt.bin, rev 1, IO
// 0x300, ROM 0xC800). Si indicano con RETRONET_BIOS / RETRONET_XTIDE_BIOS oppure si
// lasciano nella radice del repo (../). Il test si salta se assenti.
func TestBootHardDiskXTIDE(t *testing.T) {
	bios := findAsset("RETRONET_BIOS",
		filepath.Join("..", "GLABIOS_0.4.2_8X.ROM"), "GLABIOS_0.4.2_8X.ROM")
	xtideROM := findAsset("RETRONET_XTIDE_BIOS",
		filepath.Join("..", "ide_xt.bin"), "ide_xt.bin")
	if bios == "" || xtideROM == "" {
		t.Skip("imposta RETRONET_BIOS e RETRONET_XTIDE_BIOS (GLaBIOS + option ROM XTIDE) per il test del disco fisso")
	}
	rom, err := os.ReadFile(bios)
	if err != nil {
		t.Fatalf("BIOS: %v", err)
	}
	orom, err := os.ReadFile(xtideROM)
	if err != nil {
		t.Fatalf("option ROM: %v", err)
	}

	m := NewXT()
	m.CPU.SetALU(cpu.Native)
	m.UseCGA()
	m.LoadBIOS(rom)

	const sectors = 8192 // 4 MB
	md := disk.NewMemDisk(sectors)
	copy(md.Data, hddBootSector("RETRONET XT-IDE HDD BOOT OK"))
	m.AttachHardDisk(disk.NewHardDisk(md, sectors))
	m.LoadOptionROM(0xC8000, orom)
	// Nessun floppy: l'avvio va al disco fisso.

	if _, err := m.Run(80_000_000); err != nil {
		t.Fatalf("Run: %v (POST %02X)", err, m.Post.Last)
	}
	screen := m.Screen()
	if !strings.Contains(screen, "BOOT OK") {
		t.Fatalf("avvio da disco fisso non riuscito (POST %02X). Schermo:\n%s", m.Post.Last, screen)
	}
}
