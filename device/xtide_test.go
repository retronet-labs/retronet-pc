package device

import (
	"testing"

	"github.com/retronet-labs/retronet-pc/disk"
)

const xtBase = 0x300

func newTestXTIDE(sectors uint32) *XTIDE {
	x := NewXTIDE()
	x.ATA.Disk = disk.NewHardDisk(disk.NewMemDisk(sectors), sectors)
	return x
}

// writeWord/readWord seguono il protocollo della rev 1: il byte alto passa dal
// latch (porta base+8), il byte basso dalla porta dati (base+0) che commuta il
// trasferimento a 16 bit; in lettura base+0 cattura il byte alto nel latch.
func writeWord(x *XTIDE, w uint16) {
	x.Out8(xtBase+8, byte(w>>8))
	x.Out8(xtBase+0, byte(w))
}

func readWord(x *XTIDE) uint16 {
	lo := x.In8(xtBase + 0)
	hi := x.In8(xtBase + 8)
	return uint16(lo) | uint16(hi)<<8
}

func setLBA(x *XTIDE, lba uint32, count byte) {
	x.Out8(xtBase+2, count)
	x.Out8(xtBase+3, byte(lba))
	x.Out8(xtBase+4, byte(lba>>8))
	x.Out8(xtBase+5, byte(lba>>16))
	x.Out8(xtBase+6, 0xE0|byte(lba>>24)&0x0F) // LBA, master
}

// Scrittura e rilettura di un settore attraverso le porte della scheda: verifica
// la traduzione 8<->16 bit (latch) e il percorso ATA WRITE/READ SECTORS.
func TestXTIDEWriteReadSector(t *testing.T) {
	x := newTestXTIDE(2048) // 1 MB

	want := make([]byte, 512)
	for i := range want {
		want[i] = byte(i*7 + 3)
	}

	setLBA(x, 5, 1)
	x.Out8(xtBase+7, 0x30) // WRITE SECTORS
	if x.In8(xtBase+7)&ataDRQ == 0 {
		t.Fatal("dopo WRITE SECTORS manca DRQ")
	}
	for i := 0; i < 512; i += 2 {
		writeWord(x, uint16(want[i])|uint16(want[i+1])<<8)
	}

	setLBA(x, 5, 1)
	x.Out8(xtBase+7, 0x20) // READ SECTORS
	if x.In8(xtBase+7)&ataDRQ == 0 {
		t.Fatal("dopo READ SECTORS manca DRQ")
	}
	got := make([]byte, 512)
	for i := 0; i < 512; i += 2 {
		w := readWord(x)
		got[i], got[i+1] = byte(w), byte(w>>8)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d riletto = %#02x, atteso %#02x", i, got[i], want[i])
		}
	}
	if x.In8(xtBase+7)&ataDRQ != 0 {
		t.Error("a fine settore DRQ doveva azzerarsi")
	}
}

// IDENTIFY DEVICE deve restituire una geometria coerente col disco e il marcatore
// di disco fisso, leggendo le parole tramite il latch.
func TestXTIDEIdentify(t *testing.T) {
	const sectors = 16 * 63 * 100 // 100 cilindri logici
	x := newTestXTIDE(sectors)

	x.Out8(xtBase+6, 0xA0)   // seleziona master
	x.Out8(xtBase+7, 0xEC)   // IDENTIFY DEVICE
	w := make([]uint16, 256) // 256 parole
	for i := range w {
		w[i] = readWord(x)
	}
	if w[0] != 0x0040 {
		t.Errorf("parola 0 (config) = %#04x, atteso 0x0040", w[0])
	}
	if w[3] != 16 {
		t.Errorf("testine = %d, attese 16", w[3])
	}
	if w[6] != 63 {
		t.Errorf("settori/traccia = %d, attesi 63", w[6])
	}
	if w[49]&0x0200 == 0 {
		t.Error("IDENTIFY deve dichiarare il supporto LBA (parola 49 bit 9)")
	}
	total := uint32(w[60]) | uint32(w[61])<<8<<8
	if total != sectors {
		t.Errorf("settori LBA totali = %d, attesi %d", total, sectors)
	}
}

// Selezionando lo slave (assente) lo stato si legge 0: il BIOS lo interpreta come
// drive non presente.
func TestXTIDESlaveAbsent(t *testing.T) {
	x := newTestXTIDE(2048)
	x.Out8(xtBase+6, 0xB0) // seleziona slave (bit4=1)
	if s := x.In8(xtBase + 7); s != 0 {
		t.Errorf("stato dello slave assente = %#02x, atteso 0", s)
	}
	x.Out8(xtBase+6, 0xA0) // master presente
	if s := x.In8(xtBase + 7); s&ataDRDY == 0 {
		t.Errorf("stato del master = %#02x, atteso DRDY", s)
	}
}
