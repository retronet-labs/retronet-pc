package disk

import (
	"fmt"
	"io"
)

// SectorSize e' la dimensione di settore standard usata da floppy e dischi fissi.
const SectorSize = 512

// sectorRW e' il backing store ad accesso casuale di un disco fisso: lo soddisfano
// sia *os.File (disco persistente) sia un buffer in memoria (test, dischi volatili).
type sectorRW interface {
	io.ReaderAt
	io.WriterAt
}

// HardDisk e' un disco fisso indirizzabile per LBA, con una geometria CHS dedotta
// dal numero di settori (per i servizi BIOS/ATA che usano ancora il CHS).
type HardDisk struct {
	rw      sectorRW
	geo     Geometry
	sectors uint32
}

// hddGeometry deduce una geometria CHS "logica" classica (16 testine, 63 settori)
// dal numero totale di settori, limitando i cilindri al massimo ATA (16383).
func hddGeometry(totalSectors uint32) Geometry {
	const heads, spt = 16, 63
	c := int(totalSectors) / (heads * spt)
	switch {
	case c < 1:
		c = 1
	case c > 16383:
		c = 16383
	}
	return Geometry{Cylinders: c, Heads: heads, Sectors: spt, SectorSize: SectorSize}
}

// NewHardDisk avvolge un backing store come disco fisso di totalSectors settori.
func NewHardDisk(rw sectorRW, totalSectors uint32) *HardDisk {
	return &HardDisk{rw: rw, geo: hddGeometry(totalSectors), sectors: totalSectors}
}

// Sectors restituisce il numero totale di settori del disco.
func (d *HardDisk) Sectors() uint32 { return d.sectors }

// Geometry restituisce la geometria CHS logica.
func (d *HardDisk) Geometry() Geometry { return d.geo }

// ReadLBA legge count settori a partire dal blocco logico lba.
func (d *HardDisk) ReadLBA(lba uint32, count int) ([]byte, error) {
	if count <= 0 || uint32(count) > d.sectors-lba || lba >= d.sectors {
		return nil, fmt.Errorf("lettura fuori disco: lba=%d count=%d (settori=%d)", lba, count, d.sectors)
	}
	buf := make([]byte, count*SectorSize)
	if _, err := d.rw.ReadAt(buf, int64(lba)*SectorSize); err != nil && err != io.EOF {
		return nil, err
	}
	return buf, nil
}

// WriteLBA scrive i settori (len(data) multiplo di 512) a partire da lba.
func (d *HardDisk) WriteLBA(lba uint32, data []byte) error {
	count := len(data) / SectorSize
	if count <= 0 || uint32(count) > d.sectors-lba || lba >= d.sectors {
		return fmt.Errorf("scrittura fuori disco: lba=%d count=%d (settori=%d)", lba, count, d.sectors)
	}
	_, err := d.rw.WriteAt(data[:count*SectorSize], int64(lba)*SectorSize)
	return err
}

// MemDisk e' un backing store in memoria (test e dischi volatili).
type MemDisk struct{ Data []byte }

// NewMemDisk crea un disco in memoria di sectors settori (azzerato).
func NewMemDisk(sectors uint32) *MemDisk {
	return &MemDisk{Data: make([]byte, int(sectors)*SectorSize)}
}

func (m *MemDisk) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(m.Data)) {
		return 0, io.EOF
	}
	n := copy(p, m.Data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (m *MemDisk) WriteAt(p []byte, off int64) (int, error) {
	if off+int64(len(p)) > int64(len(m.Data)) {
		return 0, fmt.Errorf("scrittura oltre il disco in memoria")
	}
	return copy(m.Data[off:], p), nil
}
