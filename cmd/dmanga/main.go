package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"dmanga/internal/cli"
	"dmanga/internal/fetcher"
	_ "dmanga/internal/parser"
)

func main() {
	// Cancel in-flight requests and retry waits promptly on Ctrl+C / termination.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	fetcher.SetContext(ctx)

	cli.Run(os.Args)

	interrupted := ctx.Err() != nil
	stop()
	if interrupted {
		os.Exit(130)
	}
}
