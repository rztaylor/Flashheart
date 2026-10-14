package archtest

import (
	"bufio"
	"bytes"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const module = "github.com/rztaylor/flashheart"

// lean are the packages the hook and mcp modes run (NFR-4) and the
// commands beside them: none may link the web server or the index.
var lean = []string{
	"internal/mcpserver",
	"internal/hooks",
	"internal/hooks/claude",
	"internal/hooks/codex",
	"internal/await",
	"internal/setup",
	"internal/doctor",
}

// heavy are the packages lean ones may not depend on, even indirectly.
var heavy = []string{
	module + "/internal/index",
	module + "/internal/api",
	module + "/internal/app",
	module + "/internal/webui",
	"net/http",
}

// throughModules are third-party modules a lean package may reach
// net/http through, and why. Their packages are not walked; everything
// else, Flashheart's own code included, must not reach a heavy package.
var throughModules = map[string]string{
	"github.com/modelcontextprotocol/go-sdk/": "mcpserver's MCP SDK links net/http for its HTTP transports and OAuth; Flashheart serves MCP over stdio only (D71)",
}

// imports reads the import graph of patterns and their dependencies with
// go list: each package's direct imports.
func imports(t *testing.T, patterns ...string) map[string][]string {
	t.Helper()
	args := append([]string{"list", "-deps", "-f", "{{.ImportPath}} {{join .Imports \" \"}}"}, patterns...)
	command := exec.Command("go", args...)
	command.Dir = "../.."
	var stderr bytes.Buffer
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, stderr.String())
	}
	graph := map[string][]string{}
	lines := bufio.NewScanner(bytes.NewReader(out))
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) > 0 {
			graph[fields[0]] = fields[1:]
		}
	}
	return graph
}

// path finds a chain of imports from root to target in graph without
// walking into a package skip reports true for, or nil.
func path(graph map[string][]string, root, target string, skip func(string) bool) []string {
	from := map[string]string{root: ""}
	queue := []string{root}
	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		if pkg == target {
			chain := []string{}
			for at := pkg; at != ""; at = from[at] {
				chain = append(chain, at)
			}
			slices.Reverse(chain)
			return chain
		}
		if pkg != root && skip(pkg) {
			continue
		}
		for _, next := range graph[pkg] {
			if _, seen := from[next]; !seen {
				from[next] = pkg
				queue = append(queue, next)
			}
		}
	}
	return nil
}

func throughAllowed(pkg string) bool {
	for prefix := range throughModules {
		if strings.HasPrefix(pkg, prefix) {
			return true
		}
	}
	return false
}

func TestLeanPackagesDoNotLinkTheWebServer(t *testing.T) {
	t.Parallel()

	patterns := make([]string, len(lean))
	for i, pkg := range lean {
		patterns[i] = "./" + pkg
	}
	graph := imports(t, patterns...)

	for _, pkg := range lean {
		root := module + "/" + pkg
		if _, ok := graph[root]; !ok {
			t.Errorf("go list did not report %s; is it still a package?", root)
			continue
		}
		for _, target := range heavy {
			if chain := path(graph, root, target, throughAllowed); chain != nil {
				t.Errorf("%s depends on %s (%s); the hook and mcp modes must not link the web server or the index (.agents/facts/architecture.md)",
					pkg, target, strings.Join(chain, " -> "))
			}
		}
	}

	// Each allowance must still be needed.
	for prefix := range throughModules {
		used := false
		for _, pkg := range lean {
			chain := path(graph, module+"/"+pkg, "net/http", func(string) bool { return false })
			if chain != nil && slices.ContainsFunc(chain, func(p string) bool { return strings.HasPrefix(p, prefix) }) {
				used = true
			}
		}
		if !used {
			t.Errorf("no lean package reaches net/http through %s any more; remove it from throughModules", prefix)
		}
	}
}

func TestPathFindsAnImportChain(t *testing.T) {
	t.Parallel()

	graph := map[string][]string{
		"a":       {"b", "sdk/mcp"},
		"b":       {"c"},
		"c":       {"net/http"},
		"sdk/mcp": {"net/http"},
	}
	none := func(string) bool { return false }
	skipSDK := func(p string) bool { return strings.HasPrefix(p, "sdk/") }
	if got := path(graph, "a", "net/http", skipSDK); !slices.Equal(got, []string{"a", "b", "c", "net/http"}) {
		t.Errorf("path = %v, want the chain through b", got)
	}
	graph["b"] = nil
	if got := path(graph, "a", "net/http", skipSDK); got != nil {
		t.Errorf("path through a skipped package = %v, want none", got)
	}
	if got := path(graph, "a", "net/http", none); !slices.Equal(got, []string{"a", "sdk/mcp", "net/http"}) {
		t.Errorf("path without skipping = %v, want the chain through sdk/mcp", got)
	}
}
