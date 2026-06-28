// Package device raccoglie le periferiche dell'IBM PC/XT collegate al bus I/O.
package device

// PostCode modella il latch diagnostico sulla porta 0x80: il POST del BIOS vi
// scrive i codici di avanzamento (una scheda POST li mostra su due cifre esa).
// Qui ne conserviamo l'ultimo valore, utile per osservare l'avvio.
type PostCode struct {
	Last    byte
	Written bool
}

// In8 rilegge l'ultimo codice scritto.
func (p *PostCode) In8(uint16) byte { return p.Last }

// Out8 registra il codice POST.
func (p *PostCode) Out8(_ uint16, value byte) {
	p.Last = value
	p.Written = true
}
