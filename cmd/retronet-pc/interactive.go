package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/retronet-labs/retronet-pc/device"
	"github.com/retronet-labs/retronet-pc/machine"
)

// runInteractive avvia la macchina in modalita' interattiva: schermo ridisegnato a
// ~60 Hz e input da tastiera reale (raw mode su stdin). Esce con Ctrl+] o al
// raggiungimento di maxSteps (0 = illimitato). stepsPerFrame regola quanti passi
// macchina eseguire per fotogramma (la velocita' percepita del PC).
func runInteractive(m *machine.Machine, maxSteps, stepsPerFrame int) error {
	restore, err := enterRawMode(os.Stdin)
	if err != nil {
		return fmt.Errorf("modalita' interattiva non disponibile (serve un terminale): %w", err)
	}
	defer restore()

	// I byte di stdin arrivano su un canale da un goroutine dedicato, cosi' il ciclo
	// principale non si blocca mai in lettura.
	keys := make(chan byte, 256)
	go func() {
		buf := make([]byte, 64)
		for {
			n, err := os.Stdin.Read(buf)
			for i := 0; i < n; i++ {
				keys <- buf[i]
			}
			if err != nil {
				close(keys)
				return
			}
		}
	}()

	out := os.Stdout
	fmt.Fprint(out, "\x1b[2J") // pulisci lo schermo
	defer fmt.Fprint(out, "\x1b[?25h\r\n")

	var tr keyTranslator
	var lastScreen string
	executed := 0
	unlimited := maxSteps == 0

	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	for range ticker.C {
		// Consuma senza bloccare tutti i tasti gia' arrivati.
	drain:
		for {
			select {
			case b, ok := <-keys:
				if !ok {
					return nil // stdin chiuso
				}
				tr.feed(m.Ppi, b)
			default:
				break drain
			}
		}
		tr.flush(m.Ppi)
		if tr.quit {
			return nil
		}

		budget := stepsPerFrame
		if !unlimited && maxSteps-executed < budget {
			budget = maxSteps - executed
		}
		if budget > 0 {
			n, runErr := m.Run(budget)
			executed += n
			if runErr != nil {
				renderFrame(out, m, &lastScreen, true)
				return runErr
			}
		}
		renderFrame(out, m, &lastScreen, false)
		if !unlimited && executed >= maxSteps {
			return nil
		}
	}
	return nil
}

// renderFrame ridisegna lo schermo testuale solo se e' cambiato (per evitare lo
// sfarfallio), poi posiziona il cursore dove lo tiene il 6845.
func renderFrame(out *os.File, m *machine.Machine, last *string, force bool) {
	screen := m.Screen()
	if screen == *last && !force {
		return
	}
	*last = screen

	var b strings.Builder
	b.WriteString("\x1b[?25l\x1b[H") // nascondi cursore, vai a casa
	rows := strings.Split(strings.TrimRight(screen, "\n"), "\n")
	for i, row := range rows {
		if i > 0 {
			b.WriteString("\r\n")
		}
		b.WriteString("\x1b[2K") // cancella la riga
		b.WriteString(row)
	}
	b.WriteString("\r\n\x1b[2K\x1b[7m Ctrl+] esce \x1b[0m")
	if v := m.Video; v != nil {
		if r, c, visible := v.CursorPos(); visible {
			fmt.Fprintf(&b, "\x1b[%d;%dH\x1b[?25h", r+1, c+1)
		}
	}
	_, _ = out.WriteString(b.String())
}

// keyState e' lo stato del parser delle sequenze di escape del terminale.
type keyState int

const (
	ksNormal keyState = iota
	ksEsc             // ricevuto ESC
	ksCSI             // ricevuto ESC [
	ksSS3             // ricevuto ESC O
)

// keyTranslator converte i byte del terminale ospite in pressioni di tasti sulla
// tastiera emulata: testo e Ctrl-x via Type, sequenze ANSI (frecce e navigazione)
// nei tasti estesi. Ctrl+] chiede l'uscita.
type keyTranslator struct {
	state  keyState
	params []byte
	quit   bool
}

func (tr *keyTranslator) feed(p *device.PPI, b byte) {
	switch tr.state {
	case ksNormal:
		switch b {
		case 0x1B: // ESC: forse inizio di una sequenza
			tr.state = ksEsc
		case 0x1D: // Ctrl+] : uscita dall'emulatore
			tr.quit = true
		case 0x7F: // molti terminali mandano DEL per il Backspace
			p.PressKey(device.KeyBackspace, 0)
		default:
			p.Type(string(b)) // stampabili, Ctrl-A..Z, \r \t \b
		}
	case ksEsc:
		switch b {
		case '[':
			tr.params = tr.params[:0]
			tr.state = ksCSI
		case 'O':
			tr.state = ksSS3
		default:
			// ESC isolato seguito da altro: invia Esc e ritratta il byte come normale.
			p.PressKey(device.KeyEsc, 0)
			tr.state = ksNormal
			tr.feed(p, b)
		}
	case ksCSI:
		if (b >= '0' && b <= '9') || b == ';' {
			tr.params = append(tr.params, b)
			return
		}
		tr.csiFinal(p, b)
		tr.state = ksNormal
	case ksSS3:
		tr.arrowOrFn(p, b)
		tr.state = ksNormal
	}
}

// flush risolve un ESC rimasto isolato a fine drain in una pressione del tasto Esc.
func (tr *keyTranslator) flush(p *device.PPI) {
	if tr.state == ksEsc {
		p.PressKey(device.KeyEsc, 0)
		tr.state = ksNormal
	}
}

func (tr *keyTranslator) csiFinal(p *device.PPI, final byte) {
	if final == '~' {
		switch string(tr.params) {
		case "1", "7":
			p.PressKey(device.KeyHome, 0)
		case "4", "8":
			p.PressKey(device.KeyEnd, 0)
		case "2":
			p.PressKey(device.KeyInsert, 0)
		case "3":
			p.PressKey(device.KeyDelete, 0)
		case "5":
			p.PressKey(device.KeyPageUp, 0)
		case "6":
			p.PressKey(device.KeyPageDown, 0)
		}
		return
	}
	tr.arrowOrFn(p, final)
}

// arrowOrFn mappa i byte finali comuni a CSI (ESC [) e SS3 (ESC O): frecce,
// Home/End e i tasti funzione F1-F4 (forma SS3).
func (tr *keyTranslator) arrowOrFn(p *device.PPI, final byte) {
	switch final {
	case 'A':
		p.PressKey(device.KeyUp, 0)
	case 'B':
		p.PressKey(device.KeyDown, 0)
	case 'C':
		p.PressKey(device.KeyRight, 0)
	case 'D':
		p.PressKey(device.KeyLeft, 0)
	case 'H':
		p.PressKey(device.KeyHome, 0)
	case 'F':
		p.PressKey(device.KeyEnd, 0)
	case 'P':
		p.PressKey(device.KeyF1, 0)
	case 'Q':
		p.PressKey(device.KeyF2, 0)
	case 'R':
		p.PressKey(device.KeyF3, 0)
	case 'S':
		p.PressKey(device.KeyF4, 0)
	}
}
