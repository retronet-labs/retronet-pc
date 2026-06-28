package device

import "testing"

// initXT programma il PIC come fa il BIOS dell'XT: base vettori 0x08, singolo,
// con ICW4, e tutte le linee smascherate.
func initXT(p *PIC) {
	p.Out8(0x20, 0x13) // ICW1: bit4=1, SNGL=1, IC4=1
	p.Out8(0x21, 0x08) // ICW2: base 0x08
	p.Out8(0x21, 0x01) // ICW4: 8086 mode, no auto-EOI
	p.Out8(0x21, 0x00) // OCW1: nessuna maschera
}

func TestPICVectorBaseAndAcknowledge(t *testing.T) {
	p := NewPIC()
	initXT(p)
	p.RaiseIRQ(0)
	if !p.Pending() {
		t.Fatal("IRQ0 dovrebbe essere pronto")
	}
	if v := p.Acknowledge(); v != 0x08 {
		t.Fatalf("vettore IRQ0 = %#02x, atteso 0x08", v)
	}
	// Dopo il riconoscimento, finche' non arriva l'EOI, l'ISR resta occupato.
	p.RaiseIRQ(0)
	if p.Pending() {
		t.Error("IRQ0 di pari priorita' non deve passare mentre e' in servizio")
	}
	p.Out8(0x20, 0x20) // EOI non specifico
	if !p.Pending() {
		t.Error("dopo l'EOI il nuovo IRQ0 deve poter passare")
	}
}

func TestPICMask(t *testing.T) {
	p := NewPIC()
	initXT(p)
	p.Out8(0x21, 0x01) // maschera IRQ0
	p.RaiseIRQ(0)
	if p.Pending() {
		t.Error("IRQ0 mascherato non deve essere pronto")
	}
	if p.In8(0x21) != 0x01 {
		t.Errorf("IMR letto = %#02x, atteso 0x01", p.In8(0x21))
	}
}

func TestPICPriority(t *testing.T) {
	p := NewPIC()
	initXT(p)
	p.RaiseIRQ(3)
	p.RaiseIRQ(1)
	// IRQ1 ha priorita' piu' alta di IRQ3.
	if v := p.Acknowledge(); v != 0x09 {
		t.Fatalf("atteso IRQ1 (0x09), ottenuto %#02x", v)
	}
	// IRQ3 e' bloccato finche' IRQ1 e' in servizio (priorita' piu' bassa).
	if p.Pending() {
		t.Error("IRQ3 non deve passare mentre IRQ1 e' in servizio")
	}
	p.Out8(0x20, 0x20) // EOI di IRQ1
	if v := p.Acknowledge(); v != 0x0B {
		t.Fatalf("dopo EOI atteso IRQ3 (0x0B), ottenuto %#02x", v)
	}
}

func TestPICAutoEOI(t *testing.T) {
	p := NewPIC()
	p.Out8(0x20, 0x13) // ICW1 con IC4
	p.Out8(0x21, 0x08) // ICW2
	p.Out8(0x21, 0x03) // ICW4: bit1=1 Auto-EOI
	p.Out8(0x21, 0x00) // OCW1
	p.RaiseIRQ(2)
	p.Acknowledge()
	// In Auto-EOI l'ISR non resta occupato: un nuovo IRQ2 passa subito.
	p.RaiseIRQ(2)
	if !p.Pending() {
		t.Error("in Auto-EOI il nuovo IRQ2 deve passare senza EOI")
	}
}
