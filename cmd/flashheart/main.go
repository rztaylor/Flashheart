package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/rztaylor/flashheart/internal/app"
	"github.com/rztaylor/flashheart/internal/background"
	"github.com/rztaylor/flashheart/internal/buildinfo"
	"github.com/rztaylor/flashheart/internal/cli"
	"github.com/rztaylor/flashheart/internal/logfile"
	"github.com/rztaylor/flashheart/internal/setup"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	exitCode := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, cli.Dependencies{
		Build:           buildinfo.Current(),
		Getenv:          os.Getenv,
		HomeDir:         os.UserHomeDir,
		Executable:      os.Executable,
		RunApp:          runApp,
		StartBackground: background.Start,
		OpenServeLog:    func(root string) io.Writer { return logfile.ServeLog(root) },
		Stdin:           os.Stdin,
		Interactive:     func() bool { return cli.IsTerminal(os.Stdin) },
		Getwd:           os.Getwd,
		FindCLI:         func(agent setup.Agent, home string) string { return agent.FindCLI(home) },
		OpenHandshake: func() (cli.Handshake, error) {
			return background.OpenHandshake()
		},
	})
	os.Exit(exitCode)
}

// runApp serves the board with cli's options. The adaptation lives here so
// that cli, and with it the hook and mcp modes, never links app (FH-169).
func runApp(ctx context.Context, options cli.ServeOptions) error {
	return app.Run(ctx, app.Options{
		Build:  options.Build,
		Root:   options.Root,
		Debug:  options.Debug,
		Stdout: options.Stdout,
		Stderr: options.Stderr,
		Launched: func(launched app.Launched) {
			if options.Launched != nil {
				options.Launched(cli.Launched(launched))
			}
		},
	})
}
