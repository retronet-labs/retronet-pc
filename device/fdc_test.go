package device

import (
	"testing"

	"github.com/retronet-labs/retronet-pc/disk"
)

// TestFDCReadSectorViaDMA verifica il percorso completo del floppy: programmazione
// del DMA canale 2, comando Read Data al controllore, trasferimento del settore in
// memoria e generazione di IRQ6, con i byte di risultato corretti.
func TestFDCReadSectorViaDMA(t *testing.T) {
	// Immagine 360 KB con un settore di boot riconoscibile (C0/H0/S1).
	img := make([]byte, 360*1024)
	for i := 0; i < 512; i++ {
		img[i] = byte(i)
	}
	img[510], img[511] = 0x55, 0xAA // firma di boot
	fl, err := disk.NewFloppy(img)
	if err != nil {
		t.Fatal(err)
	}

	mem := &fakeMem{}
	dma := NewDMA()
	irq := 0
	fdc := NewFDC()
	fdc.DMA, fdc.Mem, fdc.Disk = dma, mem, fl
	fdc.IRQ6 = func() { irq++ }

	// DOR: fuori reset, DMA/IRQ abilitati, motore drive 0.
	fdc.Out8(0x3F2, 0x1C)

	// Programma il DMA canale 2: destinazione fisica 0x00500, 512 byte.
	dma.Out8(0x0A, 0x06) // maschera il canale 2 durante la programmazione
	dma.Out8(0x0C, 0x00) // azzera il flip-flop
	dma.Out8(0x04, 0x00) // addr basso
	dma.Out8(0x04, 0x05) // addr alto -> 0x0500
	dma.Out8(0x81, 0x00) // pagina 0
	dma.Out8(0x05, 0xFF) // count basso
	dma.Out8(0x05, 0x01) // count alto -> 511 (= 512 byte)
	dma.Out8(0x0B, 0x46) // modo: canale 2, single, scrittura in memoria
	dma.Out8(0x0A, 0x02) // smaschera il canale 2

	irq = 0 // azzera (la sequenza DOR ha gia' generato un interrupt di reset)

	// Read Data: comando + 8 parametri (drive/head, C, H, R, N, EOT, GPL, DTL).
	for _, b := range []byte{0x06, 0x00, 0x00, 0x00, 0x01, 0x02, 0x01, 0x2A, 0xFF} {
		fdc.Out8(0x3F5, b)
	}

	// Fase di risultato: 7 byte, il primo e' ST0.
	st0 := fdc.In8(0x3F5)
	for i := 1; i < 7; i++ {
		fdc.In8(0x3F5)
	}
	if st0&0xC0 != 0 {
		t.Fatalf("ST0 segnala terminazione anomala: %#02x", st0)
	}
	if irq == 0 {
		t.Error("la lettura non ha generato IRQ6")
	}
	for i := 0; i < 512; i++ {
		if got := mem.Read8(0x0500 + uint32(i)); got != img[i] {
			t.Fatalf("byte %d trasferito = %#02x, atteso %#02x", i, got, img[i])
		}
	}
}

// newMarkedFloppy crea un'immagine 360 KB in cui ogni settore della traccia 0/testa 0
// e' riempito col proprio numero (1-based), per riconoscere quale settore e' stato
// trasferito.
func newMarkedFloppy(t *testing.T) *disk.Floppy {
	t.Helper()
	img := make([]byte, 360*1024)
	for s := 1; s <= 9; s++ {
		off := (s - 1) * 512
		for i := 0; i < 512; i++ {
			img[off+i] = byte(s)
		}
	}
	fl, err := disk.NewFloppy(img)
	if err != nil {
		t.Fatal(err)
	}
	return fl
}

// programReadDMA programma il canale 2 per ricevere nByteCount+1 byte all'indirizzo
// dst e mette l'FDC fuori reset.
func setupReadFDC(t *testing.T, fl *disk.Floppy, dst uint16, count uint16) (*FDC, *fakeMem) {
	t.Helper()
	mem := &fakeMem{}
	dma := NewDMA()
	fdc := NewFDC()
	fdc.DMA, fdc.Mem, fdc.Disk = dma, mem, fl
	fdc.IRQ6 = func() {}
	fdc.Out8(0x3F2, 0x1C) // DOR fuori reset, DMA/IRQ, motore
	dma.Out8(0x0A, 0x06)
	dma.Out8(0x0C, 0x00)
	dma.Out8(0x04, byte(dst))
	dma.Out8(0x04, byte(dst>>8))
	dma.Out8(0x81, 0x00)
	dma.Out8(0x05, byte(count))
	dma.Out8(0x05, byte(count>>8))
	dma.Out8(0x0B, 0x46)
	dma.Out8(0x0A, 0x02)
	return fdc, mem
}

func readResult(fdc *FDC) (st0, st1, rByte byte) {
	res := make([]byte, 7)
	for i := range res {
		res[i] = fdc.In8(0x3F5)
	}
	return res[0], res[1], res[5]
}

// TestFDCReadStopsAtTerminalCount verifica che una lettura programmata per un solo
// settore (conteggio DMA = 512 byte) ma con EOT alto trasferisca esattamente un
// settore — fermandosi sul Terminal Count del DMA — e non l'intera traccia. Senza
// questo, GLaBIOS riceveva piu' dati del previsto e un numero di settore sballato.
func TestFDCReadStopsAtTerminalCount(t *testing.T) {
	fl := newMarkedFloppy(t)
	fdc, mem := setupReadFDC(t, fl, 0x0500, 511) // 512 byte = 1 settore
	// Read Data: R=1, EOT=9 (intera traccia), N=2.
	for _, b := range []byte{0x06, 0x00, 0x00, 0x00, 0x01, 0x02, 0x09, 0x2A, 0xFF} {
		fdc.Out8(0x3F5, b)
	}
	st0, _, rByte := readResult(fdc)
	if st0&0xC0 != 0 {
		t.Fatalf("ST0 anomalo: %#02x", st0)
	}
	// Solo il settore 1 dev'essere stato trasferito.
	if got := mem.Read8(0x0500); got != 1 {
		t.Fatalf("primo settore = %d, atteso 1", got)
	}
	if got := mem.Read8(0x0500 + 512); got != 0 {
		t.Fatalf("oltre il primo settore la memoria e' stata sovrascritta (= %d): trasferiti troppi settori", got)
	}
	if rByte != 2 {
		t.Fatalf("settore successivo riportato = %d, atteso 2", rByte)
	}
}

// TestFDCReadSectorAboveEOT verifica che il settore richiesto in R venga letto anche
// quando R supera EOT (caso reale: GLaBIOS legge un settore alla volta con EOT fisso
// a 8 e richiede R=9,10,...). Il vecchio ciclo "for s:=r; s<=eot" non leggeva nulla,
// lasciando nel buffer dati stantii e corrompendo il caricamento dal nono settore.
func TestFDCReadSectorAboveEOT(t *testing.T) {
	fl := newMarkedFloppy(t)
	fdc, mem := setupReadFDC(t, fl, 0x0500, 511)
	// Read Data: R=9, EOT=8 (R > EOT), N=2.
	for _, b := range []byte{0x06, 0x00, 0x00, 0x00, 0x09, 0x02, 0x08, 0x2A, 0xFF} {
		fdc.Out8(0x3F5, b)
	}
	st0, _, rByte := readResult(fdc)
	if st0&0xC0 != 0 {
		t.Fatalf("ST0 anomalo: %#02x", st0)
	}
	if got := mem.Read8(0x0500); got != 9 {
		t.Fatalf("settore trasferito = %d, atteso 9 (R letto anche se > EOT)", got)
	}
	if rByte != 10 {
		t.Fatalf("settore successivo riportato = %d, atteso 10", rByte)
	}
}
