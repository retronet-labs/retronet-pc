package device

// PPI emula l'interfaccia parallela programmabile Intel 8255 dell'IBM PC/XT,
// porte 0x60-0x63. Sull'XT collega tastiera, speaker e DIP switch di
// configurazione, con direzione delle porte fissata dal BIOS (A e C in ingresso,
// B in uscita):
//
//   - Port A (0x60): in ingresso. Con PB7=1 mostra i DIP switch SW1, con PB7=0 il
//     codice di scansione della tastiera.
//   - Port B (0x61): in uscita. Bit di controllo: PB0 gate del timer 2 (speaker),
//     PB1 dato speaker, PB3 seleziona quale meta' dei DIP switch appare su Port C,
//     PB6 clock tastiera (basso = reset), PB7 abilita/azzera la tastiera.
//   - Port C (0x62): in ingresso. Nibble basso = meta' dei DIP switch (scelta da
//     PB3), nibble alto = bit di stato (parita', ecc.).
//   - 0x63: parola di controllo (configurazione delle direzioni).
//
// Questa e' una versione funzionale: memorizza Port B e la parola di controllo e
// restituisce DIP switch e tastiera secondo la selezione di Port B. Basta al POST
// del BIOS per leggere la configurazione della macchina.
type PPI struct {
	portB   byte
	control byte

	// DIPSwitches sono gli interruttori SW1 di configurazione (memoria, video,
	// numero di floppy, presenza 8087). Impostabili dall'utente della macchina.
	DIPSwitches byte
	// KeyboardData e' l'ultimo codice di scansione presentato su Port A.
	KeyboardData byte

	// IRQ1 viene chiamato quando la tastiera ha un byte pronto (collegato al PIC
	// dalla macchina).
	IRQ1 func()

	// Coda dei codici di scansione in attesa di essere consegnati alla CPU.
	kbQueue   []byte
	kbCurrent byte // codice attualmente presentato su Port A (0 = nessuno)
	kbDelay   int  // colpi di clock prima di presentare il prossimo codice
}

// kbReloadDelay modella il ritardo di trasmissione seriale della tastiera tra un
// codice e il successivo. Senza, presentare subito il codice seguente farebbe
// rientrare il gestore INT 9 del BIOS prima che abbia finito (i tasti
// arriverebbero in ordine invertito).
const kbReloadDelay = 4000

// NewPPI crea una PPI con i DIP switch a zero (configurazione da impostare).
func NewPPI() *PPI { return &PPI{} }

// In8 legge una porta della PPI.
func (p *PPI) In8(port uint16) byte {
	switch port & 0x03 {
	case 0: // Port A: tastiera oppure SW1 secondo PB7
		if p.portB&0x80 != 0 {
			return p.DIPSwitches
		}
		return p.KeyboardData
	case 1: // Port B (rilettura del latch d'uscita)
		return p.portB
	case 2: // Port C: meta' dei DIP switch selezionata da PB3, stato nel nibble alto
		if p.portB&0x08 != 0 {
			return p.DIPSwitches >> 4
		}
		return p.DIPSwitches & 0x0F
	default: // 0x63: la parola di controllo non e' leggibile
		return 0xFF
	}
}

// Out8 scrive una porta della PPI.
func (p *PPI) Out8(port uint16, value byte) {
	switch port & 0x03 {
	case 1: // Port B
		old := p.portB
		p.portB = value
		// Clear/ack della tastiera (PB7 0->1): il BIOS ha letto il codice; azzera
		// il latch (cosi' non sembra un tasto bloccato).
		if value&0x80 != 0 && old&0x80 == 0 {
			p.KeyboardData = 0
			p.kbCurrent = 0
		}
		// Riabilitazione (PB7 1->0): la tastiera torna leggibile; il prossimo codice
		// in coda arrivera' dopo il ritardo di trasmissione (gestito da Tick).
		if value&0x80 == 0 && old&0x80 != 0 {
			p.kbDelay = kbReloadDelay
		}
		// Reset della tastiera: il BIOS tiene basso il clock (PB6=0) e poi lo
		// rilascia (PB6 0->1). La tastiera esegue il Basic Assurance Test e invia
		// il codice 0xAA con IRQ1.
		if value&0x40 != 0 && old&0x40 == 0 {
			p.kbQueue = nil
			p.kbCurrent = 0xAA
			p.KeyboardData = 0xAA
			if p.IRQ1 != nil {
				p.IRQ1()
			}
		}
	case 3: // parola di controllo
		p.control = value
	}
	// Port A e Port C sono in ingresso sull'XT: le scritture si ignorano.
}

// SpeakerOn indica se lo speaker e' pilotato (PB0 gate del timer 2 e PB1 dato
// entrambi alti).
func (p *PPI) SpeakerOn() bool { return p.portB&0x03 == 0x03 }

// PressScancode accoda un codice di scansione (make o break) proveniente dalla
// tastiera. Se la tastiera e' libera, abilitata e senza ritardo pendente, lo
// presenta subito con IRQ1; altrimenti lo consegnera' Tick dopo il ritardo.
func (p *PPI) PressScancode(code byte) {
	p.kbQueue = append(p.kbQueue, code)
	if p.kbCurrent == 0 && p.kbDelay == 0 && p.portB&0x80 == 0 {
		p.advanceKey()
	}
}

// Tick fa scorrere il ritardo di trasmissione della tastiera: quando scade e la
// tastiera e' libera, presenta il prossimo codice in coda.
func (p *PPI) Tick(cycles int) {
	if p.kbCurrent != 0 || len(p.kbQueue) == 0 {
		return
	}
	if p.kbDelay > 0 {
		p.kbDelay -= cycles
		if p.kbDelay > 0 {
			return
		}
		p.kbDelay = 0
	}
	p.advanceKey()
}

// advanceKey estrae il prossimo codice dalla coda, lo presenta su Port A e alza
// IRQ1.
func (p *PPI) advanceKey() {
	if len(p.kbQueue) == 0 {
		return
	}
	p.kbCurrent = p.kbQueue[0]
	p.kbQueue = p.kbQueue[1:]
	p.KeyboardData = p.kbCurrent
	if p.IRQ1 != nil {
		p.IRQ1()
	}
}
