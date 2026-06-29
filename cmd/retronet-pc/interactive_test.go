package main

import (
	"testing"

	"github.com/retronet-labs/retronet-pc/device"
)

// drainScancodes legge n codici di scansione dalla PPI simulando l'handshake INT9
// (lettura Port A + impulso d'ack PB7), con il ritardo di trasmissione tra uno e
// l'altro.
func drainScancodes(p *device.PPI, n int) []byte {
	var got []byte
	for i := 0; i < n; i++ {
		got = append(got, p.In8(0x60))
		p.Out8(0x61, 0x80) // ack
		p.Out8(0x61, 0x00) // riabilita -> avvia il ritardo
		p.Tick(20000)      // > kbReloadDelay: presenta il prossimo
	}
	return got
}

func feedAll(tr *keyTranslator, p *device.PPI, s string) {
	for i := 0; i < len(s); i++ {
		tr.feed(p, s[i])
	}
	tr.flush(p)
}

func TestTranslatorArrowCSI(t *testing.T) {
	p := device.NewPPI()
	var tr keyTranslator
	feedAll(&tr, p, "\x1b[A") // freccia su
	got := drainScancodes(p, 4)
	want := []byte{0xE0, 0x48, 0xE0, 0xC8}
	if string(got) != string(want) {
		t.Errorf("ESC[A = % X, atteso % X (Up esteso)", got, want)
	}
}

func TestTranslatorNavTilde(t *testing.T) {
	p := device.NewPPI()
	var tr keyTranslator
	feedAll(&tr, p, "\x1b[3~") // Canc
	got := drainScancodes(p, 4)
	want := []byte{0xE0, 0x53, 0xE0, 0xD3}
	if string(got) != string(want) {
		t.Errorf("ESC[3~ = % X, atteso % X (Delete esteso)", got, want)
	}
}

func TestTranslatorFunctionSS3(t *testing.T) {
	p := device.NewPPI()
	var tr keyTranslator
	feedAll(&tr, p, "\x1bOP") // F1 (forma SS3)
	got := drainScancodes(p, 2)
	want := []byte{0x3B, 0xBB}
	if string(got) != string(want) {
		t.Errorf("ESC O P = % X, atteso % X (F1)", got, want)
	}
}

func TestTranslatorLoneEsc(t *testing.T) {
	p := device.NewPPI()
	var tr keyTranslator
	feedAll(&tr, p, "\x1b") // ESC isolato -> risolto da flush
	got := drainScancodes(p, 2)
	want := []byte{0x01, 0x81}
	if string(got) != string(want) {
		t.Errorf("ESC isolato = % X, atteso % X (tasto Esc)", got, want)
	}
}

func TestTranslatorPlainAndCtrl(t *testing.T) {
	p := device.NewPPI()
	var tr keyTranslator
	feedAll(&tr, p, "a")
	if got := drainScancodes(p, 2); string(got) != string([]byte{0x1E, 0x9E}) {
		t.Errorf("'a' = % X, atteso 1E 9E", got)
	}
	p = device.NewPPI()
	tr = keyTranslator{}
	feedAll(&tr, p, "\x03") // Ctrl-C
	if got := drainScancodes(p, 4); string(got) != string([]byte{0x1D, 0x2E, 0xAE, 0x9D}) {
		t.Errorf("Ctrl-C = % X, atteso 1D 2E AE 9D", got)
	}
}

func TestTranslatorQuit(t *testing.T) {
	p := device.NewPPI()
	var tr keyTranslator
	tr.feed(p, 0x1D) // Ctrl+]
	if !tr.quit {
		t.Error("Ctrl+] doveva impostare quit")
	}
}
