// Package machine assembla un IBM PC/XT: collega la CPU 8086/8088 di
// retronet-8086 al bus di memoria mappato, al dispatcher di I/O e alle
// periferiche (PIC, PIT, PPI), gestendo il percorso degli interrupt hardware.
package machine

import (
	"github.com/retronet-labs/retronet-8086/cpu"
	"github.com/retronet-labs/retronet-pc/device"
	"github.com/retronet-labs/retronet-pc/io"
	"github.com/retronet-labs/retronet-pc/memory"
)

// Machine e' un PC/XT: CPU, memoria, bus I/O e le periferiche di base. I campi
// Pic/Pit/Ppi sono nil in una macchina "nuda" (New) e popolati da NewXT.
type Machine struct {
	CPU *cpu.CPU8086
	Mem *memory.Bus
	IO  *io.Ports

	Pic   *device.PIC
	Pit   *device.PIT
	Ppi   *device.PPI
	Video *device.MDA

	// timerCycles e' il numero di colpi di clock del PIT fatti avanzare a ogni
	// istruzione. Il modello non e' cycle-accurate: questa e' un'approssimazione
	// del rapporto tra clock della CPU e del timer, sufficiente a un tick
	// periodico credibile.
	timerCycles int
}

// New crea una macchina "nuda": CPU al profilo 8088, memoria da 1 MB, bus I/O
// vuoto, nessuna periferica. Utile per i test di basso livello.
func New() *Machine {
	m := &Machine{Mem: memory.New(), IO: io.New()}
	c := cpu.NewCPU8086()
	c.Mem = m.Mem
	c.IO = m.IO
	m.CPU = c
	return m
}

// NewXT crea un PC/XT completo delle periferiche di base, gia' cablate e mappate
// alle porte canoniche:
//
//   - 8259 PIC  -> 0x20-0x21
//   - 8253 PIT  -> 0x40-0x43 (uscita del contatore 0 collegata a IRQ0)
//   - 8255 PPI  -> 0x60-0x63
//   - MDA       -> 0x3B4-0x3BB (testo monocromatico 80x25 a 0xB0000)
//
// Dopo il reset la CPU parte dal vettore 0xFFFF0, dove va caricato il BIOS con
// Mem.LoadROM.
func NewXT() *Machine {
	m := New()
	m.Pic = device.NewPIC()
	m.Pit = device.NewPIT()
	m.Ppi = device.NewPPI()
	m.Video = device.NewMDA()

	// L'uscita del contatore 0 del timer alza IRQ0 sul PIC.
	m.Pit.IRQ0 = func() { m.Pic.RaiseIRQ(0) }

	// DIP switch SW1: tipo video monocromatico (MDA) nei bit 4-5.
	m.Ppi.DIPSwitches = 0x30

	m.IO.Map(0x20, 0x21, m.Pic)
	m.IO.Map(0x40, 0x43, m.Pit)
	m.IO.Map(0x60, 0x63, m.Ppi)
	m.IO.Map(0x3B4, 0x3BB, m.Video)

	m.timerCycles = 1
	return m
}

// Screen restituisce lo schermo testuale corrente (80x25) leggendo la RAM video
// dall'MDA. Vuoto se la macchina non ha video.
func (m *Machine) Screen() string {
	if m.Video == nil {
		return ""
	}
	return m.Video.Render(m.Mem)
}

// Map collega una periferica a un intervallo di porte I/O.
func (m *Machine) Map(lo, hi uint16, dev io.Device) { m.IO.Map(lo, hi, dev) }

// Step esegue un passo della macchina: fa avanzare il timer, serve un eventuale
// interrupt hardware riconosciuto dal PIC (se IF abilitato), altrimenti esegue
// una istruzione. In HALT senza interrupt il tempo avanza ma non si esegue.
func (m *Machine) Step() error {
	if m.Pit != nil {
		m.Pit.Tick(m.timerCycles)
	}
	if m.Pic != nil && m.Pic.Pending() && m.CPU.InterruptsEnabled() {
		m.CPU.Interrupt(m.Pic.Acknowledge())
		return nil
	}
	if m.CPU.Halted {
		return nil // in attesa di un interrupt: nessuna istruzione, ma il timer gira
	}
	return m.CPU.Step()
}

// Run esegue fino a maxSteps passi. A differenza di cpu.Run non si ferma su HALT,
// perche' un interrupt puo' risvegliare la CPU; spetta al chiamante limitare i passi.
func (m *Machine) Run(maxSteps int) (int, error) {
	for i := 0; i < maxSteps; i++ {
		if err := m.Step(); err != nil {
			return i, err
		}
	}
	return maxSteps, nil
}
