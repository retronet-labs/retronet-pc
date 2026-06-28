// Package disk modella i dischetti dell'IBM PC/XT come immagini raw, con la
// geometria (cilindri, testine, settori) e la conversione CHS -> offset usata dal
// controllore floppy.
package disk

import "fmt"

// Geometry descrive la disposizione fisica di un floppy.
type Geometry struct {
	Cylinders, Heads, Sectors, SectorSize int
}

// Bytes restituisce la dimensione totale dell'immagine implicata dalla geometria.
func (g Geometry) Bytes() int {
	return g.Cylinders * g.Heads * g.Sectors * g.SectorSize
}

// Geometrie standard riconosciute dalla dimensione dell'immagine.
var standardGeometries = []Geometry{
	{40, 2, 9, 512},  // 360 KB
	{80, 2, 9, 512},  // 720 KB
	{80, 2, 15, 512}, // 1.2 MB
	{80, 2, 18, 512}, // 1.44 MB
}

// Floppy e' un'immagine floppy raw con la sua geometria.
type Floppy struct {
	data []byte
	Geo  Geometry
}

// NewFloppy avvolge un'immagine raw, deducendo la geometria dalla dimensione.
// Restituisce errore se la dimensione non corrisponde a un formato noto.
func NewFloppy(data []byte) (*Floppy, error) {
	for _, g := range standardGeometries {
		if g.Bytes() == len(data) {
			return &Floppy{data: data, Geo: g}, nil
		}
	}
	return nil, fmt.Errorf("dimensione immagine non riconosciuta: %d byte", len(data))
}

// offset calcola la posizione del settore CHS (settori 1-based) nell'immagine.
func (f *Floppy) offset(cyl, head, sector int) (int, error) {
	g := f.Geo
	if cyl < 0 || cyl >= g.Cylinders || head < 0 || head >= g.Heads ||
		sector < 1 || sector > g.Sectors {
		return 0, fmt.Errorf("CHS fuori geometria: c=%d h=%d s=%d", cyl, head, sector)
	}
	lba := (cyl*g.Heads+head)*g.Sectors + (sector - 1)
	return lba * g.SectorSize, nil
}

// ReadSector restituisce i byte del settore CHS.
func (f *Floppy) ReadSector(cyl, head, sector int) ([]byte, error) {
	off, err := f.offset(cyl, head, sector)
	if err != nil {
		return nil, err
	}
	out := make([]byte, f.Geo.SectorSize)
	copy(out, f.data[off:off+f.Geo.SectorSize])
	return out, nil
}

// WriteSector scrive i byte nel settore CHS (al massimo SectorSize byte).
func (f *Floppy) WriteSector(cyl, head, sector int, data []byte) error {
	off, err := f.offset(cyl, head, sector)
	if err != nil {
		return err
	}
	copy(f.data[off:off+f.Geo.SectorSize], data)
	return nil
}
