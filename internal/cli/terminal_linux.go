//go:build linux

package cli

import "syscall"

const getTermios = syscall.TCGETS
