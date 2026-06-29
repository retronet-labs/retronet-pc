package device

// Codici di scansione (set 1, layout US). Per ogni tasto: il make code (alla
// pressione) e il break code (make | 0x80, al rilascio). I tasti "grigi" estesi
// (frecce, Home/End, Ins/Del, ...) sono preceduti dal prefisso 0xE0 sia sul make
// sia sul break. I modificatori (Shift/Ctrl/Alt) racchiudono il tasto tra il
// proprio make e il proprio break.

const (
	scShiftMake  = 0x2A
	scShiftBreak = 0xAA
	scCtrlMake   = 0x1D
	scCtrlBreak  = 0x9D
	scAltMake    = 0x38
	scAltBreak   = 0xB8

	scExtended = 0xE0 // prefisso dei tasti estesi
)

// KeyMods e' l'insieme dei modificatori premuti insieme a un tasto.
type KeyMods uint8

const (
	ModShift KeyMods = 1 << iota
	ModCtrl
	ModAlt
)

// Key e' un tasto logico della tastiera XT: il suo make code del set 1 e se e' un
// tasto esteso (consegnato con il prefisso 0xE0).
type Key struct {
	code     byte
	extended bool
}

// Tasti con nome, per pilotare la tastiera oltre il semplice testo: tasti
// funzione, tasti di navigazione estesi e tasti di controllo comuni.
var (
	KeyEsc       = Key{code: 0x01}
	KeyBackspace = Key{code: 0x0E}
	KeyTab       = Key{code: 0x0F}
	KeyEnter     = Key{code: 0x1C}
	KeySpace     = Key{code: 0x39}
	KeyCapsLock  = Key{code: 0x3A}

	KeyF1  = Key{code: 0x3B}
	KeyF2  = Key{code: 0x3C}
	KeyF3  = Key{code: 0x3D}
	KeyF4  = Key{code: 0x3E}
	KeyF5  = Key{code: 0x3F}
	KeyF6  = Key{code: 0x40}
	KeyF7  = Key{code: 0x41}
	KeyF8  = Key{code: 0x42}
	KeyF9  = Key{code: 0x43}
	KeyF10 = Key{code: 0x44}
	KeyF11 = Key{code: 0x57}
	KeyF12 = Key{code: 0x58}

	// Tasti del tastierino di navigazione dedicato: codici estesi (prefisso 0xE0).
	KeyUp       = Key{code: 0x48, extended: true}
	KeyDown     = Key{code: 0x50, extended: true}
	KeyLeft     = Key{code: 0x4B, extended: true}
	KeyRight    = Key{code: 0x4D, extended: true}
	KeyHome     = Key{code: 0x47, extended: true}
	KeyEnd      = Key{code: 0x4F, extended: true}
	KeyPageUp   = Key{code: 0x49, extended: true}
	KeyPageDown = Key{code: 0x51, extended: true}
	KeyInsert   = Key{code: 0x52, extended: true}
	KeyDelete   = Key{code: 0x53, extended: true}
)

// usKeyrow descrive un tasto alfanumerico con il carattere non-shiftato e quello
// shiftato del layout US.
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

// charKey descrive il tasto (e l'eventuale Shift) che produce un carattere.
type charKey struct {
	code  byte
	shift bool
}

// keymap mappa un carattere ASCII stampabile al suo tasto (codice + Shift).
var keymap = buildKeymap()

func buildKeymap() map[byte]charKey {
	m := make(map[byte]charKey)
	for _, k := range usKeyrows {
		m[k.lo] = charKey{k.code, false}
		if k.hi != k.lo {
			m[k.hi] = charKey{k.code, true}
		}
	}
	return m
}

// PressKey consegna alla tastiera un tasto con i suoi modificatori: emette i make
// dei modificatori (Shift, Ctrl, Alt), poi il make/break del tasto (preceduto da
// 0xE0 se esteso), poi i break dei modificatori in ordine inverso.
func (p *PPI) PressKey(k Key, mods KeyMods) {
	if mods&ModShift != 0 {
		p.PressScancode(scShiftMake)
	}
	if mods&ModCtrl != 0 {
		p.PressScancode(scCtrlMake)
	}
	if mods&ModAlt != 0 {
		p.PressScancode(scAltMake)
	}
	if k.extended {
		p.PressScancode(scExtended)
	}
	p.PressScancode(k.code) // make
	if k.extended {
		p.PressScancode(scExtended)
	}
	p.PressScancode(k.code | 0x80) // break
	if mods&ModAlt != 0 {
		p.PressScancode(scAltBreak)
	}
	if mods&ModCtrl != 0 {
		p.PressScancode(scCtrlBreak)
	}
	if mods&ModShift != 0 {
		p.PressScancode(scShiftBreak)
	}
}

// Type "digita" una stringa accodando i codici di scansione. I caratteri
// stampabili usano lo Shift quando serve; i caratteri di controllo ASCII sono
// tradotti: TAB/INVIO/BACKSPACE/ESC nei rispettivi tasti, e Ctrl-A..Ctrl-Z (0x01
// .. 0x1A) come Ctrl + lettera.
func (p *PPI) Type(s string) {
	for i := 0; i < len(s); i++ {
		switch b := s[i]; b {
		case '\n', '\r':
			p.PressKey(KeyEnter, 0)
		case '\b':
			p.PressKey(KeyBackspace, 0)
		case '\t':
			p.PressKey(KeyTab, 0)
		case 0x1B:
			p.PressKey(KeyEsc, 0)
		default:
			if b >= 0x01 && b <= 0x1A { // Ctrl-A .. Ctrl-Z
				if k, ok := keymap['a'+b-1]; ok {
					p.PressKey(Key{code: k.code}, ModCtrl)
				}
				continue
			}
			if k, ok := keymap[b]; ok {
				var mods KeyMods
				if k.shift {
					mods = ModShift
				}
				p.PressKey(Key{code: k.code}, mods)
			}
		}
	}
}
