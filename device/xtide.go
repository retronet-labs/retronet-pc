package device

// XTIDE emula la scheda XT-IDE rev 1: adatta un disco IDE a 16 bit al bus a 8 bit
// dell'IBM PC/XT tramite un latch per il byte alto del dato. Occupa 16 porte a
// partire da una base (canonica 0x300). L'option ROM (XTIDE Universal BIOS) va
// caricata a 0xC8000: al POST aggancia INT 13h e rileva il disco parlando con
// questi registri. Mappa delle porte (porta & 0x0F), come la rev 1 reale:
//
//	0x0      dato: in lettura legge 16 bit dal disco, ritorna il byte basso e
//	         cattura quello alto nel latch; in scrittura combina il latch col
//	         byte basso e scrive i 16 bit
//	0x1-0x7  registri ATA (error/feature, sector count, LBA/CHS, drive, status/cmd)
//	0x8      byte alto del dato (latch)
//	0xE      Device Control (scrittura) / Alternate Status (lettura)
type XTIDE struct {
	ATA      *ATA
	dataHigh byte // latch del byte alto del dato a 16 bit
}

// NewXTIDE crea una scheda XT-IDE con un canale ATA vuoto (assegnare ATA.Disk).
func NewXTIDE() *XTIDE { return &XTIDE{ATA: NewATA()} }

// In8 legge una porta della scheda.
func (x *XTIDE) In8(port uint16) byte {
	switch port & 0x0F {
	case 0x0:
		w := x.ATA.ReadData16()
		x.dataHigh = byte(w >> 8)
		return byte(w)
	case 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7:
		return x.ATA.ReadReg(int(port & 0x07))
	case 0x8:
		return x.dataHigh
	case 0xE:
		return x.ATA.AltStatus()
	}
	return 0xFF
}

// Out8 scrive una porta della scheda.
func (x *XTIDE) Out8(port uint16, value byte) {
	switch port & 0x0F {
	case 0x0:
		x.ATA.WriteData16(uint16(value) | uint16(x.dataHigh)<<8)
	case 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7:
		x.ATA.WriteReg(int(port&0x07), value)
	case 0x8:
		x.dataHigh = value
	case 0xE:
		x.ATA.WriteDevCtl(value)
	}
}
