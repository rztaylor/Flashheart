//go:build unix

package background

import (
	"os"
	"syscall"
)

// detachedAttributes starts the child in a new session so terminal signals
// and closing the terminal do not reach it.
func detachedAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

func closeOnExec(file *os.File) {
	syscall.CloseOnExec(int(file.Fd()))
}
