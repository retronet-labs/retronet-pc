//go:build darwin

package main

import "syscall"

// Su darwin i termios si leggono/scrivono con gli ioctl TIOCGETA/TIOCSETA.
const (
	ioctlGetTermios = syscall.TIOCGETA
	ioctlSetTermios = syscall.TIOCSETA
)
