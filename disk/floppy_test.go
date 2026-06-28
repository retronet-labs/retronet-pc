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
	// Un'immagine piu' piccola (es. boot sector) viene riempita al formato minimo.
	boot := make([]byte, 512)
	boot[0] = 0xEB
	f, err := NewFloppy(boot)
	if err != nil {
		t.Fatalf("boot sector: %v", err)
	}
	if f.Geo.Bytes() != 360*1024 {
		t.Errorf("boot sector riempito a %d byte, atteso 360 KB", f.Geo.Bytes())
	}
	if s, _ := f.ReadSector(0, 0, 1); s[0] != 0xEB {
		t.Errorf("primo byte del settore di boot perso nel padding")
	}
	if _, err := NewFloppy(make([]byte, 2_000_000)); err == nil {
		t.Error("immagine piu' grande del massimo dovrebbe dare errore")
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
