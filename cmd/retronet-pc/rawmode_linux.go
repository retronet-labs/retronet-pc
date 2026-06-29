//go:build linux

package main

import "syscall"

// Su linux i termios si leggono/scrivono con gli ioctl TCGETS/TCSETS.
const (
	ioctlGetTermios = syscall.TCGETS
	ioctlSetTermios = syscall.TCSETS
)
