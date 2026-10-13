//go:build !(darwin || freebsd || netbsd || openbsd || linux)

package cli

import "os"

// IsTerminal reports false where Flashheart cannot tell: setup then only
// shows its changes unless given --yes.
func IsTerminal(*os.File) bool { return false }
