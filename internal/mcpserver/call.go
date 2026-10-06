package mcpserver

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/config"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/gitinfo"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// runWindow is how far back a call folds the event log, as serve does.
const runWindow = 48 * time.Hour

var runPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}:[A-Za-z0-9_.-]{1,128}(/[A-Za-z0-9_.-]{1,128})?$`)

// Attribution is the optional run argument every tool accepts (MCP-4).
type Attribution struct {
	Run string `json:"run,omitempty" jsonschema:"your run id from the recovery note, if the tool asks for it"`
}

// call is one tool call's view of the board: read fresh, so every call sees
// other writers' changes (no index in mcp mode, NFR-4).
type call struct {
	srv      *server
	now      time.Time
	settings config.Config
	runs     runs.Settings
	board    board.Board
	analysis board.Analysis
	where    gitinfo.Info
	// project is the caller's project, "" when it has none.
	project string
	set     *runs.Set
	folded  map[string]bool
	// run is the caller's run id, "" when unknown; candidates lists the
	// live runs that made it ambiguous.
	run        string
	candidates []string
}

// begin reads the board and works out who is calling.
func (srv *server) begin(runArg string) (*call, error) {
	runArg = strings.TrimSpace(runArg)
	if runArg != "" && !runPattern.MatchString(runArg) {
		return nil, fail("invalid_input", "pass the run id as the recovery note shows it, like claude:3f2a9c1e, or leave it out", "run %q is not a run id", runArg)
	}
	settings, err := config.Load(srv.options.Root)
	if err != nil {
		return nil, err
	}
	c := &call{srv: srv, now: srv.options.Now().UTC(), settings: settings, runs: runs.SettingsFor(settings.QuietMinutes, settings.LeaseMinutes), set: runs.NewSet(), folded: map[string]bool{}}
	if c.board, _, err = srv.store.ReadBoard(); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// No board root yet: an empty board until the first write.
			c.analysis = board.Analyze(c.board)
			c.attribute(runArg)
			return c, nil
		}
		return nil, err
	}
	c.where = srv.store.Where(srv.options.Cwd, c.now)
	project, err := srv.store.ProjectFor(c.where.Project, c.where.Repo, settings.AutoCreateProjects)
	switch {
	case err == nil:
		c.project = project
		if !slices.ContainsFunc(c.board.Projects, func(p board.Project) bool { return p.Name == project }) {
			// Just created: read again so the project is in the board.
			if c.board, _, err = srv.store.ReadBoard(); err != nil {
				return nil, err
			}
		}
	case errors.Is(err, store.ErrNotFound), errors.Is(err, store.ErrNeedsMigration):
	default:
		return nil, err
	}
	c.analysis = board.Analyze(c.board)
	if err := c.fold(c.project); err != nil {
		return nil, err
	}
	c.attribute(runArg)
	return c, nil
}

// fold adds a project's recent events to the call's runs.
func (c *call) fold(project string) error {
	if project == "" || c.folded[project] {
		return nil
	}
	c.folded[project] = true
	err := c.srv.log.Read(project, c.now.Add(-runWindow), c.set.Apply)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// attribute resolves the caller's run (agent-protocol §7.1): the run
// argument (in full or shortened as recovery notes show it), else the one
// live session working in this worktree on this branch.
func (c *call) attribute(runArg string) {
	if runArg != "" {
		c.run = runArg
		var matches []string
		for _, r := range c.set.Runs() {
			if r.ID == runArg {
				return
			}
			if strings.HasPrefix(r.ID, runArg) && !strings.Contains(r.ID[len(runArg):], "/") {
				matches = append(matches, r.ID)
			}
		}
		if len(matches) == 1 {
			c.run = matches[0]
		}
		return
	}
	if c.where.Worktree == "" && c.where.Repo == "" && c.srv.options.Cwd == "" {
		return
	}
	for _, r := range c.set.Runs() {
		if r.Kind != events.KindSession || c.set.State(r.ID, c.now, c.runs) == runs.Ended {
			continue
		}
		if !c.here(r) {
			continue
		}
		c.candidates = append(c.candidates, r.ID)
	}
	if len(c.candidates) == 1 {
		c.run, c.candidates = c.candidates[0], nil
	}
}

// here reports whether a run works where this server does.
func (c *call) here(r *runs.Run) bool {
	if c.where.Worktree != "" {
		return r.Worktree == c.where.Worktree && r.Branch == c.where.Branch
	}
	return r.Cwd != "" && r.Cwd == c.srv.options.Cwd
}

// requireRun returns the caller's run for writes that belong to one.
func (c *call) requireRun() (string, error) {
	if c.run != "" {
		return c.run, nil
	}
	if len(c.candidates) > 1 {
		short := make([]string, 0, len(c.candidates))
		for _, id := range c.candidates {
			short = append(short, protocol.ShortRun(id))
		}
		return "", fail("ambiguous_run", "pass run (your run id from the recovery note): one of "+strings.Join(short, ", "),
			"%d sessions are working in this worktree, so the caller is ambiguous", len(c.candidates))
	}
	return "", fail("ambiguous_run", "pass run, your run id from the recovery note ([Flashheart] run=…)",
		"no live session was found working in %s", c.srv.options.Cwd)
}

// by is who a write is attributed to.
func (c *call) by() string {
	if c.run == "" {
		return "unknown"
	}
	return c.run
}

// runLabel is the caller's run as shown to the model.
func (c *call) runLabel() string {
	if c.run == "" {
		return "unknown"
	}
	return protocol.ShortRun(c.run)
}

// session is the caller's top-level session run.
func (c *call) session() string {
	session, _, _ := strings.Cut(c.run, "/")
	return session
}

// callerProject returns the caller's parsed project.
func (c *call) callerProject() *board.Project {
	return c.projectNamed(c.project)
}

func (c *call) projectNamed(name string) *board.Project {
	for index := range c.board.Projects {
		if c.board.Projects[index].Name == name {
			return &c.board.Projects[index]
		}
	}
	return nil
}

// projectArg resolves a project argument (a name or a key); empty means
// the caller's.
func (c *call) projectArg(value string) (*board.Project, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if project := c.callerProject(); project != nil {
			return project, nil
		}
		return nil, fail("not_found", "pass project, one of: "+strings.Join(c.projectNames(), ", "), "this session's working directory is not a project on the board")
	}
	for index := range c.board.Projects {
		project := &c.board.Projects[index]
		if project.Name == value || strings.EqualFold(project.Key, value) && (!project.KeyDerived || project.OwnsIDs()) {
			return project, nil
		}
	}
	return nil, fail("not_found", "pass one of: "+strings.Join(c.projectNames(), ", "), "no project %q", value)
}

func (c *call) projectNames() []string {
	names := make([]string, 0, len(c.board.Projects))
	for _, project := range c.board.Projects {
		names = append(names, project.Name)
	}
	return names
}

// find returns a live ticket by id, folding its project's runs.
func (c *call) find(id string) (*board.Project, board.Ticket, error) {
	id = strings.TrimSpace(id)
	key, _, ok := board.ParseID(id)
	if !ok {
		return nil, board.Ticket{}, fail("invalid_input", "pass a ticket id like FH-42", "%q is not a ticket id", id)
	}
	for index := range c.board.Projects {
		project := &c.board.Projects[index]
		if project.Key != key {
			continue
		}
		for _, ticket := range project.Tickets {
			if ticket.ID == id {
				if err := c.fold(project.Name); err != nil {
					return nil, board.Ticket{}, err
				}
				return project, ticket, nil
			}
		}
		if slices.Contains(project.Archived, id) {
			return nil, board.Ticket{}, fail("not_found", "ask the human to unarchive it if work should continue", "%s is archived", id)
		}
	}
	return nil, board.Ticket{}, fail("not_found", "check the id with list_tickets", "no ticket %s", id)
}

// blocked describes a ticket's blocking reasons.
func (c *call) blocked(project *board.Project, ticket board.Ticket) []string {
	var reasons []string
	for _, reason := range c.analysis.Blocked[board.Ref{Project: project.Name, ID: ticket.ID}] {
		reasons = append(reasons, reason.Describe())
	}
	return reasons
}

// holder is the run holding a live claim on a ticket, or nil.
func (c *call) holder(id string) *runs.Run {
	return c.set.Holder(id, c.now, c.runs)
}

// linked is the caller's ticket: its claim, else the in-progress ticket on
// its branch (RUN-5).
func (c *call) linked() runs.Link {
	if c.run == "" {
		return runs.Link{}
	}
	return c.set.Link(c.run, func(project, branch string) []string {
		if p := c.projectNamed(project); p != nil {
			return p.InProgressOnBranch(branch)
		}
		return nil
	})
}

// state describes a run for other agents: state and last activity.
func (c *call) state(r *runs.Run) string {
	return string(c.set.State(r.ID, c.now, c.runs)) + ", active " + ago(c.now.Sub(r.LastActivity))
}

func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

// keysInUse lists other projects' keys (KEY-5).
func (c *call) keysInUse(except string) []string {
	var keys []string
	for _, project := range c.board.Projects {
		if project.Name != except && (!project.KeyDerived || project.OwnsIDs()) {
			keys = append(keys, project.Key)
		}
	}
	slices.Sort(keys)
	return keys
}
