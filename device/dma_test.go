package device

import "testing"

// Write8 completa fakeMem (definito in mda_test.go) come memWriter per il DMA.
func (f *fakeMem) Write8(addr uint32, v byte) { f.write(addr, v) }

func TestDMATransferToMemory(t *testing.T) {
	d := NewDMA()
	mem := &fakeMem{}

	// Programma il canale 2: indirizzo 0x0100, pagina 0x02 (base 0x20100),
	// conteggio 3 (= 4 byte).
	d.Out8(0x0C, 0)    // azzera il flip-flop
	d.Out8(0x04, 0x00) // addr basso
	d.Out8(0x04, 0x01) // addr alto
	d.Out8(0x81, 0x02) // pagina del canale 2
	d.Out8(0x05, 0x03) // count basso
	d.Out8(0x05, 0x00) // count alto

	d.TransferToMemory(2, mem, []byte{0xDE, 0xAD, 0xBE, 0xEF})

	base := uint32(0x20100)
	for i, v := range []byte{0xDE, 0xAD, 0xBE, 0xEF} {
		if got := mem.Read8(base + uint32(i)); got != v {
			t.Errorf("mem[%#x] = %#02x, atteso %#02x", base+uint32(i), got, v)
		}
	}
}

// Il refresh DRAM (canale 0) deve accendere il bit Terminal Count TC0 nello stato
// dopo un giro completo del conteggio; la lettura dello stato azzera i bit TC.
func TestDMARefreshTerminalCount(t *testing.T) {
	d := NewDMA()
	// Conteggio canale 0 = 2 (= 3 cicli per il TC).
	d.Out8(0x0C, 0)
	d.Out8(0x01, 0x02)
	d.Out8(0x01, 0x00)
	d.RefreshCycle() // 2 -> 1
	d.RefreshCycle() // 1 -> 0
	if d.In8(0x08)&0x01 != 0 {
		t.Error("TC0 non doveva essere ancora acceso")
	}
	d.RefreshCycle() // 0 -> underflow: TC0
	st := d.In8(0x08)
	if st&0x01 == 0 {
		t.Fatalf("TC0 doveva essere acceso, stato=%#02x", st)
	}
	if d.In8(0x08)&0x01 != 0 {
		t.Error("la lettura dello stato deve azzerare TC0")
	}
}

func TestDMATransferFromMemory(t *testing.T) {
	d := NewDMA()
	mem := &fakeMem{}
	for i, v := range []byte{0x11, 0x22, 0x33} {
		mem.write(0x30200+uint32(i), v)
	}
	d.Out8(0x0C, 0)
	d.Out8(0x06, 0x00) // addr canale 3 (porta 0x06)
	d.Out8(0x06, 0x02) // -> 0x0200
	d.Out8(0x82, 0x03) // pagina canale 3
	d.Out8(0x07, 0x02) // count canale 3 (porta 0x07): 2 (= 3 byte)
	d.Out8(0x07, 0x00)
	out := d.TransferFromMemory(3, mem, 3)
	if string(out) != "\x11\x22\x33" {
		t.Errorf("letti %v, attesi 11 22 33", out)
	}
}
