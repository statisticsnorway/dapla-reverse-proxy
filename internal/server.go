package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

func Run(ctx context.Context, log *slog.Logger) error {
	cfg, err := loadConfigFromEnv()
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	return run(ctx, cfg, log)
}

func run(ctx context.Context, cfg config, log *slog.Logger) error {
	app := newProxyHandler(cfg, cfg.UpstreamURL, log)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.routes(),
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	log.Info(
		"proxy started",
		"listen_addr", cfg.ListenAddr,
		"upstream_url", cfg.UpstreamURL,
		"client_ip_header", cfg.ClientIPHeader,
	)

	select {
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("listen failed: %w", err)

	case <-ctx.Done():
		log.Info("shutdown signal received")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}

		err := <-errCh
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf("server shutdown returned error: %w", err)
	}
}
