package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/loaiza000/eject/internal/cli"
)

// version is set from an approved release build; source builds report dev.
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := cli.New(version).ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "eject:", err)
		var exitErr *cli.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}
}
