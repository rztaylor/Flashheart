package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// fileAccess lists, per import path, the functions that open, create,
// list, stat, change or remove files. Calling one is touching the
// filesystem; process plumbing (os.Pipe, os.Getwd, os.Executable) is not.
var fileAccess = map[string][]string{
	"os": {
		"Chmod", "Chown", "Chtimes", "CopyFS", "Create", "CreateTemp", "DirFS",
		"Lchown", "Link", "Lstat", "Mkdir", "MkdirAll", "MkdirTemp", "Open",
		"OpenFile", "OpenInRoot", "OpenRoot", "ReadDir", "ReadFile", "Readlink",
		"Remove", "RemoveAll", "Rename", "Stat", "Symlink", "Truncate", "WriteFile",
	},
	"io/ioutil":     {"ReadDir", "ReadFile", "TempDir", "TempFile", "WriteFile"},
	"path/filepath": {"Glob", "Walk", "WalkDir"},
}

// allowed are the only packages (module-relative directories) that may call
// a fileAccess function, and why. A package that is not here reaches the
// filesystem through store, which confines every board-root path to an
// os.Root (SEC-2). Adding a package needs a reason a reviewer can check:
// either its paths are never under the board root, or it is an accepted
// exception named in .agents/facts/architecture.md.
var allowed = map[string]string{
	"internal/store": "owns the board root; every root path goes through an os.Root (attach sources outside the root are stat'ed and opened by path, SEC-2)",

	// Accepted exceptions under the root.
	"internal/logfile": "exception: appends the diagnostic logs <root>/.flashheart/serve.log and hook-errors.log (and reads them for doctor), which must work when the store does not",
	"internal/index":   "exception: stats and lists the root's directories to add fsnotify watches; never opens or writes a file",
	"internal/doctor":  "exception: stats the root folder itself and, without access(2), proves it writable with a probe file it removes",

	// Never under the root.
	"internal/setup":     "agents' configuration under the home directory (~/.claude, backups), never the board root",
	"internal/gitinfo":   "reads .git metadata in the caller's repositories, never the board root",
	"internal/gitchange": "stats files in an agent's worktree, never the board root",
	"scripts/hookbench":  "developer benchmark, not in the binary: seeds and reads a temporary root",
	"scripts/release":    "release tooling, not in the binary: reads the repository's CHANGELOG.md, LICENSE and README.md and writes archives to build/dist (D86)",
}

func TestOnlyAllowedPackagesTouchFiles(t *testing.T) {
	t.Parallel()

	module := filepath.Join("..", "..")
	found := map[string][]string{}
	for _, top := range []string{"cmd", "internal", "scripts"} {
		err := goFiles(filepath.Join(module, top), func(name string, file *ast.File) {
			relative, _ := filepath.Rel(module, filepath.Dir(name))
			pkg := filepath.ToSlash(relative)
			for _, call := range fileCalls(file) {
				found[pkg] = append(found[pkg], filepath.Base(name)+": "+call)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(found) == 0 {
		t.Fatal("found no file access at all; is the module root right?")
	}

	for pkg, calls := range found {
		if _, ok := allowed[pkg]; !ok {
			slices.Sort(calls)
			t.Errorf("%s touches files directly (%s); go through store, or add it to the allowlist in %s with the reason its paths are not under the board root",
				pkg, strings.Join(slices.Compact(calls), ", "), "internal/archtest/confinement_test.go")
		}
	}
	for pkg := range allowed {
		if _, ok := found[pkg]; !ok {
			t.Errorf("%s is allowlisted but no longer touches files; remove it from the allowlist", pkg)
		}
	}
}

// The default board root is spelled once, as cli.DefaultRoot (FH-108).
func TestDefaultRootIsDeclaredOnce(t *testing.T) {
	t.Parallel()

	var spelled []string
	err := goFiles(filepath.Join("..", "..", "internal"), func(name string, file *ast.File) {
		ast.Inspect(file, func(node ast.Node) bool {
			if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING && strings.Contains(literal.Value, "Kanban") {
				spelled = append(spelled, filepath.ToSlash(name)+": "+literal.Value)
			}
			return true
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(spelled) != 1 || !strings.HasSuffix(spelled[0], "cli/cli.go: \"~/reports/Kanban\"") {
		t.Errorf("the default root is spelled %v; want it once, as cli.DefaultRoot", spelled)
	}
}

// goFiles parses every non-test Go file under dir.
func goFiles(dir string, each func(name string, file *ast.File)) error {
	return filepath.WalkDir(dir, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if base := entry.Name(); base == "testdata" || base == "node_modules" || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		each(name, file)
		return nil
	})
}

// fileCalls returns the fileAccess functions a Go file calls, as
// "os.Open", resolving each import's local name.
func fileCalls(file *ast.File) []string {
	local := map[string]string{} // local name -> import path
	for _, spec := range file.Imports {
		importPath, _ := strconv.Unquote(spec.Path.Value)
		if _, watched := fileAccess[importPath]; !watched {
			continue
		}
		alias := importPath[strings.LastIndex(importPath, "/")+1:]
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		local[alias] = importPath
	}
	if len(local) == 0 {
		return nil
	}
	var calls []string
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		if importPath, ok := local[ident.Name]; ok && slices.Contains(fileAccess[importPath], selector.Sel.Name) {
			calls = append(calls, ident.Name+"."+selector.Sel.Name)
		}
		return true
	})
	return calls
}
