package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/app"
	"github.com/rztaylor/flashheart/internal/background"
	"github.com/rztaylor/flashheart/internal/buildinfo"
	"github.com/rztaylor/flashheart/internal/hooks"
	"github.com/rztaylor/flashheart/internal/hooks/claude"
	"github.com/rztaylor/flashheart/internal/logfile"
	"github.com/rztaylor/flashheart/internal/mcpserver"
	"github.com/rztaylor/flashheart/internal/migrate"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/setup"
	"github.com/rztaylor/flashheart/internal/store"
)

// RootEnv names the environment variable that overrides the default root.
const RootEnv = "FLASHHEART_ROOT"

// backgroundStartTimeout bounds how long the launching process waits for the
// detached server to start and attempt to open the browser.
const backgroundStartTimeout = 20 * time.Second

// Handshake reports a background child's startup outcome to its launcher.
type Handshake interface {
	Ready(background.Report) error
	Fail(error) error
}

// Dependencies are process-owned collaborators used by Run.
type Dependencies struct {
	Build           buildinfo.Info
	Getenv          func(string) string
	HomeDir         func() (string, error)
	Executable      func() (string, error)
	RunApp          func(context.Context, app.Options) error
	StartBackground func(context.Context, background.Options) (background.Report, error)
	OpenHandshake   func() (Handshake, error)
	// OpenServeLog returns the background child's diagnostic log for a root.
	OpenServeLog func(root string) io.Writer
	// Stdin is the hook payload and MCP message source.
	Stdin io.Reader
	// Getwd returns the process's working directory.
	Getwd func() (string, error)
	// FindClaude locates the claude CLI that registers MCP servers.
	FindClaude func(home string) string
	// RunCommand runs an external command for setup; nil runs it directly.
	RunCommand func(name string, args ...string) ([]byte, error)
}

type usageError struct{ message string }

func (e usageError) Error() string { return e.message }

type globalFlags struct {
	root    string
	rootSet bool
	debug   bool
}

type command struct {
	name    string
	summary string
	usage   string
	detail  string
	run     func(ctx context.Context, env *environment, flags *flag.FlagSet, globals *globalFlags) error
}

type environment struct {
	stdout, stderr io.Writer
	deps           Dependencies
	serve          *serveFlags
	migrate        *migrateFlags
	setup          *setupFlags
}

type serveFlags struct {
	foreground      bool
	backgroundChild bool
}

type setupFlags struct {
	write, uninstall bool
}

type migrateFlags struct {
	write bool
	keys  keyFlags
}

// keyFlags collects repeated --key project=KEY arguments.
type keyFlags map[string]string

func (k keyFlags) String() string { return "" }

func (k keyFlags) Set(value string) error {
	project, key, found := strings.Cut(value, "=")
	if !found || project == "" || key == "" {
		return fmt.Errorf("--key expects project=KEY, got %q", value)
	}
	k[project] = key
	return nil
}

var errNotYetAvailable = errors.New("not yet available")

func commands() []command {
	notYet := func(context.Context, *environment, *flag.FlagSet, *globalFlags) error { return errNotYetAvailable }
	return []command{
		{
			name:    "serve",
			summary: "open the board in your browser (default)",
			usage:   "serve [--root DIR] [--debug] [--foreground]",
			detail: "Open the board in your browser. The server runs in the background and\n" +
				"returns this terminal; it stops when you choose Quit or close its last tab.",
			run: runServe,
		},
		{
			name:    "mcp",
			summary: "run the MCP server for agents on stdio",
			usage:   "mcp [--root DIR]",
			detail: "Serve the Flashheart MCP tools on stdin and stdout. Agents start this\n" +
				"from their MCP configuration (flashheart setup writes it); it works on\n" +
				"the project of $CLAUDE_PROJECT_DIR, else of the working directory.",
			run: runMCP,
		},
		{
			name:    "hook",
			summary: "record an agent hook event read from stdin",
			usage:   "hook <agent> <event> [--root DIR]",
			detail: "Record a Claude Code hook event (agent: claude) read from stdin in the\n" +
				"board's event log, and print the recovery note at session start. Agents\n" +
				"run this from their hook configuration; it always exits 0 and logs\n" +
				"problems to <root>/.flashheart/hook-errors.log.",
			run: runHook,
		},
		{
			name:    "setup",
			summary: "show or apply an agent's configuration for Flashheart",
			usage:   "setup claude [--uninstall] [--write] [--root DIR]",
			detail: "Show the changes that connect Claude Code to Flashheart: its hooks in\n" +
				"~/.claude/settings.json, the flashheart MCP server, and the Flashheart\n" +
				"skill (which replaces kanban-tracker). Nothing changes without --write;\n" +
				"backups go to ~/.claude/flashheart-backup/. --uninstall shows, and with\n" +
				"--write applies, the reverse.",
			run: runSetup,
		},
		{name: "doctor", summary: "check the board root and agent configuration (not yet available)", usage: "doctor [--root DIR]", detail: "Check the board root, permissions, agent configuration and recent hook errors.", run: notYet},
		{
			name:    "migrate",
			summary: "convert a board from format v1 to v2",
			usage:   "migrate [--root DIR] [--key PROJECT=KEY]... [--write]",
			detail: "Show how a v1 board (column folders) becomes format v2: ticket ids in\n" +
				"creation order, a folder per ticket, references rewritten. Nothing is\n" +
				"written without --write; replaced files move into .flashheart/backup/.",
			run: runMigrate,
		},
		{name: "version", summary: "print version information", usage: "version", detail: "Print the Flashheart version, commit and agent protocol version.", run: runVersion},
	}
}

// Run executes the command line and returns a process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, deps Dependencies) int {
	if ctx == nil {
		ctx = context.Background()
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	env := &environment{stdout: stdout, stderr: stderr, deps: deps, serve: &serveFlags{}, migrate: &migrateFlags{keys: keyFlags{}}, setup: &setupFlags{}}

	name, flagArgs := splitCommand(args)
	if name == "" && wantsHelp(flagArgs) {
		writeUsage(stdout)
		return 0
	}
	if name == "" {
		name = "serve"
	}
	var selected *command
	for _, candidate := range commands() {
		if candidate.name == name {
			selected = &candidate
			break
		}
	}
	if selected == nil {
		return reportUsage(stderr, usageError{fmt.Sprintf("unknown command %q", name)})
	}

	globals := &globalFlags{}
	flags := newFlagSet(name, globals)
	switch name {
	case "serve":
		flags.BoolVar(&env.serve.foreground, "foreground", false, "")
		flags.BoolVar(&env.serve.backgroundChild, "background-child", false, "")
	case "migrate":
		flags.BoolVar(&env.migrate.write, "write", false, "")
		flags.Var(env.migrate.keys, "key", "")
	case "setup":
		flags.BoolVar(&env.setup.write, "write", false, "")
		flags.BoolVar(&env.setup.uninstall, "uninstall", false, "")
	}
	if err := parseInterspersed(flags, flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeCommandUsage(stdout, *selected)
			return 0
		}
		if name == "hook" {
			// Exit 2 would tell the agent to block; a hook never does. Parsing
			// stopped early, so find --root in the raw arguments for the log.
			globals.root, globals.rootSet = rootArgument(flagArgs)
			reportHookProblem(env, globals, err.Error(), nil)
			return 0
		}
		return reportUsage(stderr, usageError{err.Error()})
	}
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "root" {
			globals.rootSet = true
		}
	})

	err := selected.run(ctx, env, flags, globals)
	var usage usageError
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errNotYetAvailable):
		what := name
		if name == "setup" && flags.NArg() > 0 {
			what += " " + flags.Arg(0)
		}
		fmt.Fprintf(stderr, "flashheart: %s is not yet available\n", what)
		return 2
	case errors.As(err, &usage):
		return reportUsage(stderr, usage)
	default:
		fmt.Fprintf(stderr, "flashheart: %v\n", err)
		return 1
	}
}

// splitCommand finds the first positional argument (the command) and returns
// it with every other argument, so global flags may appear before or after it.
func splitCommand(args []string) (string, []string) {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			return "", args
		}
		if !strings.HasPrefix(arg, "-") {
			rest := append(append([]string{}, args[:index]...), args[index+1:]...)
			return arg, rest
		}
		if arg == "--root" || arg == "-root" {
			index++
		}
	}
	return "", args
}

// parseInterspersed parses flags wherever they appear among a command's
// arguments, so `hook claude Stop --root DIR` works; flags.Args() then holds
// the positional arguments in order.
func parseInterspersed(flags *flag.FlagSet, args []string) error {
	var positional []string
	for {
		if err := flags.Parse(args); err != nil {
			return err
		}
		rest := flags.Args()
		if len(rest) == 0 {
			break
		}
		if rest[0] == "--" || (len(args) > len(rest) && args[len(args)-len(rest)-1] == "--") {
			positional = append(positional, rest...)
			break
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
	return flags.Parse(append([]string{"--"}, positional...))
}

func wantsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "-help" || arg == "--help" {
			return true
		}
	}
	return false
}

func newFlagSet(name string, globals *globalFlags) *flag.FlagSet {
	flags := flag.NewFlagSet("flashheart "+name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&globals.root, "root", "", "")
	flags.BoolVar(&globals.debug, "debug", false, "")
	return flags
}

func runVersion(_ context.Context, env *environment, flags *flag.FlagSet, _ *globalFlags) error {
	if flags.NArg() > 0 {
		return usageError{"version takes no arguments"}
	}
	fmt.Fprintln(env.stdout, env.deps.Build.String())
	fmt.Fprintf(env.stdout, "agent protocol %d\n", protocol.Version)
	return nil
}

func runServe(ctx context.Context, env *environment, flags *flag.FlagSet, globals *globalFlags) error {
	if flags.NArg() > 0 {
		return usageError{"serve takes no arguments"}
	}
	root, err := resolveRoot(globals, env.deps)
	if err != nil {
		return err
	}
	options := app.Options{Build: env.deps.Build, Root: root, Debug: globals.debug}

	switch {
	case env.serve.backgroundChild:
		return serveBackgroundChild(ctx, env, options)
	case env.serve.foreground:
		return serveForeground(ctx, env, options)
	default:
		return serveBackground(ctx, env, options)
	}
}

func serveForeground(ctx context.Context, env *environment, options app.Options) error {
	if env.deps.RunApp == nil {
		return errors.New("application runner is unavailable")
	}
	options.Stdout, options.Stderr = env.stdout, env.stderr
	options.Launched = func(launched app.Launched) {
		if options.Debug {
			fmt.Fprintf(env.stderr, "flashheart listening on loopback %s\n", launched.Address)
		}
		writeManualURL(env.stderr, launched.BrowserError, launched.ManualURL)
	}
	return env.deps.RunApp(ctx, options)
}

func serveBackground(ctx context.Context, env *environment, options app.Options) error {
	if env.deps.StartBackground == nil || env.deps.Executable == nil {
		return errors.New("background start is unavailable")
	}
	executable, err := env.deps.Executable()
	if err != nil {
		return fmt.Errorf("locate flashheart executable: %w", err)
	}
	args := []string{"serve", "--background-child", "--root", options.Root}
	if options.Debug {
		args = append(args, "--debug")
	}
	report, err := env.deps.StartBackground(ctx, background.Options{
		Executable: executable,
		Args:       args,
		Timeout:    backgroundStartTimeout,
	})
	if err != nil {
		return err
	}
	if options.Debug {
		fmt.Fprintf(env.stderr, "flashheart listening on loopback %s (running in the background)\n", report.Address)
	}
	writeManualURL(env.stderr, report.BrowserError, report.ManualURL)
	return nil
}

func serveBackgroundChild(ctx context.Context, env *environment, options app.Options) error {
	if env.deps.OpenHandshake == nil || env.deps.RunApp == nil {
		return errors.New("background child support is unavailable")
	}
	handshake, err := env.deps.OpenHandshake()
	if err != nil {
		return fmt.Errorf("open background handshake: %w", err)
	}
	// The child's standard streams are discarded; diagnostics, including the
	// standard logger used by net/http, go to <root>/.flashheart/serve.log.
	diagnostics := io.Discard
	if env.deps.OpenServeLog != nil {
		diagnostics = env.deps.OpenServeLog(options.Root)
		log.SetOutput(diagnostics)
	}
	reported := false
	options.Stdout, options.Stderr = diagnostics, diagnostics
	options.Launched = func(launched app.Launched) {
		reported = true
		_ = handshake.Ready(background.Report{
			Address:      launched.Address,
			ManualURL:    launched.ManualURL,
			BrowserError: launched.BrowserError,
		})
	}
	err = env.deps.RunApp(ctx, options)
	switch {
	case err == nil:
	case !reported:
		_ = handshake.Fail(err)
	default:
		fmt.Fprintf(diagnostics, "flashheart: %v\n", err)
	}
	return err
}

// hookAdapters are the agents whose hook payloads Flashheart reads (HOOK-7).
var hookAdapters = map[string]hooks.Adapter{claude.Agent: claude.Adapter{}}

// runHook records one hook event. It always succeeds: a hook must never
// break the agent's session (HOOK-1), so problems are logged and reported
// on stderr only, which agents do not show the model.
func runHook(_ context.Context, env *environment, flags *flag.FlagSet, globals *globalFlags) error {
	root, err := resolveRoot(globals, env.deps)
	if err != nil {
		fmt.Fprintf(env.stderr, "flashheart hook: %v\n", err)
		return nil
	}
	args := flags.Args()
	problem := ""
	var adapter hooks.Adapter
	switch {
	case len(args) != 2:
		problem = "usage: flashheart hook <agent> <event>"
	case hookAdapters[args[0]] == nil:
		problem = fmt.Sprintf("unknown agent %q (known: %s)", args[0], claude.Agent)
	default:
		adapter = hookAdapters[args[0]]
	}
	if problem != "" {
		reportHookProblem(env, globals, problem, args)
		return nil
	}
	stdin := env.deps.Stdin
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	hooks.Run(hooks.Options{Root: root, Event: args[1], Stdin: stdin, Stdout: env.stdout, Adapter: adapter})
	return nil
}

// projectDirEnv is where Claude Code says the session works; user-scope
// MCP servers start in ~/.claude, not the project (agent-protocol §7.1).
const projectDirEnv = "CLAUDE_PROJECT_DIR"

// runMCP serves the MCP tools on stdin and stdout until the agent closes
// the connection. Only protocol messages reach stdout (CLI-3).
func runMCP(ctx context.Context, env *environment, flags *flag.FlagSet, globals *globalFlags) error {
	if flags.NArg() > 0 {
		return usageError{"mcp takes no arguments"}
	}
	root, err := resolveRoot(globals, env.deps)
	if err != nil {
		return err
	}
	cwd := ""
	if env.deps.Getenv != nil {
		cwd = strings.TrimSpace(env.deps.Getenv(projectDirEnv))
	}
	if cwd == "" && env.deps.Getwd != nil {
		cwd, _ = env.deps.Getwd()
	}
	stdin := env.deps.Stdin
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	return mcpserver.Serve(ctx, mcpserver.Options{Root: root, Cwd: cwd}, stdin, env.stdout)
}

// runSetup shows, or with --write applies, an agent's configuration
// (SET-1–SET-3, D9).
func runSetup(_ context.Context, env *environment, flags *flag.FlagSet, globals *globalFlags) error {
	switch {
	case flags.NArg() == 0:
		return usageError{"setup needs an agent: flashheart setup claude"}
	case flags.NArg() > 1:
		return usageError{"setup takes one agent"}
	case flags.Arg(0) == "codex":
		return errNotYetAvailable
	case flags.Arg(0) != "claude":
		return usageError{fmt.Sprintf("unknown agent %q (known: claude)", flags.Arg(0))}
	}
	root, err := resolveRoot(globals, env.deps)
	if err != nil {
		return err
	}
	if env.deps.HomeDir == nil || env.deps.Executable == nil {
		return errors.New("setup needs the home directory and the flashheart binary's path")
	}
	home, err := env.deps.HomeDir()
	if err != nil {
		return fmt.Errorf("find the home directory: %w", err)
	}
	binary, err := env.deps.Executable()
	if err != nil {
		return fmt.Errorf("find the flashheart binary: %w", err)
	}
	options := setup.Options{Home: home, Binary: binary, Run: env.deps.RunCommand}
	if root != filepath.Join(home, "reports", "Kanban") {
		options.Root = root
	}
	if env.deps.FindClaude != nil {
		options.Claude = env.deps.FindClaude(home)
	}
	plan, err := setup.Install(options)
	if env.setup.uninstall {
		plan, err = setup.Uninstall(options)
	}
	if err != nil {
		return err
	}
	if env.setup.write {
		return plan.Apply(env.stdout)
	}
	plan.Render(env.stdout)
	if !plan.Empty() {
		again := "--write"
		if env.setup.uninstall {
			again = "--uninstall --write"
		}
		fmt.Fprintf(env.stdout, "\nNothing was changed. Run flashheart setup claude %s to apply these changes.\n", again)
	}
	return nil
}

// rootArgument finds --root DIR or --root=DIR without parsing other flags.
func rootArgument(args []string) (string, bool) {
	for index, arg := range args {
		for _, prefix := range []string{"--root=", "-root="} {
			if value, found := strings.CutPrefix(arg, prefix); found {
				return value, true
			}
		}
		if (arg == "--root" || arg == "-root") && index+1 < len(args) {
			return args[index+1], true
		}
	}
	return "", false
}

// reportHookProblem tells a person on stderr and logs to hook-errors.log
// when the root can be resolved; agents do not show stderr from a hook that
// exits 0.
func reportHookProblem(env *environment, globals *globalFlags, problem string, args []string) {
	fmt.Fprintf(env.stderr, "flashheart hook: %s\n", problem)
	root, err := resolveRoot(globals, env.deps)
	if err != nil {
		return
	}
	log := logfile.HookErrors(root)
	_, _ = fmt.Fprintf(log, "hook %s: %s", strings.Join(args, " "), problem)
	_ = log.Close()
}

func runMigrate(_ context.Context, env *environment, flags *flag.FlagSet, globals *globalFlags) error {
	if flags.NArg() > 0 {
		return usageError{"migrate takes no arguments"}
	}
	root, err := resolveRoot(globals, env.deps)
	if err != nil {
		return err
	}
	s, err := store.Open(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("board root %s does not exist", root)
		}
		return err
	}
	defer s.Close()
	plan, err := migrate.Prepare(s, migrate.Options{Keys: env.migrate.keys})
	if err != nil {
		return err
	}
	if !env.migrate.write || len(plan.Projects) == 0 {
		plan.Write(env.stdout)
		return nil
	}
	if err := migrate.Apply(s, plan); err != nil {
		return fmt.Errorf("migration stopped part way: %w (files already converted are in place; the v1 files moved so far are in %s)", err, plan.Backup)
	}
	tickets := 0
	for _, project := range plan.Projects {
		tickets += len(project.Tickets)
	}
	fmt.Fprintf(env.stdout, "Migrated %s to board format v2: %d projects, %d tickets.\nThe v1 files are in %s.\n", root, len(plan.Projects), tickets, plan.Backup)
	return nil
}

func writeManualURL(output io.Writer, browserError, manualURL string) {
	if manualURL == "" {
		return
	}
	fmt.Fprintf(output, "Could not open a browser: %s\nOpen this URL within two minutes: %s\n", browserError, manualURL)
}

// resolveRoot applies the precedence --root, FLASHHEART_ROOT, ~/reports/Kanban
// and returns a clean absolute path with a leading ~ expanded.
func resolveRoot(globals *globalFlags, deps Dependencies) (string, error) {
	value := globals.root
	if globals.rootSet && strings.TrimSpace(value) == "" {
		return "", usageError{"--root must not be empty"}
	}
	if !globals.rootSet && deps.Getenv != nil {
		value = strings.TrimSpace(deps.Getenv(RootEnv))
	}
	if value == "" {
		value = filepath.Join("~", "reports", "Kanban")
	}
	if value == "~" || strings.HasPrefix(value, "~/") {
		if deps.HomeDir == nil {
			return "", errors.New("cannot expand ~: home directory is unavailable")
		}
		home, err := deps.HomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %w", err)
		}
		value = filepath.Join(home, strings.TrimPrefix(value, "~"))
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve board root %q: %w", value, err)
	}
	return absolute, nil
}

func reportUsage(stderr io.Writer, err usageError) int {
	fmt.Fprintf(stderr, "flashheart: %s\n\n", err.message)
	writeUsage(stderr)
	return 2
}

func writeUsage(output io.Writer) {
	fmt.Fprintln(output, "Usage: flashheart [--root DIR] [--debug] [command]")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "A local kanban board for AI-assisted projects, and the MCP server and hook")
	fmt.Fprintln(output, "handler its agents use.")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Commands:")
	for _, command := range commands() {
		fmt.Fprintf(output, "  %-9s %s\n", command.name, command.summary)
	}
	fmt.Fprintln(output)
	writeGlobalOptions(output)
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Run 'flashheart <command> --help' for help with a command.")
}

func writeCommandUsage(output io.Writer, command command) {
	fmt.Fprintf(output, "Usage: flashheart %s\n\n%s\n\n", command.usage, command.detail)
	switch command.name {
	case "serve":
		fmt.Fprintln(output, "Options:")
		fmt.Fprintln(output, "  --foreground  keep the server attached to this terminal until it stops")
		writeGlobalOptionLines(output)
		return
	case "setup":
		fmt.Fprintln(output, "Options:")
		fmt.Fprintln(output, "  --write       apply the changes (otherwise only show them)")
		fmt.Fprintln(output, "  --uninstall   show (with --write, apply) the reverse")
		writeGlobalOptionLines(output)
		return
	case "migrate":
		fmt.Fprintln(output, "Options:")
		fmt.Fprintln(output, "  --write       apply the plan (otherwise only show it)")
		fmt.Fprintln(output, "  --key P=KEY   use KEY as project P's ticket key (repeatable)")
		writeGlobalOptionLines(output)
		return
	}
	writeGlobalOptions(output)
}

func writeGlobalOptions(output io.Writer) {
	fmt.Fprintln(output, "Options:")
	writeGlobalOptionLines(output)
}

func writeGlobalOptionLines(output io.Writer) {
	fmt.Fprintf(output, "  --root DIR    board root (default $%s, else ~/reports/Kanban)\n", RootEnv)
	fmt.Fprintln(output, "  --debug       print the listener address and lifecycle summaries")
	fmt.Fprintln(output, "  -h, --help    show help")
}
