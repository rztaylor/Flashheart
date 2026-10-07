//go:build !unix

package doctor

import "os"

// writable proves this user may create files in dir by creating and
// removing a probe file: there is no access(2) to ask.
func writable(dir string) error {
	probe, err := os.CreateTemp(dir, ".flashheart-doctor-*")
	if err != nil {
		return err
	}
	probe.Close()
	return os.Remove(probe.Name())
}
