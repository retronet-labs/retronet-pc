package machine

import (
	"testing"

	"github.com/retronet-labs/retronet-8086/cpu"
)

// TestTimerInterruptServiced verifica il percorso completo degli interrupt
// hardware: il contatore 0 del PIT alza IRQ0, il PIC lo propone, la CPU (con IF
// abilitato) lo riconosce e salta al gestore, che invia l'EOI e ritorna con IRET.
// Il gestore incrementa un contatore in memoria a ogni tick.
func TestTimerInterruptServiced(t *testing.T) {
	m := NewXT()

	// Programma il PIC come il BIOS: base 0x08, singolo, con ICW4; smaschera solo IRQ0.
	m.IO.Out8(0x20, 0x13) // ICW1
	m.IO.Out8(0x21, 0x08) // ICW2: IRQ0 -> INT 08h
	m.IO.Out8(0x21, 0x01) // ICW4
	m.IO.Out8(0x21, 0xFE) // OCW1: maschera tutto tranne IRQ0

	// Programma il PIT: contatore 0, modo 3, ricarica 2 (tick frequente per il test).
	m.IO.Out8(0x43, 0x36)
	m.IO.Out8(0x40, 0x02)
	m.IO.Out8(0x40, 0x00)

	// Vettore 8 (IRQ0): gestore a 0000:0400.
	m.Mem.LoadRAM(8*4, []byte{0x00, 0x04, 0x00, 0x00})

	// Gestore IRQ0 a 0000:0400: EOI al PIC, incrementa [0x0500], IRET.
	m.Mem.LoadRAM(cpu.PhysAddr(0x0000, 0x0400), []byte{
		0xB0, 0x20, // MOV AL,0x20
		0xE6, 0x20, // OUT 0x20,AL  (EOI non specifico)
		0xFE, 0x06, 0x00, 0x05, // INC byte [0x0500]
		0xCF, // IRET
	})

	// Programma principale a 0000:0100: abilita gli interrupt e cicla in HLT.
	m.Mem.LoadRAM(cpu.PhysAddr(0x0000, 0x0100), []byte{
		0xFB,       // STI
		0xF4,       // hlt:  HLT
		0xEB, 0xFD, // JMP hlt
	})
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x0000, 0x0100
	m.CPU.Seg[cpu.SS], m.CPU.Regs[cpu.SP] = 0x0000, 0xFFFE

	if _, err := m.Run(2000); err != nil {
		t.Fatalf("esecuzione fallita: %v", err)
	}

	count := m.Mem.Read8(cpu.PhysAddr(0x0000, 0x0500))
	if count == 0 {
		t.Fatal("il gestore IRQ0 non e' mai stato eseguito")
	}
	t.Logf("il timer ha generato %d interrupt serviti", count)
}

// Senza STI (IF=0) l'IRQ0 non deve essere servito.
func TestInterruptMaskedWhenIFClear(t *testing.T) {
	m := NewXT()
	m.IO.Out8(0x20, 0x13)
	m.IO.Out8(0x21, 0x08)
	m.IO.Out8(0x21, 0x01)
	m.IO.Out8(0x21, 0xFE)
	m.IO.Out8(0x43, 0x36)
	m.IO.Out8(0x40, 0x02)
	m.IO.Out8(0x40, 0x00)
	m.Mem.LoadRAM(8*4, []byte{0x00, 0x04, 0x00, 0x00})
	m.Mem.LoadRAM(cpu.PhysAddr(0x0000, 0x0400), []byte{0xFE, 0x06, 0x00, 0x05, 0xCF})

	// Principale: CLI ed un ciclo che non tocca IF.
	m.Mem.LoadRAM(cpu.PhysAddr(0x0000, 0x0100), []byte{
		0xFA,       // CLI
		0x90,       // nop:  NOP
		0xEB, 0xFD, // JMP nop
	})
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x0000, 0x0100
	m.CPU.Seg[cpu.SS], m.CPU.Regs[cpu.SP] = 0x0000, 0xFFFE

	m.Run(500)
	if m.Mem.Read8(cpu.PhysAddr(0x0000, 0x0500)) != 0 {
		t.Fatal("con IF=0 l'IRQ0 non doveva essere servito")
	}
}
