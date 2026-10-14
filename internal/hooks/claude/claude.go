package claude

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/hooks"
)

// Agent is Claude Code's agent id (agent-protocol §2).
const Agent = "claude"

// Adapter maps Claude Code hook payloads.
type Adapter struct{}

// payload holds the fields Flashheart reads. Prompts, messages, tool
// responses and transcripts are never decoded beyond what is listed here,
// and a prompt is only classified by who sent it (origin), never kept.
type payload struct {
	SessionID        string          `json:"session_id"`
	Prompt           string          `json:"prompt"`
	Cwd              string          `json:"cwd"`
	Source           string          `json:"source"`
	Reason           string          `json:"reason"`
	ToolName         string          `json:"tool_name"`
	ToolInput        json.RawMessage `json:"tool_input"`
	ToolResponse     json.RawMessage `json:"tool_response"`
	AgentID          string          `json:"agent_id"`
	AgentType        string          `json:"agent_type"`
	NotificationType string          `json:"notification_type"`
	TaskID           flexibleID      `json:"task_id"`
	TaskSubject      string          `json:"task_subject"`
	StopHookActive   bool            `json:"stop_hook_active"`
}

// toolPrefix names Flashheart's own MCP tools in Claude Code.
const toolPrefix = "mcp__flashheart__"

// checkpointTool is Flashheart's checkpoint tool in Claude Code.
const checkpointTool = toolPrefix + "checkpoint"

// Prompts the user did not type start with one of these (agent-protocol
// §5.2; testdata/hooks/claude/MANIFEST.md). Claude Code submits
// <task-notification> when one of the agent's background commands or
// agents finishes (recorded, 2.1.291), and the Claude desktop app submits
// <ci-monitor-event> for its pull request monitor. A subagent's hand-back
// and a message from another agent arrive as <agent-message>, which the
// desktop app introduces with agentMessageLead (both from session
// transcripts, 2026-10). A prompt the user types may begin with a
// <system-reminder> the app adds; it stays the user's.
var taskPrefixes = []string{"<task-notification>", "<ci-monitor-event>"}

const (
	agentPrefix      = "<agent-message"
	agentMessageLead = "Another Claude session sent a message:"
)

// origin classifies who started a turn from its prompt, which is read for
// nothing else and never kept.
func origin(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	for _, prefix := range taskPrefixes {
		if strings.HasPrefix(prompt, prefix) {
			return events.OriginTask
		}
	}
	if rest, ok := strings.CutPrefix(prompt, agentMessageLead); ok {
		prompt = strings.TrimSpace(rest)
	}
	if strings.HasPrefix(prompt, agentPrefix) {
		return events.OriginAgent
	}
	return events.OriginUser
}

// flexibleID accepts a string or a number.
type flexibleID string

func (f *flexibleID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = flexibleID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexibleID(n.String())
	}
	return nil
}

// Agent returns "claude".
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
	// Tool and permission events from inside a subagent carry its agent_id.
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
		from := origin(p.Prompt)
		add(session, events.TurnStart, events.TurnStartData{Origin: from, Background: from != events.OriginUser})
		input.Inbox, input.Notes = session, true
	case "PreToolUse":
		input.Reply = stamp(p.ToolName, p.ToolInput, actor)
	case "PostToolUse", "PostToolUseFailure":
		ok := event == "PostToolUse"
		add(actor, events.ToolUsed, toolResult(p.ToolName, ok, p.ToolInput))
		// A checkpoint settles the session's edits, so the paths it recorded
		// may be recorded again; a failed one may still have been written,
		// and recording a path twice is harmless.
		input.Settled = p.ToolName == checkpointTool
		// Claude Code shows the model a tool result's additionalContext,
		// so a due claim reminder can go with it (D50).
		input.Notes = true
		if ok {
			if plan, found := planUpdate(p.ToolName, p.ToolInput, p.ToolResponse); found {
				add(actor, events.PlanUpdated, plan)
			}
		}
	case "PermissionRequest":
		add(actor, events.PermissionRequested, events.PermissionData{Tool: p.ToolName})
	case "PermissionDenied":
		add(actor, events.PermissionResolved, events.ResolvedData{Outcome: "denied", Tool: p.ToolName})
	case "Notification":
		kind := notificationType(p.NotificationType)
		add(actor, events.Notification, events.NotificationData{Type: kind})
		if kind == "permission" {
			add(actor, events.PermissionRequested, events.PermissionData{})
		}
	case "TaskCreated", "TaskCompleted":
		status := events.PlanPending
		if event == "TaskCompleted" {
			status = events.PlanCompleted
		}
		if p.TaskID != "" {
			add(actor, events.PlanUpdated, events.PlanData{Merge: true, Items: []events.PlanItem{{ID: string(p.TaskID), Text: p.TaskSubject, Status: status}}})
		}
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
	case "SessionEnd":
		add(session, events.RunEnd, events.RunEndData{Reason: p.Reason})
	}
	return input, nil
}

// stamp adds the calling run to a Flashheart tool's input, so the MCP
// server knows who called (agent-protocol §7.1). Claude Code applies
// updatedInput without a permission decision; other tools are untouched.
func stamp(tool string, input json.RawMessage, run string) *hooks.Output {
	if !strings.HasPrefix(tool, toolPrefix) {
		return nil
	}
	fields := map[string]json.RawMessage{}
	if len(input) > 0 && json.Unmarshal(input, &fields) != nil {
		return nil
	}
	if fields == nil {
		fields = map[string]json.RawMessage{}
	}
	encoded, _ := json.Marshal(run)
	fields["run"] = encoded
	updated, err := json.Marshal(fields)
	if err != nil {
		return nil
	}
	return &hooks.Output{UpdatedInput: updated}
}

var knownNotification = regexp.MustCompile(`^[a-z_]{1,40}$`)

func notificationType(value string) string {
	switch value {
	case "permission_prompt":
		return "permission"
	case "idle_prompt":
		return "idle"
	}
	if knownNotification.MatchString(value) {
		return value
	}
	return "other"
}

// shellTools are Claude Code's shell command tools, whose edits name no
// path.
var shellTools = []string{"Bash", "PowerShell"}

// toolResult classifies a tool result for hooks: a shell command, or an
// edit and the file it changed.
func toolResult(tool string, ok bool, input json.RawMessage) events.ToolData {
	data := events.ToolData{Tool: tool, OK: ok, Shell: slices.Contains(shellTools, tool)}
	if path := editPath(tool, input); path != "" {
		data.Paths = []string{path}
	}
	return data
}

// editPath returns the file an edit tool changed, as given (absolute).
func editPath(tool string, input json.RawMessage) string {
	var fields struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	}
	switch tool {
	case "Edit", "Write", "MultiEdit":
		_ = json.Unmarshal(input, &fields)
		return fields.FilePath
	case "NotebookEdit":
		_ = json.Unmarshal(input, &fields)
		return fields.NotebookPath
	}
	return ""
}

func planStatus(value string) string {
	switch value {
	case events.PlanInProgress, events.PlanCompleted, events.PlanDeleted:
		return value
	}
	return events.PlanPending
}

// planUpdate reads the plan from TodoWrite (the whole list) and the task
// tools (one item, merged by id) (RUN-4).
func planUpdate(tool string, input, response json.RawMessage) (events.PlanData, bool) {
	switch tool {
	case "TodoWrite":
		var todo struct {
			Todos []struct {
				Content string `json:"content"`
				Status  string `json:"status"`
			} `json:"todos"`
		}
		if json.Unmarshal(input, &todo) != nil {
			return events.PlanData{}, false
		}
		plan := events.PlanData{Items: []events.PlanItem{}}
		for _, item := range todo.Todos {
			plan.Items = append(plan.Items, events.PlanItem{Text: item.Content, Status: planStatus(item.Status)})
		}
		return plan, true
	case "TaskCreate":
		var in struct {
			Subject string `json:"subject"`
		}
		var out struct {
			Task struct {
				ID flexibleID `json:"id"`
			} `json:"task"`
		}
		_ = json.Unmarshal(input, &in)
		_ = json.Unmarshal(response, &out)
		id := string(out.Task.ID)
		if id == "" {
			id = in.Subject
		}
		if id == "" {
			return events.PlanData{}, false
		}
		return events.PlanData{Merge: true, Items: []events.PlanItem{{ID: id, Text: in.Subject, Status: events.PlanPending}}}, true
	case "TaskUpdate":
		var in struct {
			TaskID  flexibleID `json:"taskId"`
			Status  string     `json:"status"`
			Subject string     `json:"subject"`
		}
		if json.Unmarshal(input, &in) != nil || in.TaskID == "" || in.Status == "" {
			return events.PlanData{}, false
		}
		return events.PlanData{Merge: true, Items: []events.PlanItem{{ID: string(in.TaskID), Text: in.Subject, Status: planStatus(in.Status)}}}, true
	}
	return events.PlanData{}, false
}

// Render returns Claude Code's hook output: a stop decision, a tool's
// updated input or additional context as hookSpecificOutput, or nothing.
func (Adapter) Render(event string, out hooks.Output) []byte {
	var value any
	switch {
	case out.Block != "":
		value = map[string]string{"decision": "block", "reason": out.Block}
	case len(out.UpdatedInput) > 0:
		value = map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName": event,
			"updatedInput":  json.RawMessage(out.UpdatedInput),
		}}
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
