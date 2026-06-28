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
