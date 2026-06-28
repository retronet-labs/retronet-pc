package device

import "strings"

// TextVideo emula un adattatore video dell'IBM PC/XT in modo testo 80x25, gestito
// dal controllore 6845. E' parametrico sulla base della RAM video e sulle porte,
// cosi' la stessa implementazione vale per MDA e CGA:
//
//   - MDA: RAM a 0xB0000, porte 0x3B4 (indice) / 0x3B5 (dato) / 0x3B8 (modo) /
//     0x3BA (stato), monocromatico.
//   - CGA: RAM a 0xB8000, porte 0x3D4 / 0x3D5 / 0x3D8 / 0x3DA, a colori.
//
// Ogni cella e' una coppia di byte (carattere + attributo). Non essendoci un
// display grafico, il rendering produce la griglia 80x25 di caratteri leggendo la
// RAM video dal bus; i modi grafici della CGA non sono resi.
type TextVideo struct {
	index    byte
	crtc     [18]byte // registri del 6845
	mode     byte
	status   byte // alterna i bit di retrace a ogni lettura
	Base     uint32
	portBase uint16
	Columns  int
	Rows     int
	cellMask int // maschera di wrap della pagina video (in celle)
}

// memReader e' la sola capacita' che serve per leggere la RAM video; la soddisfa
// memory.Bus senza creare dipendenze tra i pacchetti.
type memReader interface {
	Read8(addr uint32) byte
}

// NewMDA crea un adattatore MDA (testo monocromatico, RAM a 0xB0000).
func NewMDA() *TextVideo {
	return &TextVideo{Base: 0xB0000, portBase: 0x3B4, Columns: 80, Rows: 25, cellMask: 0x7FF}
}

// NewCGA crea un adattatore CGA (testo a colori, RAM a 0xB8000, pagina da 16 KB).
func NewCGA() *TextVideo {
	return &TextVideo{Base: 0xB8000, portBase: 0x3D4, Columns: 80, Rows: 25, cellMask: 0x1FFF}
}

// Out8 scrive un registro del 6845 o il registro di modo.
func (v *TextVideo) Out8(port uint16, value byte) {
	switch port {
	case v.portBase: // indice
		v.index = value & 0x1F
	case v.portBase + 1: // dato
		if int(v.index) < len(v.crtc) {
			v.crtc[v.index] = value
		}
	case v.portBase + 4: // registro di modo
		v.mode = value
	}
}

// In8 legge un registro del 6845, il modo o lo stato. Lo stato alterna i bit di
// retrace a ogni lettura, cosi' i loop del BIOS che attendono il retrace terminano.
func (v *TextVideo) In8(port uint16) byte {
	switch port {
	case v.portBase:
		return v.index
	case v.portBase + 1:
		if int(v.index) < len(v.crtc) {
			return v.crtc[v.index]
		}
		return 0
	case v.portBase + 4:
		return v.mode
	case v.portBase + 6: // stato
		v.status ^= 0x09 // alterna horizontal retrace (bit0) e video (bit3)
		return v.status
	}
	return 0xFF
}

// CursorOffset restituisce la posizione del cursore in celle (registri R14/R15).
func (v *TextVideo) CursorOffset() int {
	return int(v.crtc[14])<<8 | int(v.crtc[15])
}

// startOffset e' l'indirizzo iniziale di visualizzazione in celle (R12/R13): lo
// usa lo scroll hardware.
func (v *TextVideo) startOffset() int {
	return int(v.crtc[12])<<8 | int(v.crtc[13])
}

// Render legge la RAM video dal bus e restituisce lo schermo come testo (una riga
// per riga). I byte non stampabili diventano spazio o '.'.
func (v *TextVideo) Render(mem memReader) string {
	start := v.startOffset()
	var b strings.Builder
	b.Grow(v.Columns*v.Rows + v.Rows)
	for r := 0; r < v.Rows; r++ {
		for c := 0; c < v.Columns; c++ {
			cell := (start + r*v.Columns + c) & v.cellMask
			b.WriteByte(printable(mem.Read8(v.Base + uint32(cell)*2)))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func printable(ch byte) byte {
	switch {
	case ch == 0x00 || ch == 0x20:
		return ' '
	case ch >= 0x21 && ch <= 0x7E:
		return ch
	default:
		return '.' // glifi della code page 437 non rappresentabili in ASCII
	}
}
