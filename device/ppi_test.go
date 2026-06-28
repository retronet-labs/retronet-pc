package device

import "testing"

func TestPPIPortASource(t *testing.T) {
	p := NewPPI()
	p.DIPSwitches = 0xAA

	p.Out8(0x61, 0x80) // PB7=1 -> Port A mostra i DIP switch
	if got := p.In8(0x60); got != 0xAA {
		t.Errorf("Port A con PB7=1 = %#02x, atteso 0xAA (DIP)", got)
	}
	p.Out8(0x61, 0x00)    // PB7=0 -> Port A mostra la tastiera
	p.KeyboardData = 0x1C // (impostato dopo: alzare PB7 azzera il latch)
	if got := p.In8(0x60); got != 0x1C {
		t.Errorf("Port A con PB7=0 = %#02x, atteso 0x1C (tastiera)", got)
	}
}

func TestPPIPortCNibbleSelect(t *testing.T) {
	p := NewPPI()
	p.DIPSwitches = 0x5C // basso 0xC, alto 0x5
	p.Out8(0x61, 0x00)   // PB3=0 -> nibble basso
	if got := p.In8(0x62) & 0x0F; got != 0x0C {
		t.Errorf("Port C nibble basso = %#x, atteso 0xC", got)
	}
	p.Out8(0x61, 0x08) // PB3=1 -> nibble alto
	if got := p.In8(0x62) & 0x0F; got != 0x05 {
		t.Errorf("Port C nibble alto = %#x, atteso 0x5", got)
	}
}

// Il rilascio del clock tastiera (PB6 0->1) simula il reset: la tastiera invia
// 0xAA (BAT) con IRQ1; alzare PB7 azzera il latch (niente "tasto bloccato").
func TestPPIKeyboardReset(t *testing.T) {
	p := NewPPI()
	irq := 0
	p.IRQ1 = func() { irq++ }

	p.Out8(0x61, 0x00) // PB6=0 (clock basso)
	p.Out8(0x61, 0x40) // PB6 0->1: reset -> BAT
	if irq != 1 {
		t.Fatalf("atteso 1 IRQ1 dal reset tastiera, ottenuti %d", irq)
	}
	if p.KeyboardData != 0xAA {
		t.Fatalf("la tastiera doveva presentare 0xAA, ho %#02x", p.KeyboardData)
	}
	// Con PB7=0 Port A mostra il codice tastiera.
	p.Out8(0x61, 0x40) // PB7=0
	if p.In8(0x60) != 0xAA {
		t.Errorf("Port A doveva valere 0xAA")
	}
	// PB7 0->1: clear del latch.
	p.Out8(0x61, 0xC0)
	if p.KeyboardData != 0 {
		t.Errorf("il clear (PB7) doveva azzerare il latch, ho %#02x", p.KeyboardData)
	}
}

func TestPPISpeaker(t *testing.T) {
	p := NewPPI()
	p.Out8(0x61, 0x03) // PB0+PB1
	if !p.SpeakerOn() {
		t.Error("speaker dovrebbe essere attivo con PB0 e PB1 alti")
	}
	p.Out8(0x61, 0x01)
	if p.SpeakerOn() {
		t.Error("speaker non deve suonare con il solo gate")
	}
}
