package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const placeholder = "/Users/example/src/demo"

func main() {
	binary := flag.String("binary", "build/flashheart", "flashheart binary to measure")
	n := flag.Int("n", 1000, "measured invocations")
	limit := flag.Duration("p95", 50*time.Millisecond, "fail when p95 exceeds this")
	flag.Parse()
	if err := run(*binary, *n, *limit); err != nil {
		fmt.Fprintln(os.Stderr, "hookbench:", err)
		os.Exit(1)
	}
}

func run(binary string, n int, limit time.Duration) error {
	work, err := os.MkdirTemp("", "flashheart-hookbench-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	work, _ = filepath.EvalSymlinks(work)
	root := filepath.Join(work, "root")
	repo := filepath.Join(work, "src", "demo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/feature/demo\n"), 0o644); err != nil {
		return err
	}

	// A realistic mix: mostly tool results, with prompts, plans and stops.
	mix := []string{"PostToolUse/edit", "PostToolUse/read", "PostToolUse/bash", "PostToolUse/todowrite", "PostToolUse/edit", "UserPromptSubmit/prompt", "PostToolUse/write", "Stop/stop"}
	payloads := make([][]byte, len(mix))
	events := make([]string, len(mix))
	for i, name := range mix {
		data, err := os.ReadFile(filepath.Join("testdata", "hooks", "claude", name+".json"))
		if err != nil {
			return err
		}
		payloads[i] = []byte(strings.ReplaceAll(string(data), placeholder, repo))
		events[i] = strings.Split(name, "/")[0]
	}
	invoke := func(i int) (time.Duration, error) {
		cmd := exec.Command(binary, "hook", "claude", events[i%len(mix)], "--root", root)
		cmd.Stdin = bytes.NewReader(payloads[i%len(mix)])
		start := time.Now()
		out, err := cmd.CombinedOutput()
		elapsed := time.Since(start)
		if err != nil || len(out) > 0 {
			return 0, fmt.Errorf("invocation %d: %v %s", i, err, out)
		}
		return elapsed, nil
	}
	// Warm up: create the project and the cwd cache.
	for i := range 20 {
		if _, err := invoke(i); err != nil {
			return err
		}
	}
	durations := make([]time.Duration, 0, n)
	for i := range n {
		d, err := invoke(i)
		if err != nil {
			return err
		}
		durations = append(durations, d)
	}
	if data, _ := os.ReadFile(filepath.Join(root, ".flashheart", "hook-errors.log")); len(data) > 0 {
		return fmt.Errorf("hook errors:\n%s", data)
	}
	slices.Sort(durations)
	pct := func(p float64) time.Duration { return durations[min(len(durations)-1, int(p*float64(len(durations))))] }
	fmt.Printf("hook latency over %d warm invocations: p50 %v, p95 %v, p99 %v, max %v\n",
		n, pct(0.50).Round(10*time.Microsecond), pct(0.95).Round(10*time.Microsecond), pct(0.99).Round(10*time.Microsecond), durations[len(durations)-1].Round(10*time.Microsecond))
	if pct(0.95) > limit {
		return fmt.Errorf("p95 %v exceeds %v", pct(0.95), limit)
	}
	return nil
}
