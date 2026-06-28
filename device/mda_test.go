package device

import "strings"

import "testing"

// fakeMem e' un memReader minimale per i test (memoria piatta da 1 MB).
type fakeMem struct{ data [1 << 20]byte }

func (f *fakeMem) Read8(addr uint32) byte    { return f.data[addr&0xFFFFF] }
func (f *fakeMem) write(addr uint32, v byte) { f.data[addr&0xFFFFF] = v }

// scrive una stringa (carattere + attributo 0x07) a partire dalla cella offset.
func (f *fakeMem) writeText(base uint32, cell int, s string) {
	for i := 0; i < len(s); i++ {
		f.write(base+uint32(cell+i)*2, s[i])
		f.write(base+uint32(cell+i)*2+1, 0x07)
	}
}

func TestMDARenderText(t *testing.T) {
	m := NewMDA()
	mem := &fakeMem{}
	mem.writeText(m.Base, 0, "HELLO")        // riga 0
	mem.writeText(m.Base, 80, "RetroNet PC") // riga 1

	screen := m.Render(mem)
	lines := strings.Split(screen, "\n")
	if len(lines) < 2 {
		t.Fatalf("schermo troppo corto: %d righe", len(lines))
	}
	if !strings.HasPrefix(lines[0], "HELLO") {
		t.Errorf("riga 0 = %q, attesa che inizi con HELLO", lines[0][:10])
	}
	if !strings.HasPrefix(lines[1], "RetroNet PC") {
		t.Errorf("riga 1 = %q", lines[1][:12])
	}
	// La riga 0 deve essere lunga 80 colonne.
	if len(lines[0]) != 80 {
		t.Errorf("colonne riga 0 = %d, attese 80", len(lines[0]))
	}
}

func TestMDACrtcAndStatus(t *testing.T) {
	m := NewMDA()
	// Posiziona il cursore (R14/R15) a cella 0x0123.
	m.Out8(0x3B4, 14)
	m.Out8(0x3B5, 0x01)
	m.Out8(0x3B4, 15)
	m.Out8(0x3B5, 0x23)
	if m.CursorOffset() != 0x0123 {
		t.Errorf("cursore = %#x, atteso 0x0123", m.CursorOffset())
	}
	// Lo stato deve alternare i bit di retrace a letture successive.
	s1 := m.In8(0x3BA)
	s2 := m.In8(0x3BA)
	if s1 == s2 {
		t.Errorf("lo stato 0x3BA non alterna: %#02x", s1)
	}
}

func TestMDAStartAddressScroll(t *testing.T) {
	m := NewMDA()
	mem := &fakeMem{}
	mem.writeText(m.Base, 80, "SECONDA") // testo nella cella 80
	// Imposta start address a 80 (R12/R13): la riga 0 mostra ora la "seconda" riga.
	m.Out8(0x3B4, 12)
	m.Out8(0x3B5, 0x00)
	m.Out8(0x3B4, 13)
	m.Out8(0x3B5, 80)
	screen := m.Render(mem)
	if !strings.HasPrefix(screen, "SECONDA") {
		t.Errorf("con start=80 la riga 0 dovrebbe mostrare SECONDA, ho: %q", screen[:10])
	}
}

func TestCGARenderAndPorts(t *testing.T) {
	c := NewCGA()
	if c.Base != 0xB8000 {
		t.Fatalf("base CGA = %#X, attesa 0xB8000", c.Base)
	}
	mem := &fakeMem{}
	mem.writeText(c.Base, 0, "CGA OK")
	// Le porte CGA sono a 0x3D4-0x3DA.
	c.Out8(0x3D4, 14)
	c.Out8(0x3D5, 0x00)
	c.Out8(0x3D4, 15)
	c.Out8(0x3D5, 0x05)
	if c.CursorOffset() != 0x0005 {
		t.Errorf("cursore CGA = %#x", c.CursorOffset())
	}
	screen := c.Render(mem)
	if !strings.HasPrefix(screen, "CGA OK") {
		t.Errorf("render CGA = %q", screen[:6])
	}
	s1 := c.In8(0x3DA)
	if c.In8(0x3DA) == s1 {
		t.Errorf("stato CGA 0x3DA non alterna")
	}
}
