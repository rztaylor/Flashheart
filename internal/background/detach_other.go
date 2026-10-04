//go:build !unix

package background

import (
	"os"
	"syscall"
)

// detachedAttributes is a no-op where sessions are unavailable; the child
// still runs without the launcher's standard streams.
func detachedAttributes() *syscall.SysProcAttr { return nil }

func closeOnExec(*os.File) {}
