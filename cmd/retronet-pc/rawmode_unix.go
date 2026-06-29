//go:build darwin || linux

package main

import (
	"os"
	"syscall"
	"unsafe"
)

// enterRawMode mette il terminale in raw mode (niente eco, niente bufferizzazione
// di riga, niente traduzione segnali) e restituisce una funzione per ripristinare
// lo stato originale. Replica l'approccio a termios di retronet-terminal,
// aggiungendo il supporto darwin (ioctl TIOCGETA/TIOCSETA, vedi rawmode_darwin.go).
func enterRawMode(f *os.File) (func() error, error) {
	fd := f.Fd()
	var original syscall.Termios
	if err := ioctlTermios(fd, ioctlGetTermios, &original); err != nil {
		return nil, err
	}
	raw := original
	raw.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP |
		syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	raw.Oflag &^= syscall.OPOST
	raw.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	raw.Cflag &^= syscall.CSIZE | syscall.PARENB
	raw.Cflag |= syscall.CS8
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := ioctlTermios(fd, ioctlSetTermios, &raw); err != nil {
		return nil, err
	}
	return func() error { return ioctlTermios(fd, ioctlSetTermios, &original) }, nil
}

func ioctlTermios(fd, request uintptr, t *syscall.Termios) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(unsafe.Pointer(t)))
	if errno != 0 {
		return errno
	}
	return nil
}
