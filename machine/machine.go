// Package machine assembla un IBM PC/XT: collega la CPU 8086/8088 di
// retronet-8086 al bus di memoria mappato e al dispatcher di I/O, su cui si
// innestano le periferiche.
package machine

import (
	"github.com/retronet-labs/retronet-8086/cpu"
	"github.com/retronet-labs/retronet-pc/io"
	"github.com/retronet-labs/retronet-pc/memory"
)

// Machine e' un PC/XT: CPU, memoria e bus I/O. Le periferiche si collegano con
// IO.Map e (per il video) leggendo la memoria video dal Bus.
type Machine struct {
	CPU *cpu.CPU8086
	Mem *memory.Bus
	IO  *io.Ports
}

// New crea una macchina con CPU al profilo 8088 (default), memoria da 1 MB e bus
// I/O vuoto. Dopo il reset la CPU parte dal vettore 0xFFFF0, dove va caricato il
// BIOS con Mem.LoadROM.
func New() *Machine {
	m := &Machine{
		Mem: memory.New(),
		IO:  io.New(),
	}
	c := cpu.NewCPU8086()
	c.Mem = m.Mem
	c.IO = m.IO
	m.CPU = c
	return m
}

// Map collega una periferica a un intervallo di porte I/O.
func (m *Machine) Map(lo, hi uint16, dev io.Device) { m.IO.Map(lo, hi, dev) }

// Step esegue una singola istruzione.
func (m *Machine) Step() error { return m.CPU.Step() }

// Run esegue fino a maxSteps istruzioni o fino a HALT.
func (m *Machine) Run(maxSteps int) (int, error) { return m.CPU.Run(maxSteps) }
