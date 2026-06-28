package machine

import (
	"testing"

	"github.com/retronet-labs/retronet-8086/cpu"
	"github.com/retronet-labs/retronet-pc/device"
)

// Un programma in RAM scrive sulla porta diagnostica 0x80 (POST) via OUT.
func TestProgramWritesPostPort(t *testing.T) {
	m := New()
	post := &device.PostCode{}
	m.Map(0x80, 0x80, post)

	m.Mem.LoadRAM(cpu.PhysAddr(0x0000, 0x0100), []byte{
		0xB0, 0x42, // MOV AL,0x42
		0xE6, 0x80, // OUT 0x80,AL
		0xF4, // HLT
	})
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x0000, 0x0100

	if _, err := m.Run(100); err != nil {
		t.Fatal(err)
	}
	if !post.Written || post.Last != 0x42 {
		t.Fatalf("POST = %#02x written=%v, atteso 0x42", post.Last, post.Written)
	}
}

// Un programma scrive nella RAM video (0xB0000) tramite ES e l'MDA lo mostra.
func TestVideoTextOutput(t *testing.T) {
	m := NewXT()
	m.Mem.LoadRAM(cpu.PhysAddr(0x0000, 0x0100), []byte{
		0xB8, 0x00, 0xB0, // MOV AX,0xB000
		0x8E, 0xC0, // MOV ES,AX
		0x26, 0xC6, 0x06, 0x00, 0x00, 0x4F, // MOV byte [es:0x0000],'O'
		0x26, 0xC6, 0x06, 0x02, 0x00, 0x4B, // MOV byte [es:0x0002],'K'
		0xF4, // HLT
	})
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x0000, 0x0100

	for i := 0; i < 20 && !m.CPU.Halted; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	screen := m.Screen()
	if len(screen) < 2 || screen[0] != 'O' || screen[1] != 'K' {
		t.Fatalf("schermo non mostra OK: inizia con %q", screen[:2])
	}
}

// Lettura di un settore floppy attraverso il routing delle porte della macchina:
// programmazione del DMA (0x00-0x0F, pagina 0x81) e del FDC (0x3F0-0x3F7), con il
// settore che finisce in memoria via DMA canale 2.
func TestMachineFloppyRead(t *testing.T) {
	m := NewXT()
	img := make([]byte, 360*1024)
	for i := 0; i < 512; i++ {
		img[i] = byte(i)
	}
	if err := m.LoadFloppy(img); err != nil {
		t.Fatal(err)
	}

	// DMA canale 2 -> indirizzo fisico 0x00500, 512 byte.
	m.IO.Out8(0x0A, 0x06)
	m.IO.Out8(0x0C, 0x00)
	m.IO.Out8(0x04, 0x00)
	m.IO.Out8(0x04, 0x05)
	m.IO.Out8(0x81, 0x00)
	m.IO.Out8(0x05, 0xFF)
	m.IO.Out8(0x05, 0x01)
	m.IO.Out8(0x0B, 0x46)
	m.IO.Out8(0x0A, 0x02)

	m.IO.Out8(0x3F2, 0x1C) // DOR: motore, DMA/IRQ
	for _, b := range []byte{0x06, 0x00, 0x00, 0x00, 0x01, 0x02, 0x01, 0x2A, 0xFF} {
		m.IO.Out8(0x3F5, b) // Read Data + parametri
	}
	st0 := m.IO.In8(0x3F5)
	for i := 1; i < 7; i++ {
		m.IO.In8(0x3F5)
	}
	if st0&0xC0 != 0 {
		t.Fatalf("ST0 anomalo: %#02x", st0)
	}
	for i := 0; i < 512; i++ {
		if got := m.Mem.Read8(0x0500 + uint32(i)); got != img[i] {
			t.Fatalf("byte %d = %#02x, atteso %#02x", i, got, img[i])
		}
	}
}

// All'accensione la CPU parte dal reset vector 0xFFFF0, dove va il BIOS in ROM.
// Qui un mini-"BIOS" in ROM scrive un codice POST e si ferma.
func TestBootFromResetVector(t *testing.T) {
	m := New()
	post := &device.PostCode{}
	m.Map(0x80, 0x80, post)

	m.Mem.LoadROM(cpu.PhysAddr(0xFFFF, 0x0000), []byte{
		0xB0, 0xAA, // MOV AL,0xAA
		0xE6, 0x80, // OUT 0x80,AL
		0xF4, // HLT
	})
	// La CPU appena creata e' gia' al reset (CS=0xFFFF, IP=0).
	if _, err := m.Run(100); err != nil {
		t.Fatal(err)
	}
	if post.Last != 0xAA {
		t.Fatalf("POST dal BIOS in ROM = %#02x, atteso 0xAA", post.Last)
	}
	// La ROM non deve essere scrivibile dal programma.
	m.Mem.Write8(cpu.PhysAddr(0xFFFF, 0x0000), 0x00)
	if m.Mem.Read8(cpu.PhysAddr(0xFFFF, 0x0000)) != 0xB0 {
		t.Errorf("la ROM del BIOS e' stata sovrascritta")
	}
}
