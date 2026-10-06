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
		RunApp:          app.Run,
		StartBackground: background.Start,
		OpenServeLog:    func(root string) io.Writer { return logfile.ServeLog(root) },
		Stdin:           os.Stdin,
		Getwd:           os.Getwd,
		FindClaude:      setup.FindClaude,
		OpenHandshake: func() (cli.Handshake, error) {
			return background.OpenHandshake()
		},
	})
	os.Exit(exitCode)
}
