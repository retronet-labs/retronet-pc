package io

import "testing"

type fakeDevice struct {
	in       byte
	lastPort uint16
	lastVal  byte
}

func (f *fakeDevice) In8(port uint16) byte {
	f.lastPort = port
	return f.in
}

func (f *fakeDevice) Out8(port uint16, value byte) {
	f.lastPort = port
	f.lastVal = value
}

func TestPortsDispatch(t *testing.T) {
	p := New()
	f := &fakeDevice{in: 0x5A}
	p.Map(0x60, 0x63, f)

	if got := p.In8(0x61); got != 0x5A {
		t.Errorf("In8(0x61) = %#02x, atteso 0x5A", got)
	}
	if f.lastPort != 0x61 {
		t.Errorf("porta letta = %#x, attesa 0x61", f.lastPort)
	}
	p.Out8(0x62, 0x99)
	if f.lastVal != 0x99 || f.lastPort != 0x62 {
		t.Errorf("Out8: porta=%#x val=%#02x", f.lastPort, f.lastVal)
	}
}

func TestUnmappedPort(t *testing.T) {
	p := New()
	if got := p.In8(0x0070); got != 0xFF {
		t.Errorf("porta non mappata = %#02x, atteso 0xFF", got)
	}
	p.Out8(0x0070, 0x12) // non deve andare in panico
}
