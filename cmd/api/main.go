package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"sleet0922/graduation_project/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.Bootstrap(ctx, app.Options{})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "application bootstrap failed: %v\n", err)
		os.Exit(1)
	}
	if err := application.Run(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "application stopped with error: %v\n", err)
		os.Exit(1)
	}
}
