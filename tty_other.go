//go:build !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package main

import (
	"io"
	"os"
)

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return (st.Mode() & os.ModeCharDevice) != 0
}
