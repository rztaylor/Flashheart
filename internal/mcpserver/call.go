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
}

// begin reads the board and works out who is calling, for a tool that
// writes: the caller's project is created (auto_create_projects) or given
// this repository, the cwd cache is kept, and a caller with no run gets
// one started for its connection.
func (srv *server) begin(inv *invocation, runArg string) (*call, error) {
	return srv.start(inv, runArg, true)
}

// read is begin for a tool that only reads: it changes nothing on the
// board, so a project that does not exist yet is only named (c.missing)
// and no run is started.
func (srv *server) read(inv *invocation, runArg string) (*call, error) {
	return srv.start(inv, runArg, false)
}

func (srv *server) start(inv *invocation, runArg string, write bool) (c *call, err error) {
	// The wrapper reads which run the call was attributed to.
	defer func() {
		if c != nil && inv != nil {
			inv.run = c.run
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
	c = &call{srv: srv, now: srv.options.Now().UTC(), settings: settings, runs: runs.SettingsFor(settings.QuietMinutes, settings.LeaseMinutes), set: runs.NewSet(), folded: map[string]bool{}, loaded: map[string]bool{}}
	if c.board, err = srv.store.ReadHeads(); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// No board root yet: an empty board until the first write.
			c.analysis = board.Analyze(c.board)
			return c, c.attribute(inv, runArg)
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
	if err := c.attribute(inv, runArg); err != nil {
		return nil, err
	}
	if err := c.own(inv, write); err != nil {
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

// fullSession is the length from which a session id is taken as complete
// rather than shortened (agent-protocol §2 shortens to 8 characters).
const fullSession = 16

// attribute resolves the caller's run (agent-protocol §7.1): the run
// argument (in full, or shortened as recovery notes show it, which must
// name one known run), else the run the server started for this
// connection, else the one live session of the caller's agent working in
// this worktree on this branch. A run the server started is never another
// connection's caller.
func (c *call) attribute(inv *invocation, runArg string) error {
	if runArg != "" {
		session, agent, _ := strings.Cut(runArg, "/")
		var matches []string
		for _, r := range c.set.Runs() {
			if r.ID == runArg {
				c.run = r.ID
				return nil
			}
			rSession, rAgent, _ := strings.Cut(r.ID, "/")
			if strings.HasPrefix(rSession, session) && rAgent == agent {
				matches = append(matches, r.ID)
			}
		}
		_, sessionID, _ := strings.Cut(session, ":")
		switch {
		case len(matches) == 1:
			c.run = matches[0]
		case len(matches) > 1:
			return fail("ambiguous_run", "pass your full run id, one of "+strings.Join(matches, ", "), "run %q names %d runs", runArg, len(matches))
		case len(sessionID) < fullSession:
			return fail("invalid_input", "pass the run id from your recovery note, or leave run out", "no run matches %q", runArg)
		default:
			// A full id not seen yet: a session whose hooks have not reported.
			c.run = runArg
		}
		return nil
	}
	// One server serves one session, so a run it started is the caller's
	// for the rest of the connection.
	if inv != nil {
		if run, _ := inv.conn.current(); run != "" {
			c.run, c.started = run, true
			return nil
		}
	}
	if c.where.Worktree == "" && c.where.Repo == "" && c.srv.options.Cwd == "" {
		return nil
	}
	for _, r := range c.set.Runs() {
		if r.Kind != events.KindSession || r.Source == events.SourceMCP || c.set.State(r.ID, c.now, c.runs) == runs.Ended {
			continue
		}
		if inv != nil && inv.conn != nil && inv.conn.agent != "" && r.Agent != inv.conn.agent {
			continue
		}
		switch {
		case c.here(r):
			c.candidates = append(c.candidates, r.ID)
		case c.where.Worktree != "" && r.Worktree == c.where.Worktree:
			c.nearby = append(c.nearby, r.ID)
		}
	}
	if len(c.candidates) == 1 {
		c.run, c.candidates = c.candidates[0], nil
	}
	return nil
}

// guard refuses a write to a ticket held by another live session (one not
// the caller's own session or subagent, agent-protocol §7.2).
func (c *call) guard(id string) error {
	if holder := c.holder(id); holder != nil && !related(holder.ID, c.run) {
		return fail("claimed", "work on the ticket you hold, or claim this one first", "%s is held by %s (%s)", id, protocol.ShortRun(holder.ID), c.state(holder))
	}
	return nil
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
	// No run is recorded in an archived project's repository.
	if c.archived != "" {
		return "", c.archivedError()
	}
	if len(c.candidates) > 1 {
		return "", fail("ambiguous_run", "pass run (your run id from the recovery note): one of "+shortRuns(c.candidates),
			"%d sessions are working in this worktree, so the caller is ambiguous", len(c.candidates))
	}
	if len(c.nearby) > 0 {
		return "", fail("ambiguous_run", "pass run (your run id from the recovery note): one of "+shortRuns(c.nearby),
			"a session works in this worktree on another branch than %s, so the caller is ambiguous", c.where.Branch)
	}
	return "", fail("ambiguous_run", "pass run, your run id from the recovery note ([Flashheart] run=…)",
		"no live session was found working in %s", c.srv.options.Cwd)
}

func shortRuns(ids []string) string {
	short := make([]string, 0, len(ids))
	for _, id := range ids {
		short = append(short, protocol.ShortRun(id))
	}
	return strings.Join(short, ", ")
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

// load reads projects in full, with the projects their tickets depend on
// (one level, enough for every blocking reason they show), and analyses
// the board again.
func (c *call) load(names ...string) error {
	var wanted []string
	for _, name := range names {
		if name != "" && !c.loaded[name] && c.projectNamed(name) != nil {
			wanted = append(wanted, name)
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	// A dependency's project is the one whose folders hold its id (heads
	// carry ids); its key alone can mislead, since a project with no tickets
	// may derive the same key and a ticket may sit under another key.
	owner := map[string]string{}
	for _, project := range c.board.Projects {
		for _, ticket := range project.Tickets {
			if _, taken := owner[ticket.ID]; !taken {
				owner[ticket.ID] = project.Name
			}
		}
	}
	read := func(name string) error {
		project, err := c.srv.store.ReadProject(name)
		if err != nil {
			return err
		}
		*c.projectNamed(name) = project
		c.loaded[name] = true
		return nil
	}
	for _, name := range wanted {
		if err := read(name); err != nil {
			return err
		}
	}
	for _, name := range wanted {
		for _, ticket := range c.projectNamed(name).Tickets {
			for _, id := range ticket.DependsOn {
				if other, ok := owner[id]; ok && !c.loaded[other] {
					if err := read(other); err != nil {
						return err
					}
				}
			}
		}
	}
	c.analysis = board.Analyze(c.board)
	return nil
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
		if c.archived != "" {
			return nil, c.archivedError()
		}
		return nil, fail("not_found", "pass project, one of: "+strings.Join(c.projectNames(), ", "), "this session's working directory is not a project on the board")
	}
	for index := range c.board.Projects {
		project := &c.board.Projects[index]
		if project.Name == value || strings.EqualFold(project.Key, value) && (!project.KeyDerived || project.OwnsIDs()) {
			if err := c.load(project.Name); err != nil {
				return nil, err
			}
			return &c.board.Projects[index], nil
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
		if c.board.Projects[index].Key != key {
			continue
		}
		if err := c.load(c.board.Projects[index].Name); err != nil {
			return nil, board.Ticket{}, err
		}
		project := &c.board.Projects[index]
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
	for _, project := range c.board.ArchivedProjects {
		if slices.Contains(project.IDs, id) {
			return nil, board.Ticket{}, fail("project_archived", "ask the human to restore the project in Flashheart if work should continue", "%s belongs to archived project %s", id, project.Name)
		}
	}
	return nil, board.Ticket{}, fail("not_found", "check the id with list_tickets", "no ticket %s", id)
}

// archivedError explains that the caller's project is archived: nothing is
// recorded for it until the human restores it.
func (c *call) archivedError() error {
	return fail("project_archived", "ask the human to restore it from Flashheart's archive; nothing is recorded here until then", "project %q is archived", c.archived)
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

// projectOf is the project of a ticket id, or fallback when no live ticket
// has it.
func (c *call) projectOf(id, fallback string) string {
	key, _, _ := board.ParseID(id)
	for _, project := range c.board.Projects {
		if project.Key == key && slices.ContainsFunc(project.Tickets, func(t board.Ticket) bool { return t.ID == id }) {
			return project.Name
		}
	}
	return fallback
}

// projectsFor lists the ticket's project and, when different, the caller
// run's own project.
func (c *call) projectsFor(ticketProject string) []string {
	list := []string{ticketProject}
	if r := c.set.Get(c.run); r != nil && r.Project != "" && r.Project != ticketProject {
		list = append(list, r.Project)
	}
	return list
}
