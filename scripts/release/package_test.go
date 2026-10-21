package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestDefaultTargetsAreTheSupportedPlatforms(t *testing.T) {
	t.Parallel()

	var got []string
	for _, target := range DefaultTargets {
		got = append(got, target.String())
	}
	want := []string{"darwin/arm64", "darwin/amd64", "linux/amd64", "linux/arm64"}
	if !slices.Equal(got, want) {
		t.Errorf("DefaultTargets = %v, want %v", got, want)
	}
}

func TestParseTargetAcceptsOnlySupportedPlatforms(t *testing.T) {
	t.Parallel()

	for _, ok := range []string{"darwin/arm64", "darwin/amd64", "linux/amd64", "linux/arm64"} {
		if target, err := ParseTarget(ok); err != nil || target.String() != ok {
			t.Errorf("ParseTarget(%q) = %v, %v", ok, target, err)
		}
	}
	for _, bad := range []string{"windows/amd64", "linux", "linux/", "/amd64", "linux/386", "darwin/arm64/x"} {
		if _, err := ParseTarget(bad); err == nil {
			t.Errorf("ParseTarget(%q) accepted it", bad)
		}
	}
}

func TestPackageWritesOneArchivePerTargetAndTheirChecksums(t *testing.T) {
	t.Parallel()

	root := fakeRepo(t)
	out := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	// Archives and sums from an earlier run go; anything else stays.
	for name, content := range map[string]string{
		"flashheart_0.0.9_linux_amd64.tar.gz": "stale",
		"SHA256SUMS":                          "stale",
		"notes.txt":                           "keep",
	} {
		if err := os.WriteFile(filepath.Join(out, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var built []string
	names, err := Package(Options{
		Version: "1.2.3",
		Root:    root,
		Out:     out,
		Targets: []Target{{"linux", "arm64"}, {"darwin", "amd64"}},
		Build: func(target Target, binary string) error {
			built = append(built, target.String())
			return os.WriteFile(binary, []byte("binary for "+target.String()), 0o755)
		},
	})
	if err != nil {
		t.Fatalf("Package: %v", err)
	}
	wantNames := []string{"flashheart_1.2.3_linux_arm64.tar.gz", "flashheart_1.2.3_darwin_amd64.tar.gz"}
	if !slices.Equal(names, wantNames) {
		t.Errorf("names = %v, want %v", names, wantNames)
	}
	if !slices.Equal(built, []string{"linux/arm64", "darwin/amd64"}) {
		t.Errorf("built = %v", built)
	}

	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, entry := range entries {
		files = append(files, entry.Name())
	}
	wantFiles := []string{"SHA256SUMS", "flashheart_1.2.3_darwin_amd64.tar.gz", "flashheart_1.2.3_linux_arm64.tar.gz", "notes.txt"}
	if !slices.Equal(files, wantFiles) {
		t.Errorf("out holds %v, want %v", files, wantFiles)
	}

	for _, name := range wantNames {
		target := strings.TrimSuffix(strings.TrimPrefix(name, "flashheart_1.2.3_"), ".tar.gz")
		contents := readArchive(t, filepath.Join(out, name))
		want := []archiveEntry{
			{"flashheart", 0o755, "binary for " + strings.Replace(target, "_", "/", 1)},
			{"LICENSE", 0o644, "licence text\n"},
			{"README.md", 0o644, "# readme\n"},
		}
		if !slices.Equal(contents, want) {
			t.Errorf("%s holds %+v, want %+v", name, contents, want)
		}
	}
	checkSums(t, out, []string{"flashheart_1.2.3_darwin_amd64.tar.gz", "flashheart_1.2.3_linux_arm64.tar.gz"})
}

func TestPackageRefusesUnsafeVersionsAndBuildFailures(t *testing.T) {
	t.Parallel()

	root := fakeRepo(t)
	build := func(target Target, binary string) error { return os.WriteFile(binary, nil, 0o755) }
	for _, version := range []string{"", "../1.0.0", "1.0.0/x", "1 0", "-1"} {
		_, err := Package(Options{Version: version, Root: root, Out: t.TempDir(), Targets: DefaultTargets, Build: build})
		if err == nil {
			t.Errorf("Package accepted version %q", version)
		}
	}
	for _, stamp := range []Options{{Commit: "abc 123"}, {BuildDate: "Oct 21"}, {Commit: `a"b`}} {
		stamp.Version, stamp.Root, stamp.Out, stamp.Targets, stamp.Build = "1.0.0", root, t.TempDir(), DefaultTargets, build
		if _, err := Package(stamp); err == nil {
			t.Errorf("Package accepted commit %q, build date %q", stamp.Commit, stamp.BuildDate)
		}
	}

	out := t.TempDir()
	failure := errors.New("compiler broke")
	_, err := Package(Options{
		Version: "1.0.0", Root: root, Out: out, Targets: DefaultTargets,
		Build: func(Target, string) error { return failure },
	})
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want the build failure", err)
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Errorf("a failed build left %d files in out", len(entries))
	}
}

// TestPackageScriptBuildsARunnableArchive runs scripts/package.sh for this
// machine's platform, as the release workflow does for all four, and checks
// the binary inside reports the stamped version. The frontend is already
// built: Go tests need it embedded (scripts/check.sh builds it first).
func TestPackageScriptBuildsARunnableArchive(t *testing.T) {
	t.Parallel()

	target, err := ParseTarget(runtime.GOOS + "/" + runtime.GOARCH)
	if err != nil {
		t.Skipf("no release archive for this platform: %v", err)
	}
	root := repoRoot(t)
	out := t.TempDir()
	cmd := exec.Command("sh", filepath.Join(root, "scripts", "package.sh"), "--skip-frontend", "-out", out, target.String())
	cmd.Env = append(os.Environ(),
		"FLASHHEART_VERSION=1.2.3",
		"FLASHHEART_COMMIT=abc1234",
		"FLASHHEART_BUILD_DATE=2026-10-21T12:00:00Z",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("scripts/package.sh: %v\n%s", err, output)
	}

	name := "flashheart_1.2.3_" + target.OS + "_" + target.Arch + ".tar.gz"
	contents := readArchive(t, filepath.Join(out, name))
	var got []string
	for _, entry := range contents {
		got = append(got, entry.name)
	}
	if !slices.Equal(got, []string{"flashheart", "LICENSE", "README.md"}) {
		t.Fatalf("%s holds %v", name, got)
	}
	if contents[1].content != string(readRepoFile(t, "LICENSE")) || contents[2].content != string(readRepoFile(t, "README.md")) {
		t.Error("LICENSE or README.md in the archive differ from the repository's")
	}
	checkSums(t, out, []string{name})

	binary := filepath.Join(t.TempDir(), "flashheart")
	if err := os.WriteFile(binary, []byte(contents[0].content), 0o755); err != nil {
		t.Fatal(err)
	}
	version, err := exec.Command(binary, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("flashheart version: %v\n%s", err, version)
	}
	if want := "flashheart 1.2.3 (commit abc1234, built 2026-10-21T12:00:00Z)"; !strings.Contains(string(version), want) {
		t.Errorf("flashheart version printed %q, want it to contain %q", version, want)
	}
}

type archiveEntry struct {
	name    string
	mode    int64
	content string
}

func readArchive(t *testing.T, path string) []archiveEntry {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	reader := tar.NewReader(gz)
	var entries []archiveEntry
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return entries
		}
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if header.Typeflag != tar.TypeReg || header.Uid != 0 || header.Gid != 0 {
			t.Errorf("%s: %s is type %c owned by %d:%d, want a regular file owned by 0:0", path, header.Name, header.Typeflag, header.Uid, header.Gid)
		}
		var content bytes.Buffer
		if _, err := io.Copy(&content, reader); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, archiveEntry{header.Name, header.Mode, content.String()})
	}
}

// checkSums checks SHA256SUMS lists exactly the named archives, sorted, in
// the `sha256sum`/`shasum -a 256` format, with their real digests.
func checkSums(t *testing.T, dir string, names []string) {
	t.Helper()
	sums, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	var want strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		want.WriteString(hex.EncodeToString(digest[:]) + "  " + name + "\n")
	}
	if string(sums) != want.String() {
		t.Errorf("SHA256SUMS =\n%s\nwant\n%s", sums, want.String())
	}
}

func fakeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range map[string]string{"LICENSE": "licence text\n", "README.md": "# readme\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readRepoFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
