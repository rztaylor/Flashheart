package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/hooks"
)

// Agent is Codex's agent id (agent-protocol §2).
const Agent = "codex"

// Adapter maps Codex hook payloads.
type Adapter struct{}

// payload holds the fields Flashheart reads. Prompts, replies, tool
// responses and transcripts are never decoded: every prompt Codex sends
// UserPromptSubmit for is the user's (a stop hook's reason continues the
// turn without one; MANIFEST.md), so none is read.
type payload struct {
	SessionID      string          `json:"session_id"`
	Cwd            string          `json:"cwd"`
	Source         string          `json:"source"`
	Reason         string          `json:"reason"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	AgentID        string          `json:"agent_id"`
	AgentType      string          `json:"agent_type"`
	StopHookActive bool            `json:"stop_hook_active"`
}

// checkpointTool is Flashheart's checkpoint tool in Codex, which names MCP
// tools mcp__<server>__<tool> as Claude Code does.
const checkpointTool = "mcp__flashheart__checkpoint"

// Agent returns "codex".
func (Adapter) Agent() string { return Agent }

// Parse maps one hook event's payload to events.
func (Adapter) Parse(event string, data []byte) (hooks.Input, error) {
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return hooks.Input{}, fmt.Errorf("payload is not JSON: %w", err)
	}
	sessionRun, err := events.SessionRun(Agent, p.SessionID)
	if err != nil {
		return hooks.Input{}, errors.New("payload has no usable session_id")
	}
	session := sessionRun.String()
	// A subagent's events carry the parent's session_id and its own agent_id.
	actor := session
	if p.AgentID != "" {
		subagent, err := sessionRun.Subagent(p.AgentID)
		if err != nil {
			return hooks.Input{}, errors.New("payload has an unusable agent_id")
		}
		actor = subagent.String()
	}
	input := hooks.Input{Cwd: p.Cwd}
	add := func(run string, kind events.Kind, data any) {
		input.Events = append(input.Events, hooks.Pending{Run: run, Kind: kind, Data: data})
	}

	switch event {
	case "SessionStart":
		add(session, events.RunStart, events.RunStartData{Kind: events.KindSession, Source: p.Source})
		input.Recovery = true
	case "UserPromptSubmit":
		add(session, events.TurnStart, events.TurnStartData{Origin: events.OriginUser})
		// Codex is known to show the model a prompt's additionalContext,
		// not a tool result's (MANIFEST.md), so a due claim reminder waits
		// for the prompt (D50).
		input.Inbox, input.Notes = session, true
	case "PostToolUse":
		// Codex has no failure event: a failed command is an ordinary result.
		add(actor, events.ToolUsed, toolResult(p.ToolName, p.ToolInput, p.Cwd))
		input.Settled = p.ToolName == checkpointTool
		if plan, found := planUpdate(p.ToolName, p.ToolInput); found {
			add(actor, events.PlanUpdated, plan)
		}
	case "PermissionRequest":
		add(actor, events.PermissionRequested, events.PermissionData{Tool: p.ToolName})
	case "SubagentStart":
		if p.AgentID != "" {
			add(actor, events.RunStart, events.RunStartData{Kind: events.KindSubagent, Parent: session, AgentType: p.AgentType})
		}
	case "SubagentStop":
		if p.AgentID != "" {
			add(actor, events.RunEnd, events.RunEndData{Reason: "completed"})
		}
	case "PreCompact":
		add(session, events.Compact, events.CompactData{Phase: "pre"})
	case "PostCompact":
		add(session, events.Compact, events.CompactData{Phase: "post"})
	case "Stop":
		add(session, events.TurnEnd, events.TurnEndData{})
		input.Stop, input.StopActive = true, p.StopHookActive
	case "Interrupt":
		// Esc or a denied permission prompt ends the turn instead of Stop;
		// no output can keep it going, so it is never blocked.
		add(session, events.TurnEnd, events.TurnEndData{})
	case "SessionEnd":
		add(session, events.RunEnd, events.RunEndData{Reason: p.Reason})
	}
	return input, nil
}

// shellTool is Codex's shell command tool, whose edits name no path.
const shellTool = "Bash"

// toolResult classifies a tool result for hooks: a shell command, or an
// apply_patch and every file it changed.
func toolResult(tool string, input json.RawMessage, cwd string) events.ToolData {
	data := events.ToolData{Tool: tool, OK: true, Shell: tool == shellTool}
	if tool == "apply_patch" {
		data.Paths = patchPaths(input, cwd)
	}
	return data
}

// patchHeaders start the lines of an apply_patch that name a file; content
// lines start with +, - or a space, so they never match.
var patchHeaders = []string{"*** Add File: ", "*** Update File: ", "*** Delete File: ", "*** Move to: "}

// patchPaths returns the files an apply_patch names, each once, absolute:
// Codex sends absolute paths, and a relative one is from the session's
// directory.
func patchPaths(input json.RawMessage, cwd string) []string {
	var fields struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(input, &fields) != nil {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(fields.Command, "\n") {
		for _, header := range patchHeaders {
			path, found := strings.CutPrefix(line, header)
			if path = strings.TrimSpace(path); !found || path == "" {
				continue
			}
			if !filepath.IsAbs(path) && cwd != "" {
				path = filepath.Join(cwd, path)
			}
			if !slices.Contains(paths, path) {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func planStatus(value string) string {
	switch value {
	case events.PlanInProgress, events.PlanCompleted:
		return value
	}
	return events.PlanPending
}

// planUpdate reads the whole plan from update_plan (RUN-4). No recorded
// Codex surface offered the tool (MANIFEST.md); its input is Codex's tool
// definition: plan, a list of steps with a status.
func planUpdate(tool string, input json.RawMessage) (events.PlanData, bool) {
	if tool != "update_plan" {
		return events.PlanData{}, false
	}
	var fields struct {
		Plan []struct {
			Step   string `json:"step"`
			Status string `json:"status"`
		} `json:"plan"`
	}
	if json.Unmarshal(input, &fields) != nil || fields.Plan == nil {
		return events.PlanData{}, false
	}
	plan := events.PlanData{Items: []events.PlanItem{}}
	for _, item := range fields.Plan {
		plan.Items = append(plan.Items, events.PlanItem{Text: item.Step, Status: planStatus(item.Status)})
	}
	return plan, true
}

// Render returns Codex's hook output: a stop decision, additional context
// as hookSpecificOutput (a developer message), or nothing.
func (Adapter) Render(event string, out hooks.Output) []byte {
	var value any
	switch {
	case out.Block != "":
		value = map[string]string{"decision": "block", "reason": out.Block}
	case out.Context != "":
		value = map[string]any{"hookSpecificOutput": map[string]string{
			"hookEventName":     event,
			"additionalContext": out.Context,
		}}
	default:
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return append(data, '\n')
}
