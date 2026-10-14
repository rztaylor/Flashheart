package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/logfile"
)

const placeholder = "/Users/example/src/demo"

// Scenarios.
const (
	scenarioMix          = "mix"
	scenarioTools        = "tools"
	scenarioSessionStart = "session-start"
	scenarioStop         = "stop"
)

type options struct {
	binary, agent, scenario string
	n, seedEvents           int
	limit                   time.Duration
}

func main() {
	var o options
	flag.StringVar(&o.binary, "binary", "build/flashheart", "flashheart binary to measure")
	flag.StringVar(&o.agent, "agent", "claude", "agent whose recorded payloads (testdata/hooks/<agent>) are replayed")
	flag.StringVar(&o.scenario, "scenario", scenarioMix, "what to measure: mix (a session's hooks), tools (tool results alone, the throttled path), session-start (the recovery note) or stop (a turn end that handoff enforcement blocks)")
	tools := flag.Bool("tools", false, "same as -scenario tools")
	flag.IntVar(&o.n, "n", 1000, "measured invocations")
	flag.IntVar(&o.seedEvents, "seed-events", 0, "events a day to seed the project's log with across the run window before measuring")
	flag.DurationVar(&o.limit, "p95", 50*time.Millisecond, "fail when p95 exceeds this")
	flag.Parse()
	if *tools {
		o.scenario = scenarioTools
	}
	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "hookbench:", err)
		os.Exit(1)
	}
}

// step is one hook invocation: its event, its payload, and whether it is
// timed and must print something.
type step struct {
	event   string
	payload []byte
	timed   bool
	output  bool
}

func run(o options) error {
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
	payloads := payloadSource{dir: filepath.Join("testdata", "hooks", o.agent), repo: repo}

	steps, session, err := scenario(o.scenario, o.agent, payloads)
	if err != nil {
		return err
	}
	// A home of its own, so a session start never refreshes the developer's
	// installed skill.
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	b := bench{binary: o.binary, agent: o.agent, root: root, env: append(os.Environ(), "HOME="+home)}
	// Set up the board: the project, its tickets, the session the stop
	// scenario enforces a handoff on, and the seeded log.
	now := time.Now().UTC()
	seeded, err := setUp(root, repo, o, session, now)
	if err != nil {
		return err
	}
	// Warm up: the cwd cache and the page cache.
	for i := range 20 {
		if _, err := b.invoke(steps(i)); err != nil {
			return err
		}
	}
	durations := make([]time.Duration, 0, o.n)
	invocations := 20
	for i := 0; len(durations) < o.n; i++ {
		for _, s := range steps(20 + i) {
			d, err := b.invoke1(s)
			if err != nil {
				return err
			}
			invocations++
			if s.timed {
				durations = append(durations, d)
			}
		}
	}
	if data, _ := os.ReadFile(logfile.HookErrorsPath(root)); len(data) > 0 {
		return fmt.Errorf("hook errors:\n%s", data)
	}
	slices.Sort(durations)
	pct := func(p float64) time.Duration { return durations[min(len(durations)-1, int(p*float64(len(durations))))] }
	fmt.Printf("%s %s hook latency over %d warm invocations (%d seeded events): p50 %v, p95 %v, p99 %v, max %v\n",
		o.agent, o.scenario, o.n, seeded, pct(0.50).Round(10*time.Microsecond), pct(0.95).Round(10*time.Microsecond), pct(0.99).Round(10*time.Microsecond), durations[len(durations)-1].Round(10*time.Microsecond))
	recorded := 0
	logs, _ := filepath.Glob(filepath.Join(root, "*", ".flashheart", "events", "*.jsonl"))
	for _, file := range logs {
		data, _ := os.ReadFile(file)
		recorded += bytes.Count(data, []byte("\n"))
	}
	fmt.Printf("%d events recorded for %d invocations\n", recorded-seeded, invocations)
	if pct(0.95) > o.limit {
		return fmt.Errorf("p95 %v exceeds %v", pct(0.95), o.limit)
	}
	return nil
}

// mixes are each agent's recorded payloads for the mix and tools
// scenarios. mix is a realistic session: mostly tool results, with
// prompts and stops. tools is tool results within a minute of each
// other: all but the first and each newly edited path's are held in the
// session's state.
var mixes = map[string]map[string][]string{
	"claude": {
		scenarioMix:   {"PostToolUse/edit", "PostToolUse/read", "PostToolUse/bash", "PostToolUse/todowrite", "PostToolUse/edit", "UserPromptSubmit/prompt", "PostToolUse/write", "Stop/stop"},
		scenarioTools: {"PostToolUse/read", "PostToolUse/bash", "PostToolUse/read", "PostToolUseFailure/read-missing", "PostToolUse/edit"},
	},
	// No recorded Codex session had a planning tool (MANIFEST.md).
	"codex": {
		scenarioMix:   {"PostToolUse/apply-patch", "PostToolUse/bash", "PostToolUse/wait-agent", "PostToolUse/bash-failed", "PostToolUse/app-apply-patch", "UserPromptSubmit/prompt", "PostToolUse/spawn-agent", "Stop/stop"},
		scenarioTools: {"PostToolUse/bash", "PostToolUse/wait-agent", "PostToolUse/bash-failed", "PostToolUse/spawn-agent", "PostToolUse/apply-patch"},
	},
}

// scenario returns the invocations of round i and the session the
// scenario's stops are enforced on ("" for none).
func scenario(name, agent string, payloads payloadSource) (func(i int) []step, string, error) {
	switch name {
	case scenarioMix, scenarioTools:
		mix := mixes[agent][name]
		if mix == nil {
			return nil, "", fmt.Errorf("no %s mix for agent %q", name, agent)
		}
		steps := make([]step, len(mix))
		for i, name := range mix {
			data, err := payloads.read(name)
			if err != nil {
				return nil, "", err
			}
			steps[i] = step{event: strings.Split(name, "/")[0], payload: data, timed: true}
		}
		return func(i int) []step { return []step{steps[i%len(steps)]} }, "", nil
	case scenarioSessionStart:
		// A new session each time, through every recorded start source.
		starts, err := payloads.all("SessionStart")
		if err != nil {
			return nil, "", err
		}
		return func(i int) []step {
			data, err := withSession(starts[i%len(starts)], fmt.Sprintf("bench-start-%d", i))
			if err != nil {
				panic(err)
			}
			return []step{{event: "SessionStart", payload: data, timed: true}}
		}, "", nil
	case scenarioStop:
		// One session's turns: a prompt (untimed) opens the turn, so every
		// stop is the first of its turn and enforcement blocks it.
		stop, err := payloads.read("Stop/stop")
		if err != nil {
			return nil, "", err
		}
		prompt, err := payloads.read("UserPromptSubmit/prompt")
		if err != nil {
			return nil, "", err
		}
		session, err := sessionOf(stop)
		if err != nil {
			return nil, "", err
		}
		if prompt, err = withSession(prompt, session); err != nil {
			return nil, "", err
		}
		steps := []step{{event: "UserPromptSubmit", payload: prompt}, {event: "Stop", payload: stop, timed: true, output: true}}
		return func(int) []step { return steps }, session, nil
	}
	return nil, "", fmt.Errorf("unknown scenario %q (mix, tools, session-start or stop)", name)
}

// payloadSource reads an agent's recorded payloads, pointed at the bench's
// repository.
type payloadSource struct{ dir, repo string }

func (p payloadSource) read(name string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(p.dir, name+".json"))
	if err != nil {
		return nil, err
	}
	return []byte(strings.ReplaceAll(string(data), placeholder, p.repo)), nil
}

// all reads every recorded payload of an event, in name order.
func (p payloadSource) all(event string) ([][]byte, error) {
	files, err := filepath.Glob(filepath.Join(p.dir, event, "*.json"))
	if err != nil {
		return nil, err
	}
	var list [][]byte
	for _, file := range files {
		if strings.HasSuffix(file, ".events.json") {
			continue
		}
		data, err := p.read(strings.TrimSuffix(strings.TrimPrefix(file, p.dir+string(filepath.Separator)), ".json"))
		if err != nil {
			return nil, err
		}
		list = append(list, data)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("no recorded %s payloads in %s", event, p.dir)
	}
	return list, nil
}

// sessionOf and withSession read and replace a payload's session_id.
func sessionOf(payload []byte) (string, error) {
	var fields map[string]any
	if err := json.Unmarshal(payload, &fields); err != nil {
		return "", err
	}
	session, _ := fields["session_id"].(string)
	if session == "" {
		return "", errors.New("payload has no session_id")
	}
	return session, nil
}

func withSession(payload []byte, session string) ([]byte, error) {
	var fields map[string]any
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, err
	}
	if _, ok := fields["session_id"]; !ok {
		return nil, errors.New("payload has no session_id")
	}
	fields["session_id"] = session
	return json.Marshal(fields)
}

type bench struct {
	binary, agent, root string
	env                 []string
}

func (b bench) invoke(steps []step) (time.Duration, error) {
	var total time.Duration
	for _, s := range steps {
		d, err := b.invoke1(s)
		if err != nil {
			return 0, err
		}
		total += d
	}
	return total, nil
}

func (b bench) invoke1(s step) (time.Duration, error) {
	cmd := exec.Command(b.binary, "hook", b.agent, s.event, "--root", b.root)
	cmd.Stdin, cmd.Env = bytes.NewReader(s.payload), b.env
	start := time.Now()
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)
	switch {
	case err != nil:
		return 0, fmt.Errorf("%s: %v %s", s.event, err, out)
	case s.output && len(out) == 0:
		return 0, fmt.Errorf("%s printed nothing: enforcement did not block", s.event)
	case !s.output && s.event != "SessionStart" && len(out) > 0:
		return 0, fmt.Errorf("%s printed %s", s.event, out)
	}
	return elapsed, nil
}
