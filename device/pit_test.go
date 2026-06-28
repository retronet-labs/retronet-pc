package device

import "testing"

func TestPITGeneratesIRQ0(t *testing.T) {
	pit := NewPIT()
	ticks := 0
	pit.IRQ0 = func() { ticks++ }

	// Contatore 0, access LSB/MSB, modo 3, ricarica 4.
	pit.Out8(0x43, 0x36) // 00 11 011 0: sel0, LSB/MSB, modo3
	pit.Out8(0x40, 0x04) // LSB
	pit.Out8(0x40, 0x00) // MSB -> ricarica 4

	pit.Tick(10) // 10 colpi su periodo 4 -> 2 azzeramenti
	if ticks == 0 {
		t.Fatal("IRQ0 non generato dal contatore 0")
	}
}

func TestPITReloadZeroIs65536(t *testing.T) {
	pit := NewPIT()
	fired := false
	pit.IRQ0 = func() { fired = true }
	pit.Out8(0x43, 0x36)
	pit.Out8(0x40, 0x00)
	pit.Out8(0x40, 0x00) // ricarica 0 = 65536
	pit.Tick(65536)
	if !fired {
		t.Error("con ricarica 0 (65536) un periodo intero deve generare un tick")
	}
	pit.Tick(1) // pochi colpi: nessun nuovo azzeramento atteso a breve
}

func TestPITReadBackLatch(t *testing.T) {
	pit := NewPIT()
	pit.Out8(0x43, 0x34) // sel0, LSB/MSB, modo2
	pit.Out8(0x40, 0x00)
	pit.Out8(0x40, 0x10) // ricarica 0x1000
	pit.Tick(0x100)      // count ~ 0x0F00
	pit.Out8(0x43, 0x00) // latch del contatore 0
	lo := pit.In8(0x40)
	hi := pit.In8(0x40)
	val := uint16(lo) | uint16(hi)<<8
	if val == 0 || val > 0x1000 {
		t.Errorf("valore latchato fuori range: %#04x", val)
	}
}
