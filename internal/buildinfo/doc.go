// Package buildinfo exposes immutable executable build metadata.
//
// It owns the version, commit and build date shared by the CLI and the info
// API. Release builds set its variables with linker flags; buildinfo performs
// no Git, filesystem or network discovery at runtime.
package buildinfo
