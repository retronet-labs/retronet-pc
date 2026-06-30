package device

import "github.com/retronet-labs/retronet-pc/disk"

// Bit del registro di stato ATA.
const (
	ataBSY  = 0x80 // occupato
	ataDRDY = 0x40 // drive pronto
	ataDF   = 0x20 // device fault
	ataDSC  = 0x10 // seek complete
	ataDRQ  = 0x08 // richiesta dati (buffer pronto al trasferimento PIO)
	ataERR  = 0x01 // errore (dettagli nel registro Error)
)

// ATA emula un canale IDE/ATA con un solo disco (master) in modello funzionale: i
// comandi si eseguono in blocco e i dati passano via PIO a parole di 16 bit dal/al
// buffer interno. E' pilotato dalla scheda XT-IDE (xtide.go), che adatta il bus a
// 8 bit. Lo slave e' assente: selezionandolo lo stato si legge 0 (nessun drive).
type ATA struct {
	Disk *disk.HardDisk

	features byte
	secCount byte
	lba0     byte // sector number / LBA 7:0
	lba1     byte // cyl low / LBA 15:8
	lba2     byte // cyl high / LBA 23:16
	drvHead  byte // bit6=LBA, bit4=drive, bit3:0=head / LBA 27:24
	status   byte
	errReg   byte

	buf      []byte
	bufPos   int
	writing  bool // buffer in attesa di dati dall'host (WRITE SECTORS)
	writeLBA uint32
}

// NewATA crea un canale ATA pronto (nessun disco finche' non si assegna Disk).
func NewATA() *ATA { return &ATA{status: ataDRDY | ataDSC} }

// present indica se il drive selezionato c'e': solo il master (bit4=0) con un disco.
func (a *ATA) present() bool { return a.Disk != nil && a.drvHead&0x10 == 0 }

// WriteReg scrive un registro del command block (1..7); 7 = registro comando.
func (a *ATA) WriteReg(reg int, val byte) {
	switch reg {
	case 1:
		a.features = val
	case 2:
		a.secCount = val
	case 3:
		a.lba0 = val
	case 4:
		a.lba1 = val
	case 5:
		a.lba2 = val
	case 6:
		a.drvHead = val
	case 7:
		a.command(val)
	}
}

// ReadReg legge un registro del command block (1..7); 7 = registro di stato.
func (a *ATA) ReadReg(reg int) byte {
	switch reg {
	case 1:
		return a.errReg
	case 2:
		return a.secCount
	case 3:
		return a.lba0
	case 4:
		return a.lba1
	case 5:
		return a.lba2
	case 6:
		return a.drvHead
	case 7:
		return a.statusValue()
	}
	return 0
}

// AltStatus legge lo stato senza effetti collaterali (registro del control block).
func (a *ATA) AltStatus() byte { return a.statusValue() }

func (a *ATA) statusValue() byte {
	if !a.present() {
		return 0 // nessun drive: bus flottante letto come 0
	}
	return a.status
}

// WriteDevCtl gestisce il Device Control: il bit SRST (0x04) resetta il drive.
func (a *ATA) WriteDevCtl(val byte) {
	if val&0x04 != 0 {
		a.status = ataDRDY | ataDSC
		a.errReg = 0x01
		a.buf, a.bufPos, a.writing = nil, 0, false
	}
}

// ReadData16 restituisce la prossima parola dal buffer (READ SECTORS / IDENTIFY).
func (a *ATA) ReadData16() uint16 {
	if a.bufPos+1 >= len(a.buf) {
		if a.bufPos+1 == len(a.buf) {
			w := uint16(a.buf[a.bufPos]) | uint16(a.buf[a.bufPos+1])<<8
			a.bufPos += 2
			a.status &^= ataDRQ
			return w
		}
		return 0xFFFF
	}
	w := uint16(a.buf[a.bufPos]) | uint16(a.buf[a.bufPos+1])<<8
	a.bufPos += 2
	if a.bufPos >= len(a.buf) {
		a.status &^= ataDRQ
	}
	return w
}

// WriteData16 accetta la prossima parola nel buffer (WRITE SECTORS); a buffer pieno
// scrive i settori sul disco.
func (a *ATA) WriteData16(w uint16) {
	if !a.writing || a.bufPos+1 >= len(a.buf) {
		return
	}
	a.buf[a.bufPos] = byte(w)
	a.buf[a.bufPos+1] = byte(w >> 8)
	a.bufPos += 2
	if a.bufPos >= len(a.buf) {
		if err := a.Disk.WriteLBA(a.writeLBA, a.buf); err != nil {
			a.errReg, a.status = 0x10, ataDRDY|ataDSC|ataERR // IDNF
		} else {
			a.status = ataDRDY | ataDSC
		}
		a.writing = false
		a.status &^= ataDRQ
	}
}

func (a *ATA) sectorCount() int {
	if a.secCount == 0 {
		return 256
	}
	return int(a.secCount)
}

// currentLBA traduce i registri indirizzo nel blocco logico, in modo LBA o CHS.
func (a *ATA) currentLBA() uint32 {
	if a.drvHead&0x40 != 0 { // modo LBA
		return uint32(a.drvHead&0x0F)<<24 | uint32(a.lba2)<<16 | uint32(a.lba1)<<8 | uint32(a.lba0)
	}
	cyl := int(a.lba2)<<8 | int(a.lba1)
	head := int(a.drvHead & 0x0F)
	sector := int(a.lba0)
	geo := a.Disk.Geometry()
	return uint32((cyl*geo.Heads+head)*geo.Sectors + (sector - 1))
}

func (a *ATA) command(cmd byte) {
	a.errReg = 0
	if !a.present() {
		a.status = 0
		return
	}
	switch {
	case cmd == 0xEC: // IDENTIFY DEVICE
		a.buf, a.bufPos, a.writing = a.identify(), 0, false
		a.status = ataDRDY | ataDSC | ataDRQ
	case cmd == 0x20 || cmd == 0x21 || cmd == 0xC4: // READ SECTORS / READ MULTIPLE
		a.readSectors()
	case cmd == 0x30 || cmd == 0x31 || cmd == 0xC5: // WRITE SECTORS / WRITE MULTIPLE
		a.beginWrite()
	case cmd >= 0x10 && cmd <= 0x1F: // RECALIBRATE
		a.status = ataDRDY | ataDSC
	case cmd == 0x91 || cmd == 0x90 || cmd == 0xEF || cmd == 0xC6 ||
		cmd == 0x40 || cmd == 0x41 || // INIT PARAMS, DIAGNOSTIC, SET FEATURES, SET MULTIPLE, VERIFY
		cmd == 0xE0 || cmd == 0xE1 || cmd == 0xE7 || cmd == 0xEA: // standby/idle/flush
		if cmd == 0x90 {
			a.errReg = 0x01 // diagnostica: drive 0 ok
		}
		a.status = ataDRDY | ataDSC
	default:
		a.errReg, a.status = 0x04, ataDRDY|ataDSC|ataERR // ABRT: comando non gestito
	}
}

func (a *ATA) readSectors() {
	data, err := a.Disk.ReadLBA(a.currentLBA(), a.sectorCount())
	if err != nil {
		a.errReg, a.status = 0x10, ataDRDY|ataDSC|ataERR // IDNF
		return
	}
	a.buf, a.bufPos, a.writing = data, 0, false
	a.status = ataDRDY | ataDSC | ataDRQ
}

func (a *ATA) beginWrite() {
	a.writeLBA = a.currentLBA()
	a.buf = make([]byte, a.sectorCount()*disk.SectorSize)
	a.bufPos, a.writing = 0, true
	a.status = ataDRDY | ataDSC | ataDRQ
}

// identify costruisce il blocco IDENTIFY DEVICE (256 parole). Le stringhe ATA sono
// memorizzate con i byte scambiati all'interno di ogni parola.
func (a *ATA) identify() []byte {
	geo := a.Disk.Geometry()
	total := a.Disk.Sectors()
	b := make([]byte, 512)
	setW := func(i int, v uint16) { b[i*2] = byte(v); b[i*2+1] = byte(v >> 8) }
	setStr := func(start, words int, s string) {
		raw := make([]byte, words*2)
		for i := range raw {
			raw[i] = ' '
		}
		copy(raw, s)
		for i := 0; i < words; i++ {
			b[(start+i)*2] = raw[i*2+1]
			b[(start+i)*2+1] = raw[i*2]
		}
	}
	setW(0, 0x0040) // disco fisso, non rimovibile
	setW(1, uint16(geo.Cylinders))
	setW(3, uint16(geo.Heads))
	setW(6, uint16(geo.Sectors))
	setStr(10, 10, "RNHD00000001")              // numero di serie
	setStr(23, 4, "1.0")                        // revisione firmware
	setStr(27, 20, "RetroNet XT-IDE Hard Disk") // modello
	setW(47, 0x8000)                            // READ/WRITE MULTIPLE non supportati
	setW(49, 0x0200)                            // capacita': LBA supportato
	setW(53, 0x0001)                            // parole 54-58 valide
	setW(54, uint16(geo.Cylinders))
	setW(55, uint16(geo.Heads))
	setW(56, uint16(geo.Sectors))
	chsCap := uint32(geo.Cylinders * geo.Heads * geo.Sectors)
	setW(57, uint16(chsCap))
	setW(58, uint16(chsCap>>16))
	setW(60, uint16(total)) // capacita' totale LBA28
	setW(61, uint16(total>>16))
	return b
}
