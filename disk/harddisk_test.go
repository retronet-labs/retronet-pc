package disk

import (
	"bytes"
	"testing"
)

func TestHardDiskGeometryAndRoundTrip(t *testing.T) {
	const sectors = 16 * 63 * 200 // 200 cilindri logici
	hd := NewHardDisk(NewMemDisk(sectors), sectors)

	geo := hd.Geometry()
	if geo.Heads != 16 || geo.Sectors != 63 || geo.Cylinders != 200 {
		t.Fatalf("geometria = %dx%dx%d, attesa 200x16x63", geo.Cylinders, geo.Heads, geo.Sectors)
	}
	if hd.Sectors() != sectors {
		t.Fatalf("settori = %d, attesi %d", hd.Sectors(), sectors)
	}

	want := bytes.Repeat([]byte{0xAB, 0xCD}, 256) // 1 settore
	if err := hd.WriteLBA(100, want); err != nil {
		t.Fatalf("WriteLBA: %v", err)
	}
	got, err := hd.ReadLBA(100, 1)
	if err != nil {
		t.Fatalf("ReadLBA: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("round-trip LBA fallito")
	}
}

func TestHardDiskOutOfRange(t *testing.T) {
	hd := NewHardDisk(NewMemDisk(64), 64)
	if _, err := hd.ReadLBA(64, 1); err == nil {
		t.Error("lettura oltre l'ultimo settore doveva fallire")
	}
	if err := hd.WriteLBA(63, make([]byte, 2*SectorSize)); err == nil {
		t.Error("scrittura a cavallo della fine doveva fallire")
	}
}

func TestHardDiskCylinderCap(t *testing.T) {
	// Un disco enorme limita i cilindri logici al massimo ATA (16383).
	const sectors = 16 * 63 * 20000
	hd := NewHardDisk(NewMemDisk(8), sectors) // backing piccolo: testiamo solo la geometria
	if c := hd.Geometry().Cylinders; c != 16383 {
		t.Errorf("cilindri = %d, attesi 16383 (cap ATA)", c)
	}
}
