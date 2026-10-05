//go:build unix

package store

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// flock takes an exclusive advisory lock on f, polling until wait elapses.
// Locks belong to the open file, so separate opens exclude each other even
// within one process.
func flock(f *os.File, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EINTR) {
			return err
		}
		if time.Now().After(deadline) {
			return ErrBusy
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func funlock(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
