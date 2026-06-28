package device

import "strings"

// MDA emula il Monochrome Display Adapter dell'IBM PC/XT: solo testo, 80x25, con
// il controllore video 6845. La memoria video sta a 0xB0000 (4 KB): ogni cella e'
// una coppia di byte (carattere + attributo). I registri del 6845 sono accessibili
// dalle porte 0x3B4 (indice) / 0x3B5 (dato); 0x3B8 e' il registro di modo, 0x3BA
// lo stato (con i bit di retrace).
//
// Non essendoci un display grafico, il "rendering" produce la griglia 80x25 di
// caratteri leggendo la RAM video dal bus: cosi' si osserva a schermo l'output del
// BIOS e dei programmi.
type MDA struct {
	index   byte
	crtc    [18]byte // registri del 6845
	mode    byte
	status  byte // stato 0x3BA, alterna i bit di retrace a ogni lettura
	Base    uint32
	Columns int
	Rows    int
}

// memReader e' la sola capacita' che serve all'MDA per leggere la RAM video; la
// soddisfa memory.Bus senza creare dipendenze tra i pacchetti.
type memReader interface {
	Read8(addr uint32) byte
}

// NewMDA crea un adattatore MDA con buffer a 0xB0000 e schermo 80x25.
func NewMDA() *MDA {
	return &MDA{Base: 0xB0000, Columns: 80, Rows: 25}
}

// Out8 scrive un registro del 6845 o il registro di modo.
func (m *MDA) Out8(port uint16, value byte) {
	switch port {
	case 0x3B4:
		m.index = value & 0x1F
	case 0x3B5:
		if int(m.index) < len(m.crtc) {
			m.crtc[m.index] = value
		}
	case 0x3B8:
		m.mode = value
	}
}

// In8 legge un registro del 6845, il modo o lo stato. Lo stato alterna i bit di
// retrace a ogni lettura, cosi' i loop del BIOS che attendono il retrace terminano.
func (m *MDA) In8(port uint16) byte {
	switch port {
	case 0x3B4:
		return m.index
	case 0x3B5:
		if int(m.index) < len(m.crtc) {
			return m.crtc[m.index]
		}
		return 0
	case 0x3B8:
		return m.mode
	case 0x3BA:
		m.status ^= 0x09 // alterna horizontal retrace (bit0) e video (bit3)
		return m.status
	}
	return 0xFF
}

// CursorOffset restituisce la posizione del cursore in celle (registri R14/R15).
func (m *MDA) CursorOffset() int {
	return int(m.crtc[14])<<8 | int(m.crtc[15])
}

// startOffset e' l'indirizzo iniziale di visualizzazione in celle (R12/R13): lo
// usa lo scroll hardware.
func (m *MDA) startOffset() int {
	return int(m.crtc[12])<<8 | int(m.crtc[13])
}

// Render legge la RAM video dal bus e restituisce lo schermo come testo (una riga
// per riga dello schermo). I byte non stampabili diventano spazio o '.'.
func (m *MDA) Render(mem memReader) string {
	cells := m.Columns * m.Rows
	start := m.startOffset()
	var b strings.Builder
	b.Grow(cells + m.Rows)
	for r := 0; r < m.Rows; r++ {
		for c := 0; c < m.Columns; c++ {
			cell := (start + r*m.Columns + c) & 0x7FF // 4 KB = 2048 celle, con wrap
			ch := mem.Read8(m.Base + uint32(cell)*2)
			b.WriteByte(printable(ch))
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
