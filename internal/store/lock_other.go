//go:build !unix

package store

import (
	"os"
	"time"
)

// Advisory locks are unix-only; other platforms are undecided (NFR-2), so
// writes there rely on the hash precondition alone.
func flock(*os.File, time.Duration) error { return nil }

func funlock(*os.File) error { return nil }
