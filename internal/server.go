package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
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

	proxyServer := newHTTPServer(cfg.ListenAddr, app.proxyRoutes(), cfg, log)
	healthServer := newHTTPServer(cfg.HealthListenAddr, app.healthRoutes(), cfg, log)

	errCh := make(chan error, 1)
	go func() {
		errCh <- proxyServer.ListenAndServe()
	}()
	go func() {
		_ = healthServer.ListenAndServe()
	}()

	log.Info(
		"proxy started",
		"listen_addr", cfg.ListenAddr,
		"health_listen_addr", cfg.HealthListenAddr,
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

		if err := shutdownServer(context.Background(), cfg.ShutdownTimeout, proxyServer); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		_ = shutdownServer(context.Background(), cfg.ShutdownTimeout, healthServer)

		err := <-errCh
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf("shutdown returned error: %w", err)
	}
}

func newHTTPServer(addr string, handler http.Handler, cfg config, log *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}
}

func shutdownServer(parent context.Context, timeout time.Duration, server *http.Server) error {
	shutdownCtx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
	return nil
}
