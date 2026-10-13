//go:build darwin || freebsd || netbsd || openbsd || linux

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

// IsTerminal reports whether f is a terminal, as isatty(3) does: whether
// it has terminal attributes. A pipe or /dev/null has none, so setup does
// not ask a question nobody can answer (FH-129).
func IsTerminal(f *os.File) bool {
	var attributes syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(getTermios), uintptr(unsafe.Pointer(&attributes)))
	return errno == 0
}
