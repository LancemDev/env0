//go:build linux

package main

import (
	"io"
	"os"
	"syscall"
	"unsafe"
)

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(
		syscall.SYS_IOCTL,
		f.Fd(),
		syscall.TCGETS,
		uintptr(unsafe.Pointer(&termios)),
		0, 0, 0,
	)
	return err == 0
}
