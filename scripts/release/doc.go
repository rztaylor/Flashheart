// Command release is the release tooling shared by
// .github/workflows/release.yml and scripts/package.sh (D86).
//
// It owns reading the newest released section of CHANGELOG.md (`version`
// prints X.Y.Z, `notes` prints the section as release notes, and a missing
// or malformed heading is an error) and packaging (`package`
// cross-compiles cmd/flashheart without cgo for each supported platform,
// stamps internal/buildinfo, and writes
// flashheart_<version>_<os>_<arch>.tar.gz with the binary, LICENSE and
// README.md, plus one SHA256SUMS).
//
// It does not build the frontend (scripts/package.sh does, once), talk to
// GitHub, create tags or publish anything: the workflow's publish job does
// that. Its tests also run scripts/package.sh for the host platform.
package main
