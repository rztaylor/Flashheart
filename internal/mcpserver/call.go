package mcpserver

import (
	"errors"
	"fmt"
	"io/fs"
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
	// project is the caller's project, "" when it has none; archived names
	// the caller's project when it is archived (PRJ-5).
	project  string
	archived string
	// missing names the caller's project that a write would create.
	missing string
	// loaded lists the projects read in full; the board holds every other
	// project's head only (store.ReadHeads), so a call reads what it needs
	// at NFR-1 scale.
	loaded map[string]bool
	set    *runs.Set
	folded map[string]bool
	// seen holds every folded event, so runs are folded in time order
	// across projects.
	seen []events.Event
	// outside is set when the caller works outside any git repository, so
	// it has no project (PRJ-4).
	outside bool
	// run is the caller's run id, "" when unknown; candidates lists the
	// live runs that made it ambiguous. nearby lists live sessions of the
	// caller's agent in this worktree on another branch, which may be the
	// caller after a branch switch its hooks have not reported yet.
	run        string
	candidates []string
	nearby     []string
	// started is set when the caller is the run the server started for its
	// connection.
	started bool
	// inv is the tool call; policy is what its tool declared (policy.go).
	inv    *invocation
	policy policy
}

// start reads the board and works out who is calling (attribute). For a
// tool that writes, the caller's project is created (auto_create_projects)
// or given this repository, the cwd cache is kept, and a caller with no
// run gets one started for its connection. A tool that only reads changes
// nothing on the board: a project that does not exist yet is only named
// (c.missing) and no run is started.
func (srv *server) start(inv *invocation, runArg string, write bool) (c *call, err error) {
	// The wrapper reads which run the call was attributed to.
	defer func() {
		if c != nil && inv != nil {
			inv.run, inv.every = c.run, time.Duration(c.settings.ActivitySeconds)*time.Second
		}
	}()
	runArg = strings.TrimSpace(runArg)
	if runArg != "" && !runPattern.MatchString(runArg) {
		return nil, fail("invalid_input", "pass the run id as the recovery note shows it, like claude:3f2a9c1e, or leave it out", "run %q is not a run id", runArg)
	}
	settings, err := config.Load(srv.options.Root)
	if err != nil {
		return nil, err
	}
	c = &call{srv: srv, inv: inv, now: srv.options.Now().UTC(), settings: settings, runs: runs.SettingsFor(settings.QuietMinutes, settings.LeaseMinutes), set: runs.NewSet(), folded: map[string]bool{}, loaded: map[string]bool{}}
	if c.board, err = srv.store.ReadHeads(); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// No board root yet: an empty board until the first write.
			c.analysis = board.Analyze(c.board)
			c.where = srv.store.Locate(srv.options.Cwd, c.now)
			c.outside = c.where.Repo == ""
			return c, c.attribute(inv, runArg, write)
		}
		return nil, err
	}
	var project string
	if write {
		c.where = srv.store.Where(srv.options.Cwd, c.now)
		project, err = srv.store.ProjectFor(c.where.Project, c.where.Repo, settings.AutoCreateProjects)
	} else {
		c.where = srv.store.Locate(srv.options.Cwd, c.now)
		project, err = srv.store.FindProject(c.where.Project, c.where.Repo)
		if errors.Is(err, store.ErrNotFound) && settings.AutoCreateProjects && project != "" {
			c.missing, project = project, ""
		}
	}
	switch {
	case err == nil:
		c.project = project
		if !slices.ContainsFunc(c.board.Projects, func(p board.Project) bool { return p.Name == project }) {
			// Just created: read again so the project is in the board.
			if c.board, err = srv.store.ReadHeads(); err != nil {
				return nil, err
			}
		}
	case errors.Is(err, store.ErrProjectArchived):
		c.archived = project
	case errors.Is(err, store.ErrNoProject):
		c.outside = true
	case errors.Is(err, store.ErrNotFound), errors.Is(err, store.ErrNeedsMigration):
	default:
		return nil, err
	}
	if err := c.load(c.project); err != nil {
		return nil, err
	}
	if err := c.fold(c.project); err != nil {
		return nil, err
	}
	if err := c.attribute(inv, runArg, write); err != nil {
		return nil, err
	}
	return c, nil
}

// fold adds a project's recent events to the call's runs. A run's events
// can sit in several projects' logs (a claim on another project's ticket),
// so the runs are refolded from every project read so far in time order.
func (c *call) fold(project string) error {
	if project == "" || c.folded[project] {
		return nil
	}
	c.folded[project] = true
	err := c.srv.log.Read(project, c.now.Add(-runWindow), func(e events.Event) { c.seen = append(c.seen, e) })
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	slices.SortStableFunc(c.seen, func(a, b events.Event) int { return a.Time.Compare(b.Time) })
	c.set = runs.NewSet()
	for _, e := range c.seen {
		c.set.Apply(e)
	}
	return nil
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
	return events.RunOf(c.run).Session().String()
}

// blocked describes a ticket's blocking reasons.
func (c *call) blocked(project *board.Project, ticket board.Ticket) []string {
	var reasons []string
	for _, reason := range c.analysis.Blocked[board.Ref{Project: project.Name, ID: ticket.ID}] {
		reasons = append(reasons, reason.Describe())
	}
	return reasons
}

// holder is the run holding a live claim on a ticket, or nil. A claim's
// lease is renewed by the holder's activity wherever it works, so the
// project the claim names as the holder's home is read too (agent-protocol
// §6); if it cannot be read, the lease is judged on what has been.
func (c *call) holder(id string) *runs.Run {
	if r := c.set.Claimant(id); r != nil && r.Home != "" && !c.folded[r.Home] {
		_ = c.fold(r.Home)
	}
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
