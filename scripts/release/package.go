package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Target is one platform a release archive is built for.
type Target struct{ OS, Arch string }

func (t Target) String() string { return t.OS + "/" + t.Arch }

// DefaultTargets are the supported platforms (NFR-2), in release order.
var DefaultTargets = []Target{{"darwin", "arm64"}, {"darwin", "amd64"}, {"linux", "amd64"}, {"linux", "arm64"}}

// ParseTarget reads "os/arch", accepting only a supported platform.
func ParseTarget(s string) (Target, error) {
	for _, target := range DefaultTargets {
		if target.String() == s {
			return target, nil
		}
	}
	names := make([]string, len(DefaultTargets))
	for i, target := range DefaultTargets {
		names[i] = target.String()
	}
	return Target{}, fmt.Errorf("unsupported target %q (supported: %s)", s, strings.Join(names, ", "))
}

// ArchiveName is the release asset name for a version and target.
func ArchiveName(version string, target Target) string {
	return fmt.Sprintf("flashheart_%s_%s_%s.tar.gz", version, target.OS, target.Arch)
}

// SumsName is the checksums file covering a run's archives.
const SumsName = "SHA256SUMS"

// Options configure Package.
type Options struct {
	Version, Commit, BuildDate string
	Root                       string // the repository: LICENSE, README.md, cmd/flashheart
	Out                        string // where the archives and SHA256SUMS go
	Targets                    []Target
	// Build writes the flashheart binary for target to binary. Nil means
	// cross-compiling Root's cmd/flashheart with the go command.
	Build func(target Target, binary string) error
}

var safeVersion = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]*$`)

// Package builds flashheart for each target and writes
// flashheart_<version>_<os>_<arch>.tar.gz (the binary, LICENSE and
// README.md at the archive's top level) into Out, then SHA256SUMS over
// them. Archives and sums left in Out by an earlier run are removed first;
// other files are kept. It returns the archive names in target order.
func Package(o Options) ([]string, error) {
	if !safeVersion.MatchString(o.Version) {
		return nil, fmt.Errorf("version %q is not usable in a file name", o.Version)
	}
	// The linker splits -ldflags on spaces and quotes.
	for label, value := range map[string]string{"commit": o.Commit, "build date": o.BuildDate} {
		if strings.ContainsAny(value, " \t\r\n'\"") {
			return nil, fmt.Errorf("%s %q holds a space or quote", label, value)
		}
	}
	if len(o.Targets) == 0 {
		return nil, fmt.Errorf("no targets")
	}
	build := o.Build
	if build == nil {
		build = goBuild(o)
	}
	if err := os.MkdirAll(o.Out, 0o755); err != nil {
		return nil, err
	}
	if err := removeEarlierRun(o.Out); err != nil {
		return nil, err
	}
	staging, err := os.MkdirTemp("", "flashheart-package-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)

	// Everything is written to staging first, so a failed build leaves Out
	// without a partial set.
	var names []string
	sums := map[string]string{}
	for _, target := range o.Targets {
		binary := filepath.Join(staging, target.OS+"_"+target.Arch, "flashheart")
		if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
			return nil, err
		}
		if err := build(target, binary); err != nil {
			return nil, fmt.Errorf("build %s: %w", target, err)
		}
		name := ArchiveName(o.Version, target)
		digest, err := writeArchive(filepath.Join(staging, name), modTime(o.BuildDate), []archiveFile{
			{"flashheart", binary, 0o755},
			{"LICENSE", filepath.Join(o.Root, "LICENSE"), 0o644},
			{"README.md", filepath.Join(o.Root, "README.md"), 0o644},
		})
		if err != nil {
			return nil, fmt.Errorf("archive %s: %w", target, err)
		}
		names = append(names, name)
		sums[name] = digest
	}

	sorted := slices.Sorted(maps.Keys(sums))
	var lines strings.Builder
	for _, name := range sorted {
		fmt.Fprintf(&lines, "%s  %s\n", sums[name], name)
	}
	if err := os.WriteFile(filepath.Join(staging, SumsName), []byte(lines.String()), 0o644); err != nil {
		return nil, err
	}
	for _, name := range append(sorted, SumsName) {
		if err := moveFile(filepath.Join(staging, name), filepath.Join(o.Out, name)); err != nil {
			return nil, err
		}
	}
	return names, nil
}

// goBuild cross-compiles cmd/flashheart without cgo, stamping the build
// metadata as scripts/build.sh does.
func goBuild(o Options) func(Target, string) error {
	const buildinfo = "github.com/rztaylor/flashheart/internal/buildinfo"
	ldflags := fmt.Sprintf("-s -w -X %s.Version=%s -X %s.Commit=%s -X %s.BuildDate=%s",
		buildinfo, o.Version, buildinfo, o.Commit, buildinfo, o.BuildDate)
	return func(target Target, binary string) error {
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", binary, "./cmd/flashheart")
		cmd.Dir = o.Root
		cmd.Env = append(os.Environ(), "GOOS="+target.OS, "GOARCH="+target.Arch, "CGO_ENABLED=0")
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
}

func removeEarlierRun(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == SumsName || (strings.HasPrefix(name, "flashheart_") && strings.HasSuffix(name, ".tar.gz")) {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

// modTime is the archive entries' time: the build date when it is one,
// otherwise now.
func modTime(buildDate string) time.Time {
	if t, err := time.Parse(time.RFC3339, buildDate); err == nil {
		return t
	}
	return time.Now().UTC().Truncate(time.Second)
}

type archiveFile struct {
	name, source string
	mode         int64
}

// writeArchive writes a gzipped tar of files, owned by 0:0 with no user or
// group names, and returns its SHA-256 in hex.
func writeArchive(path string, mtime time.Time, files []archiveFile) (string, error) {
	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()
	hash := sha256.New()
	gz := gzip.NewWriter(io.MultiWriter(out, hash))
	tw := tar.NewWriter(gz)
	for _, file := range files {
		data, err := os.ReadFile(file.source)
		if err != nil {
			return "", err
		}
		header := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     file.name,
			Mode:     file.mode,
			Size:     int64(len(data)),
			ModTime:  mtime,
			Format:   tar.FormatUSTAR,
		}
		if err := tw.WriteHeader(header); err != nil {
			return "", err
		}
		if _, err := tw.Write(data); err != nil {
			return "", err
		}
	}
	if err := tw.Close(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// moveFile renames, falling back to a copy across file systems (the
// staging directory is in the system temporary directory).
func moveFile(from, to string) error {
	if err := os.Rename(from, to); err == nil {
		return nil
	}
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, data, 0o644)
}
