package device

// PIC emula il controllore di interrupt programmabile Intel 8259A nella
// configurazione dell'IBM PC/XT: un solo 8259, porte 0x20 (comandi) e 0x21
// (dati/maschera), otto linee IRQ0-7. Le linee hanno priorita' fissa e annidata
// con IRQ0 la piu' alta.
//
// # Come funziona
//
// Tre registri a 8 bit, un bit per IRQ:
//
//   - IRR (Interrupt Request Register): le richieste arrivate e non ancora servite.
//   - IMR (Interrupt Mask Register): le linee mascherate (1 = bloccata).
//   - ISR (In-Service Register): gli interrupt attualmente in servizio.
//
// Quando una periferica alza una linea (RaiseIRQ) si accende il bit in IRR. Il
// PIC propone alla CPU l'IRQ pronto a priorita' piu' alta che non sia mascherato
// ne' bloccato da un interrupt in servizio di pari/maggiore priorita'. Al
// riconoscimento (Acknowledge) il bit passa da IRR a ISR e il PIC restituisce il
// numero di vettore = base + linea. Il gestore, a fine routine, invia un End Of
// Interrupt (EOI) che libera il bit in ISR.
//
// # Programmazione
//
// All'avvio il BIOS inizializza il chip con la sequenza ICW1..ICW4 (la base dei
// vettori arriva da ICW2; sull'XT e' 0x08, quindi IRQ0 -> INT 08h). A regime si
// usano OCW1 (scrittura della maschera su 0x21) e OCW2 (EOI su 0x20).
type PIC struct {
	irr, imr, isr byte
	base          byte // numero di vettore per IRQ0 (ICW2 & 0xF8)
	autoEOI       bool

	// Stato della sequenza di inizializzazione (0 = operativo).
	initStep   int
	icw4Needed bool
}

// NewPIC crea un PIC non inizializzato (tutte le linee mascherate finche' il BIOS
// non lo programma, come a freddo).
func NewPIC() *PIC { return &PIC{imr: 0xFF} }

// RaiseIRQ segnala la richiesta sulla linea (0-7): accende il bit in IRR.
func (p *PIC) RaiseIRQ(line int) {
	if line >= 0 && line < 8 {
		p.irr |= 1 << uint(line)
	}
}

// Pending indica se c'e' un IRQ pronto da consegnare alla CPU.
func (p *PIC) Pending() bool {
	_, ok := p.highestPending()
	return ok
}

// Acknowledge riconosce l'IRQ a priorita' piu' alta (ciclo INTA): sposta il bit
// da IRR a ISR (salvo Auto-EOI) e restituisce il numero di vettore. Va chiamato
// solo se Pending e' true.
func (p *PIC) Acknowledge() byte {
	line, ok := p.highestPending()
	if !ok {
		return p.base // difensivo: non dovrebbe accadere
	}
	bit := byte(1) << uint(line)
	p.irr &^= bit
	if !p.autoEOI {
		p.isr |= bit
	}
	return p.base + byte(line)
}

// highestPending restituisce la linea pronta a priorita' piu' alta (numero piu'
// basso) non mascherata e non bloccata da un servizio di priorita' >=.
func (p *PIC) highestPending() (int, bool) {
	avail := p.irr &^ p.imr
	if avail == 0 {
		return 0, false
	}
	isrLowest := 8
	for i := 0; i < 8; i++ {
		if p.isr&(1<<uint(i)) != 0 {
			isrLowest = i
			break
		}
	}
	for i := 0; i < 8; i++ {
		if avail&(1<<uint(i)) != 0 {
			if i < isrLowest {
				return i, true
			}
			return 0, false // il piu' prioritario in attesa cede a un servizio in corso
		}
	}
	return 0, false
}

// In8 legge una porta del PIC. 0x21 restituisce la maschera (IMR); 0x20
// restituisce l'IRR (lettura di default).
func (p *PIC) In8(port uint16) byte {
	if port&1 == 1 {
		return p.imr
	}
	return p.irr
}

// Out8 scrive una porta del PIC: la sequenza di inizializzazione (ICW) oppure i
// comandi operativi (OCW).
func (p *PIC) Out8(port uint16, value byte) {
	if port&1 == 0 { // porta 0x20: comandi
		switch {
		case value&0x10 != 0: // ICW1: inizia l'inizializzazione
			p.icw4Needed = value&0x01 != 0
			p.imr = 0
			p.isr = 0
			p.irr = 0
			p.initStep = 2 // prossima scrittura su 0x21 = ICW2
		case value&0x18 == 0: // OCW2
			p.handleOCW2(value)
		}
		// OCW3 (value&0x18==0x08) non gestita: lettura poll/registri ignorata.
		return
	}

	// Porta 0x21: dati.
	switch p.initStep {
	case 2: // ICW2: base dei vettori (i 3 bit bassi sono ignorati)
		p.base = value & 0xF8
		// Sull'XT il PIC e' singolo (ICW1 SNGL=1): niente ICW3.
		if p.icw4Needed {
			p.initStep = 4
		} else {
			p.initStep = 0
		}
	case 4: // ICW4: bit1 = Auto-EOI
		p.autoEOI = value&0x02 != 0
		p.initStep = 0
	default: // OCW1: scrittura della maschera
		p.imr = value
	}
}

// handleOCW2 gestisce i comandi di fine interrupt (EOI).
func (p *PIC) handleOCW2(value byte) {
	switch value & 0x60 {
	case 0x20: // EOI non specifico: libera l'ISR a priorita' piu' alta
		for i := 0; i < 8; i++ {
			if p.isr&(1<<uint(i)) != 0 {
				p.isr &^= 1 << uint(i)
				return
			}
		}
	case 0x60: // EOI specifico: libera la linea indicata
		p.isr &^= 1 << uint(value&0x07)
	}
}
