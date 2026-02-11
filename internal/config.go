package internal

import (
	"context"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/sethvargo/go-envconfig"
)

type config struct {
	ListenAddr                    string        `env:"LISTEN_ADDR,default=:8080"`
	UpstreamURL                   *url.URL      `env:"UPSTREAM_URL,required"`
	AllowedIPs                    []string      `env:"ALLOWED_IPS,required"`
	ClientIPHeader                string        `env:"CLIENT_IP_HEADER,default=X-Forwarded-For"`
	ReadTimeout                   time.Duration `env:"SERVER_READ_TIMEOUT,default=15s"`
	ReadHeaderTimeout             time.Duration `env:"SERVER_READ_HEADER_TIMEOUT,default=5s"`
	WriteTimeout                  time.Duration `env:"SERVER_WRITE_TIMEOUT,default=30s"`
	IdleTimeout                   time.Duration `env:"SERVER_IDLE_TIMEOUT,default=120s"`
	ShutdownTimeout               time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT,default=10s"`
	UpstreamResponseHeaderTimeout time.Duration `env:"UPSTREAM_RESPONSE_HEADER_TIMEOUT,default=15s"`
}

func loadConfigFromEnv() (config, error) {
	return loadConfig(envconfig.OsLookuper())
}

func loadConfig(lookuper envconfig.Lookuper) (config, error) {
	var cfg config
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target:   &cfg,
		Lookuper: lookuper,
	}); err != nil {
		return config{}, fmt.Errorf("failed to process environment configuration: %w", err)
	}

	err := validateUpstreamUrl(cfg.UpstreamURL)
	if err != nil {
		return config{}, err
	}

	allowedIPs, err := normalizeAllowedIPs(cfg.AllowedIPs)
	if err != nil {
		return config{}, err
	}
	cfg.AllowedIPs = allowedIPs

	return cfg, nil
}

func validateUpstreamUrl(url *url.URL) error {
	if url == nil {
		return fmt.Errorf("UPSTREAM_URL must be set")
	}

	if url.Scheme != "http" && url.Scheme != "https" {
		return fmt.Errorf("UPSTREAM_URL scheme must be http or https")
	}
	if url.Host == "" {
		return fmt.Errorf("UPSTREAM_URL must include host")
	}

	return nil
}

func normalizeAllowedIPs(values []string) ([]string, error) {
	allowed := make([]string, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		if strings.Contains(trimmed, "/") {
			return nil, fmt.Errorf("ALLOWED_IPS must contain only specific IPs (CIDR not supported): %q", trimmed)
		}

		addr, err := netip.ParseAddr(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid ip in ALLOWED_IPS (%q): %w", trimmed, err)
		}
		allowed = append(allowed, addr.String())
	}

	if len(allowed) == 0 {
		return nil, fmt.Errorf("ALLOWED_IPS must contain at least one valid IP address")
	}

	return allowed, nil
}
