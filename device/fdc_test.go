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
