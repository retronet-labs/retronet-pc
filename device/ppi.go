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
}

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
		p.portB = value
	case 3: // parola di controllo
		p.control = value
	}
	// Port A e Port C sono in ingresso sull'XT: le scritture si ignorano.
}

// SpeakerOn indica se lo speaker e' pilotato (PB0 gate del timer 2 e PB1 dato
// entrambi alti).
func (p *PPI) SpeakerOn() bool { return p.portB&0x03 == 0x03 }
