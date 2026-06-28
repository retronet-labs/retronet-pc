// Comando retronet-pc: avvia un IBM PC/XT con un BIOS in ROM (e un floppy
// opzionale) e ne mostra lo schermo testuale e l'ultimo codice POST.
//
// Le ROM del BIOS e le immagini floppy NON sono incluse: vanno fornite
// dall'utente (per il BIOS si veda il README, es. GLaBIOS open source).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/retronet-labs/retronet-pc/machine"
)

func main() {
	bios := flag.String("bios", "", "file ROM del BIOS (caricato in cima al 1 MB)")
	floppy := flag.String("floppy", "", "immagine floppy raw per il drive A:")
	steps := flag.Int("steps", 20_000_000, "numero massimo di passi da eseguire")
	keys := flag.String("keys", "", "testo da digitare sulla tastiera dopo l'avvio")
	flag.Parse()

	if *bios == "" {
		fmt.Fprintln(os.Stderr, "uso: retronet-pc -bios <rom> [-floppy <img>] [-steps N]")
		os.Exit(2)
	}

	m := machine.NewXT()

	rom, err := os.ReadFile(*bios)
	if err != nil {
		fmt.Fprintln(os.Stderr, "BIOS:", err)
		os.Exit(1)
	}
	m.LoadBIOS(rom)
	fmt.Printf("BIOS caricato: %d byte a %05X-FFFFF\n", len(rom), 0x100000-len(rom))

	if *floppy != "" {
		img, err := os.ReadFile(*floppy)
		if err != nil {
			fmt.Fprintln(os.Stderr, "floppy:", err)
			os.Exit(1)
		}
		if err := m.LoadFloppy(img); err != nil {
			fmt.Fprintln(os.Stderr, "floppy:", err)
			os.Exit(1)
		}
		fmt.Printf("floppy A: %d byte (%dx%dx%d)\n", len(img),
			m.Fdc.Disk.Geo.Cylinders, m.Fdc.Disk.Geo.Heads, m.Fdc.Disk.Geo.Sectors)
	}

	if *keys != "" {
		// Lascia completare POST e avvio, poi "digita" il testo: il ritardo di
		// trasmissione della tastiera scandisce la consegna dei codici.
		m.Run(8_000_000)
		m.Ppi.Type(*keys)
	}

	executed, err := m.Run(*steps)
	fmt.Printf("eseguiti %d passi", executed)
	if err != nil {
		fmt.Printf(" (stop: %v)", err)
	}
	fmt.Printf("; ultimo codice POST: %02X\n", m.Post.Last)
	fmt.Println("---- schermo ----")
	fmt.Print(m.Screen())
	fmt.Println("-----------------")
}
