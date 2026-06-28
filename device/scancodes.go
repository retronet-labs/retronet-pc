package device

// Codici di scansione (set 1, layout US) per i tasti ASCII piu' comuni: il make
// code si invia alla pressione, il break code (make | 0x80) al rilascio. La
// tabella copre lettere, cifre, spazio, invio e qualche simbolo; le maiuscole
// sono trattate come minuscole (niente Shift), sufficiente per digitare comandi.
var asciiScancode = map[byte]byte{
	'1': 0x02, '2': 0x03, '3': 0x04, '4': 0x05, '5': 0x06,
	'6': 0x07, '7': 0x08, '8': 0x09, '9': 0x0A, '0': 0x0B,
	'-': 0x0C, '=': 0x0D,
	'q': 0x10, 'w': 0x11, 'e': 0x12, 'r': 0x13, 't': 0x14,
	'y': 0x15, 'u': 0x16, 'i': 0x17, 'o': 0x18, 'p': 0x19,
	'a': 0x1E, 's': 0x1F, 'd': 0x20, 'f': 0x21, 'g': 0x22,
	'h': 0x23, 'j': 0x24, 'k': 0x25, 'l': 0x26,
	'z': 0x2C, 'x': 0x2D, 'c': 0x2E, 'v': 0x2F, 'b': 0x30,
	'n': 0x31, 'm': 0x32, ',': 0x33, '.': 0x34, '/': 0x35,
	' ': 0x39, '\n': 0x1C, '\r': 0x1C, '\b': 0x0E, '\t': 0x0F,
}

// Type "digita" una stringa: per ogni carattere riconosciuto accoda il make e il
// break code. Le maiuscole sono mappate alle minuscole.
func (p *PPI) Type(s string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if code, ok := asciiScancode[c]; ok {
			p.PressScancode(code)        // make
			p.PressScancode(code | 0x80) // break
		}
	}
}
