// Package memory implementa il bus di memoria a 1 MB dell'IBM PC/XT: una memoria
// piatta con regioni di sola lettura (ROM) per il BIOS e le eventuali option ROM.
// Soddisfa l'interfaccia cpu.Bus di retronet-8086, cosi' la CPU vi accede senza
// conoscere la mappa della macchina.
package memory

// Size e' lo spazio indirizzabile dell'8088: 1 MB (20 bit).
const Size = 1 << 20

const mask = Size - 1

// romRange e' un intervallo di indirizzi protetto da scrittura.
type romRange struct{ lo, hi uint32 }

// Bus e' la memoria della macchina: backing piatto da 1 MB piu' l'elenco delle
// regioni ROM (scritture ignorate).
type Bus struct {
	data [Size]byte
	rom  []romRange
}

// New crea un bus azzerato, tutto RAM scrivibile.
func New() *Bus { return &Bus{} }

// Read8 legge un byte (indirizzo mascherato a 20 bit).
func (b *Bus) Read8(addr uint32) byte { return b.data[addr&mask] }

// Write8 scrive un byte; le scritture nelle regioni ROM sono ignorate, come
// sull'hardware.
func (b *Bus) Write8(addr uint32, value byte) {
	addr &= mask
	if b.isROM(addr) {
		return
	}
	b.data[addr] = value
}

// LoadRAM copia data a partire da addr senza proteggere la regione (RAM, video).
func (b *Bus) LoadRAM(addr uint32, data []byte) {
	for i, v := range data {
		b.data[(addr+uint32(i))&mask] = v
	}
}

// LoadROM copia data a partire da addr e marca l'intervallo come sola lettura
// (BIOS, option ROM): le scritture successive verranno ignorate.
func (b *Bus) LoadROM(addr uint32, data []byte) {
	if len(data) == 0 {
		return
	}
	b.LoadRAM(addr, data)
	b.rom = append(b.rom, romRange{addr & mask, (addr + uint32(len(data)) - 1) & mask})
}

func (b *Bus) isROM(addr uint32) bool {
	for _, r := range b.rom {
		if addr >= r.lo && addr <= r.hi {
			return true
		}
	}
	return false
}
