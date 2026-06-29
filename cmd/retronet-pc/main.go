// Comando retronet-pc: avvia un IBM PC/XT con un BIOS in ROM (e un floppy
// opzionale) e ne mostra lo schermo testuale e l'ultimo codice POST.
//
// Le ROM del BIOS e le immagini floppy NON sono incluse: vanno fornite
// dall'utente (per il BIOS si veda il README, es. GLaBIOS open source).
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"

	"github.com/retronet-labs/retronet-8086/cpu"
	"github.com/retronet-labs/retronet-pc/machine"
)

func main() {
	bios := flag.String("bios", "", "file ROM del BIOS (caricato in cima al 1 MB)")
	floppy := flag.String("floppy", "", "immagine floppy raw per il drive A:")
	steps := flag.Int("steps", 20_000_000, "numero massimo di passi da eseguire (0 = illimitati)")
	keys := flag.String("keys", "", "testo da digitare sulla tastiera dopo l'avvio")
	alu := flag.String("alu", "native", "backend ALU della CPU: native (default) oppure gate (porte logiche)")
	video := flag.String("video", "mda", "adattatore video: mda (default) oppure cga")
	live := flag.Bool("live", false, "aggiorna lo schermo ogni 10M passi (Ctrl+C per fermare)")
	interactive := flag.Bool("interactive", false, "modalita' interattiva: schermo a 60 Hz e tastiera reale (Ctrl+] esce)")
	ips := flag.Int("ips", 200_000, "passi macchina per fotogramma in modalita' interattiva (velocita' percepita)")
	flag.Parse()

	if *bios == "" {
		fmt.Fprintln(os.Stderr, "uso: retronet-pc -bios <rom> [-floppy <img>] [-alu gate|native] [-keys ...] [-steps N] [-live]")
		os.Exit(2)
	}

	m := machine.NewXT()
	if *alu == "gate" {
		m.CPU.SetALU(cpu.Gate)
	} else {
		m.CPU.SetALU(cpu.Native)
	}
	if *video == "cga" {
		m.UseCGA()
	}

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

	if *interactive {
		// In modalita' interattiva maxSteps=0 significa illimitato (lo gestisce
		// runInteractive); non lo si mappa su math.MaxInt.
		if err := runInteractive(m, *steps, *ips); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	maxSteps := *steps
	if maxSteps == 0 {
		maxSteps = math.MaxInt
	}

	var executed int
	if *live {
		// Modalità live: aggiorna lo schermo ogni 10M passi; Ctrl+C per uscire.
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		const chunk = 10_000_000
		for executed < maxSteps {
			select {
			case <-sig:
				goto done
			default:
			}
			limit := chunk
			if maxSteps-executed < chunk {
				limit = maxSteps - executed
			}
			n, runErr := m.Run(limit)
			executed += n
			fmt.Printf("\033[H\033[2J") // clear screen
			cs, ip := m.CPU.Seg[cpu.CS], m.CPU.IP
			fmt.Printf("passi: %d  POST: %02X  CS:IP=%04X:%04X  Halted=%v\n",
				executed, m.Post.Last, cs, ip, m.CPU.Halted)
			base := uint32(cs)<<4 + uint32(ip)
			fmt.Printf("bytes@CS:IP: ")
			for i := uint32(0); i < 16; i++ {
				fmt.Printf("%02X ", m.Mem.Read8(base+i))
			}
			fmt.Println()
			r := m.CPU.Regs
			s := m.CPU.Seg
			fmt.Printf("AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X BP=%04X SP=%04X\n",
				r[cpu.AX], r[cpu.BX], r[cpu.CX], r[cpu.DX], r[cpu.SI], r[cpu.DI], r[cpu.BP], r[cpu.SP])
			fmt.Printf("DS=%04X ES=%04X SS=%04X  phys[BP+DI]=%05X  port(DX)=%04X\n",
				s[cpu.DS], s[cpu.ES], s[cpu.SS],
				uint32(s[cpu.SS])<<4+uint32(r[cpu.BP]+r[cpu.DI]),
				r[cpu.DX])
			fmt.Println("---- schermo ----")
			fmt.Print(m.Screen())
			fmt.Println("-----------------")
			if runErr != nil {
				fmt.Printf("stop: %v\n", runErr)
				return
			}
		}
	done:
		fmt.Printf("\nfermat a %d passi\n", executed)
		return
	}

	executed, err = m.Run(maxSteps)
	fmt.Printf("eseguiti %d passi", executed)
	if err != nil {
		fmt.Printf(" (stop: %v)", err)
	}
	fmt.Printf("; ultimo codice POST: %02X\n", m.Post.Last)
	fmt.Println("---- schermo ----")
	fmt.Print(m.Screen())
	fmt.Println("-----------------")
}
