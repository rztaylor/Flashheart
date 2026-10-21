package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const usage = `usage:
  release version [-changelog FILE]   print the newest released version (X.Y.Z)
  release notes [-changelog FILE]     print that version's changelog section
  release package -version V [-commit C] [-date D] [-root DIR] [-out DIR] [os/arch ...]
                                      build the release archives and SHA256SUMS
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "version", "notes":
		err = runChangelog(args[0], args[1:], stdout, stderr)
	case "package":
		err = runPackage(args[1:], stdout, stderr)
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "release %s: %v\n", args[0], err)
		return 1
	}
	return 0
}

func runChangelog(command string, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("changelog", "CHANGELOG.md", "the changelog to read")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments %q", flags.Args())
	}
	data, err := os.ReadFile(*path)
	if err != nil {
		return err
	}
	release, err := NewestRelease(string(data))
	if err != nil {
		return fmt.Errorf("%s: %w", *path, err)
	}
	if command == "version" {
		_, err = fmt.Fprintln(stdout, release.Version)
	} else {
		_, err = fmt.Fprint(stdout, release.Notes)
	}
	return err
}

func runPackage(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("package", flag.ContinueOnError)
	flags.SetOutput(stderr)
	version := flags.String("version", "", "the version, X.Y.Z (or dev)")
	commit := flags.String("commit", "unknown", "the commit stamped into the binary")
	date := flags.String("date", "unknown", "the build date stamped into the binary (RFC 3339)")
	root := flags.String("root", ".", "the repository root")
	out := flags.String("out", filepath.Join("build", "dist"), "the directory for the archives")
	if err := flags.Parse(args); err != nil {
		return err
	}
	targets := DefaultTargets
	if flags.NArg() > 0 {
		targets = nil
		for _, arg := range flags.Args() {
			target, err := ParseTarget(arg)
			if err != nil {
				return err
			}
			targets = append(targets, target)
		}
	}
	names, err := Package(Options{Version: *version, Commit: *commit, BuildDate: *date, Root: *root, Out: *out, Targets: targets})
	if err != nil {
		return err
	}
	for _, name := range append(names, SumsName) {
		fmt.Fprintln(stdout, filepath.Join(*out, name))
	}
	return nil
}
