// Package io implementa lo spazio di I/O dell'IBM PC/XT: un dispatcher che
// instrada letture e scritture verso le periferiche mappate per intervallo di
// porte. Soddisfa l'interfaccia cpu.Ports di retronet-8086.
package io

// Device e' una periferica collegata al bus I/O.
type Device interface {
	In8(port uint16) byte
	Out8(port uint16, value byte)
}

type mapping struct {
	lo, hi uint16
	dev    Device
}

// Ports e' il dispatcher dell'I/O: mantiene gli intervalli di porte mappati e vi
// instrada gli accessi. Le porte non mappate leggono 0xFF (bus a riposo) e
// ignorano le scritture.
type Ports struct {
	maps []mapping
}

// New crea un dispatcher vuoto.
func New() *Ports { return &Ports{} }

// Map collega dev all'intervallo di porte [lo, hi].
func (p *Ports) Map(lo, hi uint16, dev Device) {
	p.maps = append(p.maps, mapping{lo, hi, dev})
}

// In8 instrada una lettura; 0xFF se nessuna periferica copre la porta.
func (p *Ports) In8(port uint16) byte {
	if d := p.find(port); d != nil {
		return d.In8(port)
	}
	return 0xFF
}

// Out8 instrada una scrittura; ignorata se nessuna periferica copre la porta.
func (p *Ports) Out8(port uint16, value byte) {
	if d := p.find(port); d != nil {
		d.Out8(port, value)
	}
}

func (p *Ports) find(port uint16) Device {
	for _, m := range p.maps {
		if port >= m.lo && port <= m.hi {
			return m.dev
		}
	}
	return nil
}
