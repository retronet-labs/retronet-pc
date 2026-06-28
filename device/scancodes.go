package device

// Codici di scansione (set 1, layout US). Per ogni tasto: il make code (alla
// pressione) e il break code (make | 0x80, al rilascio). I caratteri che sul
// layout US richiedono lo Shift (maiuscole e simboli) vengono inviati racchiusi
// tra make e break del tasto Shift sinistro.

const (
	scShiftMake  = 0x2A
	scShiftBreak = 0xAA
)

// keyDef e' il codice di scansione di un carattere e se serve lo Shift.
type keyDef struct {
	code  byte
	shift bool
}

// usKeyrow descrive un tasto con il carattere non-shiftato e quello shiftato.
type usKeyrow struct {
	code   byte
	lo, hi byte
}

var usKeyrows = []usKeyrow{
	{0x02, '1', '!'}, {0x03, '2', '@'}, {0x04, '3', '#'}, {0x05, '4', '$'},
	{0x06, '5', '%'}, {0x07, '6', '^'}, {0x08, '7', '&'}, {0x09, '8', '*'},
	{0x0A, '9', '('}, {0x0B, '0', ')'}, {0x0C, '-', '_'}, {0x0D, '=', '+'},
	{0x10, 'q', 'Q'}, {0x11, 'w', 'W'}, {0x12, 'e', 'E'}, {0x13, 'r', 'R'},
	{0x14, 't', 'T'}, {0x15, 'y', 'Y'}, {0x16, 'u', 'U'}, {0x17, 'i', 'I'},
	{0x18, 'o', 'O'}, {0x19, 'p', 'P'}, {0x1A, '[', '{'}, {0x1B, ']', '}'},
	{0x1E, 'a', 'A'}, {0x1F, 's', 'S'}, {0x20, 'd', 'D'}, {0x21, 'f', 'F'},
	{0x22, 'g', 'G'}, {0x23, 'h', 'H'}, {0x24, 'j', 'J'}, {0x25, 'k', 'K'},
	{0x26, 'l', 'L'}, {0x27, ';', ':'}, {0x28, '\'', '"'}, {0x29, '`', '~'},
	{0x2B, '\\', '|'}, {0x2C, 'z', 'Z'}, {0x2D, 'x', 'X'}, {0x2E, 'c', 'C'},
	{0x2F, 'v', 'V'}, {0x30, 'b', 'B'}, {0x31, 'n', 'N'}, {0x32, 'm', 'M'},
	{0x33, ',', '<'}, {0x34, '.', '>'}, {0x35, '/', '?'}, {0x39, ' ', ' '},
}

// keymap mappa un carattere ASCII al suo tasto (codice + necessita' di Shift).
var keymap = buildKeymap()

func buildKeymap() map[byte]keyDef {
	m := make(map[byte]keyDef)
	for _, k := range usKeyrows {
		m[k.lo] = keyDef{k.code, false}
		if k.hi != k.lo {
			m[k.hi] = keyDef{k.code, true}
		}
	}
	// Tasti di controllo comuni.
	m['\n'] = keyDef{0x1C, false}
	m['\r'] = keyDef{0x1C, false}
	m['\b'] = keyDef{0x0E, false}
	m['\t'] = keyDef{0x0F, false}
	return m
}

// Type "digita" una stringa accodando i codici di scansione: i caratteri
// shiftati vengono racchiusi tra make e break dello Shift.
func (p *PPI) Type(s string) {
	for i := 0; i < len(s); i++ {
		k, ok := keymap[s[i]]
		if !ok {
			continue
		}
		if k.shift {
			p.PressScancode(scShiftMake)
		}
		p.PressScancode(k.code)        // make
		p.PressScancode(k.code | 0x80) // break
		if k.shift {
			p.PressScancode(scShiftBreak)
		}
	}
}
