//go:build unix

package doctor

import "syscall"

// writable reports whether this user may create files in dir, without
// creating one (access(2) with W_OK).
func writable(dir string) error {
	return syscall.Access(dir, 0x2)
}
