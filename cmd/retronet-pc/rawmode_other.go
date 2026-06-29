//go:build !darwin && !linux

package main

import (
	"errors"
	"os"
)

// enterRawMode non e' supportata fuori da darwin/linux: la modalita' interattiva
// non e' disponibile (resta utilizzabile -live e l'esecuzione non interattiva).
func enterRawMode(*os.File) (func() error, error) {
	return nil, errors.New("raw mode non supportata su questo sistema")
}
