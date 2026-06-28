package memory

import "testing"

func TestROMWriteProtected(t *testing.T) {
	b := New()
	b.LoadROM(0xFE000, []byte{0x11, 0x22})
	if b.Read8(0xFE000) != 0x11 || b.Read8(0xFE001) != 0x22 {
		t.Fatalf("ROM non caricata: %#02x %#02x", b.Read8(0xFE000), b.Read8(0xFE001))
	}
	b.Write8(0xFE000, 0xFF) // deve essere ignorata
	if b.Read8(0xFE000) != 0x11 {
		t.Errorf("scrittura in ROM non ignorata: %#02x", b.Read8(0xFE000))
	}
}

func TestRAMWritable(t *testing.T) {
	b := New()
	b.Write8(0x00500, 0xAB)
	if b.Read8(0x00500) != 0xAB {
		t.Errorf("RAM non scrivibile: %#02x", b.Read8(0x00500))
	}
}

func TestAddressWrap(t *testing.T) {
	b := New()
	b.Write8(Size, 0x7E) // 0x100000 -> wrap a 0
	if b.Read8(0) != 0x7E {
		t.Errorf("wrap a 1 MB errato: %#02x", b.Read8(0))
	}
}
