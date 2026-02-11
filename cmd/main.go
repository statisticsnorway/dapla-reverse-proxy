package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"dapla-reverse-proxy/internal"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := internal.Run(ctx, log); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("proxy exited with error", "error", err)
		os.Exit(1)
	}

	log.Info("proxy stopped")
}
