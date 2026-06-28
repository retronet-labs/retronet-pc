package disk

import (
	"bytes"
	"testing"
)

func TestGeometryFromSize(t *testing.T) {
	cases := []struct {
		size  int
		cyl   int
		sects int
	}{
		{360 * 1024, 40, 9},
		{720 * 1024, 80, 9},
		{1200 * 1024, 80, 15},
		{1440 * 1024, 80, 18},
	}
	for _, c := range cases {
		f, err := NewFloppy(make([]byte, c.size))
		if err != nil {
			t.Fatalf("%d byte: %v", c.size, err)
		}
		if f.Geo.Cylinders != c.cyl || f.Geo.Sectors != c.sects {
			t.Errorf("%d byte: geometria %dx%d, attesa %d cilindri %d settori", c.size, f.Geo.Cylinders, f.Geo.Sectors, c.cyl, c.sects)
		}
	}
	if _, err := NewFloppy(make([]byte, 12345)); err == nil {
		t.Error("dimensione non standard dovrebbe dare errore")
	}
}

func TestReadWriteSector(t *testing.T) {
	f, _ := NewFloppy(make([]byte, 360*1024))
	data := bytes.Repeat([]byte{0xAB}, 512)
	if err := f.WriteSector(1, 1, 5, data); err != nil {
		t.Fatal(err)
	}
	got, err := f.ReadSector(1, 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Error("settore riletto diverso da quello scritto")
	}
	if _, err := f.ReadSector(0, 0, 0); err == nil {
		t.Error("il settore 0 (i settori sono 1-based) deve dare errore")
	}
}
