package device

// PIT emula il timer programmabile Intel 8253 dell'IBM PC/XT: tre contatori a 16
// bit indipendenti, porte 0x40-0x42 (dati dei contatori) e 0x43 (parola di
// controllo). L'ingresso di clock e' ~1,193182 MHz.
//
// Ruolo dei contatori sull'XT:
//   - contatore 0: tick di sistema, la cui uscita pilota IRQ0 (~18,2 Hz col
//     valore di ricarica 0 = 65536);
//   - contatore 1: refresh della DRAM;
//   - contatore 2: tono dello speaker (abilitato dalla PPI).
//
// Questa implementazione modella i contatori come down-counter con ricarica:
// e' sufficiente a generare il tick periodico IRQ0 e a rispondere a letture e
// scritture dei contatori. Le sottigliezze elettriche delle sei modalita' non
// sono riprodotte: ai fini dell'IRQ contano solo gli azzeramenti (terminal count).
type PIT struct {
	counters [3]counter
	// IRQ0 viene chiamato a ogni terminal count del contatore 0 (collegato al PIC
	// dalla macchina).
	IRQ0 func()
	// Counter1Out viene chiamato a ogni terminal count del contatore 1, che
	// sull'XT pilota il refresh della DRAM tramite il DMA canale 0.
	Counter1Out func()
}

type counter struct {
	reload  uint16 // valore di ricarica (0 = 65536)
	count   uint16 // conteggio corrente
	mode    byte   // modalita' 0-5
	access  byte   // 1=solo LSB, 2=solo MSB, 3=LSB poi MSB
	writeHi bool   // access 3: la prossima scrittura e' il byte alto
	readHi  bool   // access 3: la prossima lettura e' il byte alto
	latched bool
	latch   uint16
	running bool
}

// NewPIT crea un timer con i contatori a riposo (da programmare).
func NewPIT() *PIT { return &PIT{} }

// Tick avanza tutti i contatori di cycles colpi di clock. Ogni azzeramento del
// contatore 0 alza IRQ0.
func (t *PIT) Tick(cycles int) {
	if t.counters[0].tick(cycles) > 0 && t.IRQ0 != nil {
		t.IRQ0()
	}
	if p := t.counters[1].tick(cycles); p > 0 && t.Counter1Out != nil {
		for i := 0; i < p; i++ {
			t.Counter1Out()
		}
	}
	t.counters[2].tick(cycles)
}

// tick decrementa il contatore di n colpi e restituisce quanti azzeramenti
// (terminal count) sono avvenuti.
func (c *counter) tick(n int) int {
	if !c.running || n <= 0 {
		return 0
	}
	period := int(c.reload)
	if period == 0 {
		period = 65536
	}
	cur := int(c.count)
	if cur == 0 {
		cur = period
	}
	cur -= n
	pulses := 0
	for cur <= 0 {
		pulses++
		cur += period
	}
	c.count = uint16(cur)
	return pulses
}

// Out8 scrive una porta del PIT.
func (t *PIT) Out8(port uint16, value byte) {
	if port&3 == 3 { // 0x43: parola di controllo
		t.controlWord(value)
		return
	}
	c := &t.counters[port&3]
	switch c.access {
	case 1: // solo LSB: il byte alto e' azzerato
		c.reload = uint16(value)
		c.load()
	case 2: // solo MSB: il byte basso e' azzerato
		c.reload = uint16(value) << 8
		c.load()
	default: // 3: LSB poi MSB
		if !c.writeHi {
			c.reload = c.reload&0xFF00 | uint16(value)
			c.writeHi = true
		} else {
			c.reload = c.reload&0x00FF | uint16(value)<<8
			c.writeHi = false
			c.load()
		}
	}
}

func (c *counter) load() {
	c.count = c.reload
	c.running = true
}

// controlWord configura un contatore o ne latcha il valore per la lettura.
func (t *PIT) controlWord(v byte) {
	sel := v >> 6
	if sel == 3 {
		return // read-back (8254): non gestito sull'8253
	}
	c := &t.counters[sel]
	access := (v >> 4) & 0x03
	if access == 0 { // comando di latch del contatore
		c.latch = c.count
		c.latched = true
		return
	}
	c.access = access
	c.mode = (v >> 1) & 0x07
	c.writeHi = false
	c.readHi = false
	c.running = false
}

// In8 legge una porta del PIT (il conteggio corrente o quello latchato).
func (t *PIT) In8(port uint16) byte {
	if port&3 == 3 {
		return 0xFF // la parola di controllo non e' leggibile
	}
	c := &t.counters[port&3]
	val := c.count
	if c.latched {
		val = c.latch
	}
	switch c.access {
	case 1:
		c.latched = false
		return byte(val)
	case 2:
		c.latched = false
		return byte(val >> 8)
	default: // 3: LSB poi MSB
		if !c.readHi {
			c.readHi = true
			return byte(val)
		}
		c.readHi = false
		c.latched = false
		return byte(val >> 8)
	}
}
