package device

// DMA emula il controllore di accesso diretto alla memoria Intel 8237A dell'IBM
// PC/XT (porte 0x00-0x0F piu' i registri di pagina 0x80-0x8F). Sull'XT serve al
// refresh della DRAM (canale 0) e ai trasferimenti del floppy (canale 2).
//
// Il modello non riproduce i cicli di bus uno per uno: i registri (indirizzo,
// conteggio, pagina, modo, maschera) sono fedeli, ma il trasferimento vero e
// proprio lo esegue in blocco la periferica che lo richiede (il controllore
// floppy) chiamando TransferToMemory/TransferFromMemory sul canale.
//
// Indirizzo e conteggio sono a 16 bit e si scrivono in due accessi (byte basso
// poi alto), scanditi da un flip-flop azzerabile dalla porta 0x0C.
type DMA struct {
	channels [4]dmaChannel
	flipHi   bool // false = prossimo accesso byte basso, true = byte alto
	tcStatus byte // bit Terminal Count per canale, azzerati alla lettura dello status
}

type dmaChannel struct {
	addr   uint16
	count  uint16
	page   byte
	mode   byte
	masked bool
}

// NewDMA crea un controllore con tutti i canali mascherati (stato di reset).
func NewDMA() *DMA {
	d := &DMA{}
	for i := range d.channels {
		d.channels[i].masked = true
	}
	return d
}

// Out8 scrive un registro del controllore (porte 0x00-0x0F) o un registro di
// pagina (0x80-0x8F).
func (d *DMA) Out8(port uint16, value byte) {
	if port >= 0x80 {
		d.PageOut(port, value)
		return
	}
	switch port & 0x0F {
	case 0x00, 0x02, 0x04, 0x06: // indirizzo dei canali 0-3
		d.writeWord(&d.channels[(port>>1)&3].addr, value)
	case 0x01, 0x03, 0x05, 0x07: // conteggio dei canali 0-3
		d.writeWord(&d.channels[(port>>1)&3].count, value)
	case 0x0A: // maschera singola
		d.channels[value&3].masked = value&0x04 != 0
	case 0x0B: // modo
		d.channels[value&3].mode = value
	case 0x0C: // azzera il flip-flop byte basso/alto
		d.flipHi = false
	case 0x0D: // master clear
		*d = *NewDMA()
	case 0x0F: // maschera multipla
		for i := 0; i < 4; i++ {
			d.channels[i].masked = value&(1<<uint(i)) != 0
		}
	}
}

// In8 legge un registro del controllore o di pagina.
func (d *DMA) In8(port uint16) byte {
	if port >= 0x80 {
		return d.PageIn(port)
	}
	switch port & 0x0F {
	case 0x00, 0x02, 0x04, 0x06:
		return d.readWord(d.channels[(port>>1)&3].addr)
	case 0x01, 0x03, 0x05, 0x07:
		return d.readWord(d.channels[(port>>1)&3].count)
	case 0x08: // registro di stato: bit Terminal Count, azzerati dopo la lettura
		s := d.tcStatus
		d.tcStatus = 0
		return s
	}
	return 0xFF
}

// RefreshCycle esegue un ciclo di refresh DRAM sul canale 0 (pilotato dall'uscita
// del contatore 1 del PIT): decrementa il conteggio e, al Terminal Count, accende
// il bit TC0 nello stato e ricarica il conteggio (auto-init).
func (d *DMA) RefreshCycle() {
	c := &d.channels[0]
	if c.count == 0 {
		d.tcStatus |= 0x01
		c.count = 0xFFFF
	} else {
		c.count--
	}
}

// Page legge/scrive i registri di pagina (porte 0x80-0x8F). La mappatura XT delle
// porte ai canali e': ch0=0x87, ch1=0x83, ch2=0x81, ch3=0x82.
func (d *DMA) PageOut(port uint16, value byte) {
	if ch, ok := pageChannel(port); ok {
		d.channels[ch].page = value
	}
}

func (d *DMA) PageIn(port uint16) byte {
	if ch, ok := pageChannel(port); ok {
		return d.channels[ch].page
	}
	return 0xFF
}

func pageChannel(port uint16) (int, bool) {
	switch port & 0x0F {
	case 0x07:
		return 0, true
	case 0x03:
		return 1, true
	case 0x01:
		return 2, true
	case 0x02:
		return 3, true
	}
	return 0, false
}

func (d *DMA) writeWord(reg *uint16, value byte) {
	if !d.flipHi {
		*reg = *reg&0xFF00 | uint16(value)
	} else {
		*reg = *reg&0x00FF | uint16(value)<<8
	}
	d.flipHi = !d.flipHi
}

func (d *DMA) readWord(reg uint16) byte {
	var b byte
	if !d.flipHi {
		b = byte(reg)
	} else {
		b = byte(reg >> 8)
	}
	d.flipHi = !d.flipHi
	return b
}

// memWriter/memReaderByte sono le capacita' minime sulla memoria usate dai
// trasferimenti DMA (le soddisfa memory.Bus).
type memWriter interface {
	Read8(addr uint32) byte
	Write8(addr uint32, value byte)
}

// physAddr compone l'indirizzo fisico a 20 bit del canale (pagina << 16 | addr).
func (c *dmaChannel) physAddr() uint32 {
	return uint32(c.page)<<16 | uint32(c.addr)
}

// length restituisce il numero di byte ancora da trasferire (conteggio + 1).
func (c *dmaChannel) length() int { return int(c.count) + 1 }

// TransferToMemory copia data in memoria all'indirizzo del canale, poi avanza
// indirizzo e conteggio. Usato dal floppy per una lettura (disco -> RAM). Copia al
// massimo length() byte.
func (d *DMA) TransferToMemory(channel int, mem memWriter, data []byte) {
	c := &d.channels[channel]
	n := c.length()
	if n > len(data) {
		n = len(data)
	}
	base := c.physAddr()
	for i := 0; i < n; i++ {
		mem.Write8((base+uint32(i))&0xFFFFF, data[i])
	}
	if d.advance(c, n) {
		d.tcStatus |= 1 << uint(channel)
	}
}

// TransferFromMemory legge n byte dalla memoria all'indirizzo del canale e li
// restituisce, poi avanza indirizzo e conteggio. Usato dal floppy per una
// scrittura (RAM -> disco).
func (d *DMA) TransferFromMemory(channel int, mem memWriter, n int) []byte {
	c := &d.channels[channel]
	if max := c.length(); n > max {
		n = max
	}
	base := c.physAddr()
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = mem.Read8((base + uint32(i)) & 0xFFFFF)
	}
	if d.advance(c, n) {
		d.tcStatus |= 1 << uint(channel)
	}
	return out
}

// advance fa avanzare indirizzo e conteggio del canale e restituisce true se il
// conteggio e' andato in underflow (Terminal Count).
func (d *DMA) advance(c *dmaChannel, n int) bool {
	underflow := uint16(n) > c.count
	c.addr += uint16(n)
	c.count -= uint16(n)
	return underflow
}
