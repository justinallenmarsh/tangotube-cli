// Command tt is the TangoTube CLI.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"

	"github.com/justinallenmarsh/tangotube-cli/internal/commands"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx))
}

func run(ctx context.Context) int {
	app := commands.NewApp()
	root := commands.NewRoot(app)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return 0
	}
	var exit *commands.ExitError
	if !errors.As(err, &exit) {
		// Flag and argument problems cobra found before any command ran.
		errors.As(app.Fail(err), &exit)
	}
	return exit.Code
}
