package machine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/retronet-labs/retronet-8086/cpu"
)

// findAsset cerca un asset esterno non versionato (ROM/immagine): prima la
// variabile d'ambiente, poi alcuni percorsi candidati relativi al repo. Restituisce
// "" se non lo trova.
func findAsset(env string, candidates ...string) string {
	if p := os.Getenv(env); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// TestBootFreeDOSFloppy e' un test d'integrazione del percorso d'avvio completo:
// POST del BIOS -> boot sector FAT12 -> caricamento di KERNEL.SYS (UPX) via
// FDC/DMA -> decompressione -> kernel + FreeCom fino al prompt. Blinda in
// particolare la lettura floppy multi-settore legata al Terminal Count del DMA
// (vedi device/fdc.go), il cui difetto bloccava l'avvio.
//
// Gli asset (BIOS GLaBIOS e immagine floppy FreeDOS) NON sono nel repo: il test si
// salta se non li trova. Si indicano con RETRONET_BIOS / RETRONET_FLOPPY, oppure si
// lasciano nella radice del repo (../).
func TestBootFreeDOSFloppy(t *testing.T) {
	bios := findAsset("RETRONET_BIOS",
		filepath.Join("..", "GLABIOS_0.4.2_8X.ROM"), "GLABIOS_0.4.2_8X.ROM")
	floppy := findAsset("RETRONET_FLOPPY",
		filepath.Join("..", "x86BOOT.img"), "x86BOOT.img")
	if bios == "" || floppy == "" {
		t.Skip("imposta RETRONET_BIOS e RETRONET_FLOPPY (BIOS GLaBIOS + floppy FreeDOS) per eseguire il test d'avvio")
	}

	rom, err := os.ReadFile(bios)
	if err != nil {
		t.Fatalf("BIOS: %v", err)
	}
	img, err := os.ReadFile(floppy)
	if err != nil {
		t.Fatalf("floppy: %v", err)
	}

	m := NewXT()
	m.CPU.SetALU(cpu.Native)
	m.UseCGA()
	m.LoadBIOS(rom)
	if err := m.LoadFloppy(img); err != nil {
		t.Fatalf("LoadFloppy: %v", err)
	}

	// 80M passi bastano abbondantemente a superare POST, caricamento e
	// decompressione del kernel e ad arrivare a FreeCom (osservato a ~60M).
	if _, err := m.Run(80_000_000); err != nil {
		t.Fatalf("Run: %v (ultimo POST %02X)", err, m.Post.Last)
	}

	screen := m.Screen()
	if !strings.Contains(screen, "FreeCom") && !strings.Contains(screen, "FreeDOS") {
		t.Fatalf("l'avvio non ha raggiunto FreeDOS/FreeCom (ultimo POST %02X). Schermo:\n%s",
			m.Post.Last, screen)
	}
}
