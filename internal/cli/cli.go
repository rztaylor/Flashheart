package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/app"
	"github.com/rztaylor/flashheart/internal/background"
	"github.com/rztaylor/flashheart/internal/buildinfo"
	"github.com/rztaylor/flashheart/internal/protocol"
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
}

type serveFlags struct {
	foreground      bool
	backgroundChild bool
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
		{name: "mcp", summary: "run the MCP server on stdio (not yet available)", usage: "mcp [--root DIR]", detail: "Run the MCP server on stdio for Claude Code and Codex.", run: notYet},
		{name: "hook", summary: "handle an agent hook event (not yet available)", usage: "hook <agent> <event> [--root DIR]", detail: "Record an agent hook event read from stdin.", run: notYet},
		{name: "setup", summary: "show or apply agent configuration (not yet available)", usage: "setup <agent> [--write | --uninstall]", detail: "Show the hook, MCP and protocol changes for an agent; write them with --write.", run: notYet},
		{name: "doctor", summary: "check the board root and agent configuration (not yet available)", usage: "doctor [--root DIR]", detail: "Check the board root, permissions, agent configuration and recent hook errors.", run: notYet},
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
	env := &environment{stdout: stdout, stderr: stderr, deps: deps, serve: &serveFlags{}}

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
	if name == "serve" {
		flags.BoolVar(&env.serve.foreground, "foreground", false, "")
		flags.BoolVar(&env.serve.backgroundChild, "background-child", false, "")
	}
	if err := flags.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeCommandUsage(stdout, *selected)
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
		fmt.Fprintf(stderr, "flashheart: %s is not yet available\n", name)
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
	reported := false
	options.Stdout, options.Stderr = env.stdout, env.stderr
	options.Launched = func(launched app.Launched) {
		reported = true
		_ = handshake.Ready(background.Report{
			Address:      launched.Address,
			ManualURL:    launched.ManualURL,
			BrowserError: launched.BrowserError,
		})
	}
	err = env.deps.RunApp(ctx, options)
	if err != nil && !reported {
		_ = handshake.Fail(err)
	}
	return err
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
	if command.name == "serve" {
		fmt.Fprintln(output, "Options:")
		fmt.Fprintln(output, "  --foreground  keep the server attached to this terminal until it stops")
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
