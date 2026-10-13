//go:build darwin || freebsd || netbsd || openbsd

package cli

import "syscall"

const getTermios = syscall.TIOCGETA
