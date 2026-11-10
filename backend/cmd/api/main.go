package main

import (
	"context"
	"log/slog"
	"mini-messenger/internal/adapter/observability"
	"mini-messenger/internal/bootstrap"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	config, err := bootstrap.LoadConfig()
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err = bootstrap.Run(ctx, config); err != nil {
		observability.Report(ctx, "server stopped", err)
		os.Exit(1)
	}
}
