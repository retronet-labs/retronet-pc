package device

import "github.com/retronet-labs/retronet-pc/disk"

// FDC emula il controllore floppy NEC 765 (i8272A) dell'IBM PC/XT, porte
// 0x3F0-0x3F7. E' guidato a comandi: la CPU scrive sul registro dati (0x3F5) il
// byte di comando e i parametri, il controllore esegue (trasferendo i dati del
// settore via DMA sul canale 2) e poi presenta i byte di risultato, alzando IRQ6.
//
// Registri principali:
//   - 0x3F2 DOR (Digital Output Register): reset, abilitazione DMA/IRQ, motori,
//     selezione drive.
//   - 0x3F4 MSR (Main Status Register): RQM (pronto), DIO (direzione), BUSY.
//   - 0x3F5 dati: comando, parametri e risultati.
//
// Modello funzionale: l'esecuzione e il trasferimento DMA avvengono in blocco, non
// ciclo per ciclo. Sono implementati i comandi necessari all'avvio: Specify,
// Recalibrate, Seek, Sense Interrupt Status, Sense Drive Status, Read ID, Read Data
// e Write Data.
type FDC struct {
	dor    byte
	phase  int // 0 = idle/attesa comando, 1 = raccolta comando, 2 = risultati
	cmd    []byte
	needed int
	result []byte
	resIdx int

	pcn        [4]int // cilindro presente per drive
	st0        byte   // ultimo Status Register 0 (per Sense Interrupt)
	intPending bool

	// Collegamenti forniti dalla macchina.
	Disk *disk.Floppy // drive 0
	DMA  *DMA
	Mem  memWriter
	IRQ6 func()
}

// NewFDC crea un controllore floppy non collegato (Disk/DMA/Mem da impostare).
func NewFDC() *FDC { return &FDC{} }

// In8 legge una porta del controllore.
func (f *FDC) In8(port uint16) byte {
	switch port & 0x07 {
	case 4: // MSR
		return f.msr()
	case 5: // dati / risultati
		return f.readData()
	}
	return 0xFF
}

// Out8 scrive una porta del controllore.
func (f *FDC) Out8(port uint16, value byte) {
	switch port & 0x07 {
	case 2: // DOR
		f.writeDOR(value)
	case 5: // comando / parametri
		f.writeData(value)
	}
}

// msr compone il Main Status Register. Essendo il modello istantaneo, RQM e'
// sempre alto; DIO indica la direzione (alto nei risultati) e BUSY l'attivita'.
func (f *FDC) msr() byte {
	var s byte = 0x80 // RQM
	if f.phase == 2 {
		s |= 0x40 // DIO: dati dal controllore alla CPU
	}
	if f.phase != 0 {
		s |= 0x10 // BUSY
	}
	return s
}

func (f *FDC) writeDOR(value byte) {
	wasReset := f.dor&0x04 == 0
	f.dor = value
	if wasReset && value&0x04 != 0 {
		// Uscita dal reset: il controllore genera un interrupt; il BIOS lo rileva
		// con Sense Interrupt Status.
		f.phase, f.resIdx = 0, 0
		f.st0 = 0xC0 // ready line changed
		f.intPending = true
		f.fireIRQ()
	}
}

func (f *FDC) readData() byte {
	if f.phase != 2 || f.resIdx >= len(f.result) {
		return 0x00
	}
	b := f.result[f.resIdx]
	f.resIdx++
	if f.resIdx >= len(f.result) {
		f.phase = 0 // risultati esauriti: torna in attesa di comando
	}
	return b
}

func (f *FDC) writeData(value byte) {
	switch f.phase {
	case 0:
		f.cmd = []byte{value}
		f.needed = 1 + paramCount(value)
		f.phase = 1
	case 1:
		f.cmd = append(f.cmd, value)
	default:
		return // durante i risultati le scritture si ignorano
	}
	if f.phase == 1 && len(f.cmd) >= f.needed {
		f.execute()
	}
}

// paramCount restituisce il numero di byte di parametro che seguono il comando.
func paramCount(cmd byte) int {
	switch cmd & 0x1F {
	case 0x05, 0x06, 0x09, 0x0C: // Write/Read Data e varianti
		return 8
	case 0x0D: // Format Track
		return 5
	case 0x0F: // Seek
		return 2
	case 0x03: // Specify
		return 2
	case 0x07: // Recalibrate
		return 1
	case 0x04: // Sense Drive Status
		return 1
	case 0x0A: // Read ID
		return 1
	default: // Sense Interrupt Status (0x08) e sconosciuti
		return 0
	}
}

func (f *FDC) execute() {
	switch f.cmd[0] & 0x1F {
	case 0x03: // Specify: nessun risultato, nessun interrupt
		f.phase = 0
	case 0x07: // Recalibrate: porta a cilindro 0
		drive := f.cmd[1] & 0x03
		f.pcn[drive] = 0
		f.seekDone(drive)
	case 0x0F: // Seek
		drive := f.cmd[1] & 0x03
		f.pcn[drive] = int(f.cmd[2])
		f.seekDone(drive)
	case 0x08: // Sense Interrupt Status
		f.senseInterrupt()
	case 0x04: // Sense Drive Status -> ST3
		drive := f.cmd[1] & 0x03
		st3 := drive | 0x20 // pronto
		if f.pcn[drive] == 0 {
			st3 |= 0x10 // track 0
		}
		f.giveResult([]byte{st3}, false)
	case 0x0A: // Read ID
		f.readID()
	case 0x06: // Read Data
		f.readWrite(false)
	case 0x05: // Write Data
		f.readWrite(true)
	default: // comando non gestito
		f.giveResult([]byte{0x80}, false) // ST0 = invalid command
	}
}

func (f *FDC) seekDone(drive byte) {
	f.st0 = 0x20 | drive // seek end
	f.intPending = true
	f.phase = 0 // i risultati si ottengono con Sense Interrupt
	f.fireIRQ()
}

func (f *FDC) senseInterrupt() {
	if f.intPending {
		f.intPending = false
		f.giveResult([]byte{f.st0, byte(f.pcn[f.st0&0x03])}, false)
		return
	}
	f.giveResult([]byte{0x80}, false) // nessun interrupt in sospeso
}

func (f *FDC) readID() {
	drive := f.cmd[1] & 0x03
	head := (f.cmd[1] >> 2) & 0x01
	f.st0 = drive | head<<2
	f.intPending = true
	c := byte(f.pcn[drive])
	f.giveResult([]byte{f.st0, 0, 0, c, head, 1, 2}, true)
}

// readWrite esegue Read Data / Write Data trasferendo i settori da R a EOT via DMA
// sul canale 2. I parametri seguono lo schema del 765: cmd[1]=drive/head,
// cmd[2]=C, cmd[3]=H, cmd[4]=R, cmd[5]=N, cmd[6]=EOT.
func (f *FDC) readWrite(write bool) {
	drive := f.cmd[1] & 0x03
	head := int(f.cmd[1]>>2) & 0x01
	c := int(f.cmd[2])
	h := int(f.cmd[3])
	r := int(f.cmd[4])
	n := f.cmd[5]
	eot := int(f.cmd[6])

	st0 := drive | byte(head)<<2
	var st1 byte

	if f.Disk == nil {
		st0 |= 0x40 // abnormal termination
		st1 |= 0x04 // no data
	} else {
		for s := r; s <= eot; s++ {
			var err error
			if write {
				if f.DMA != nil && f.Mem != nil {
					data := f.DMA.TransferFromMemory(2, f.Mem, f.Disk.Geo.SectorSize)
					err = f.Disk.WriteSector(c, h, s, data)
				}
			} else {
				var data []byte
				data, err = f.Disk.ReadSector(c, h, s)
				if err == nil && f.DMA != nil && f.Mem != nil {
					f.DMA.TransferToMemory(2, f.Mem, data)
				}
			}
			if err != nil {
				st0 |= 0x40
				st1 |= 0x04
				break
			}
			r = s
		}
		r++ // il 765 restituisce la posizione del settore successivo
	}

	f.st0 = st0
	f.pcn[drive] = c
	f.intPending = true
	f.giveResult([]byte{st0, st1, 0, byte(c), byte(h), byte(r), n}, true)
}

// giveResult passa in fase di risultato con i byte dati; raiseIRQ indica se alzare
// IRQ6 (i comandi senza interrupt, come Sense Drive Status, non lo fanno).
func (f *FDC) giveResult(res []byte, raiseIRQ bool) {
	f.result = res
	f.resIdx = 0
	f.phase = 2
	if raiseIRQ {
		f.fireIRQ()
	}
}

func (f *FDC) fireIRQ() {
	if f.dor&0x08 != 0 && f.IRQ6 != nil { // IRQ/DMA abilitati nel DOR
		f.IRQ6()
	}
}
