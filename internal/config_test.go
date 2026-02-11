package internal

import (
	"strings"
	"testing"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
)

func TestNormalizeAllowedIPs(t *testing.T) {
	result, err := normalizeAllowedIPs([]string{"192.0.1.11", "2001:aaa::1"})
	if err != nil {
		t.Fatalf("normalizeAllowedIPs returned error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
}

func TestNormalizeAllowedIPsRejectsCIDR(t *testing.T) {
	_, err := normalizeAllowedIPs([]string{"10.0.0.0/24"})
	if err == nil {
		t.Fatal("expected error for CIDR in ALLOWED_IPS")
	}
	if !strings.Contains(err.Error(), "CIDR") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeAllowedIPsRejectsEmptyList(t *testing.T) {
	_, err := normalizeAllowedIPs([]string{"  ", ""})
	if err == nil {
		t.Fatal("expected error for empty ALLOWED_IPS")
	}
	if !strings.Contains(err.Error(), "at least one valid IP") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	cfg, err := loadConfig(envconfig.MapLookuper(map[string]string{
		"UPSTREAM_URL": "https://internal-ingress.example.com",
		"ALLOWED_IPS":  "203.0.113.10",
	}))
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if cfg.ListenAddr != ":8080" {
		t.Fatalf("unexpected listen address: %s", cfg.ListenAddr)
	}
	if cfg.ReadTimeout != 15*time.Second {
		t.Fatalf("unexpected read timeout: %s", cfg.ReadTimeout)
	}
	if cfg.ClientIPHeader != "X-Forwarded-For" {
		t.Fatalf("unexpected client ip header: %s", cfg.ClientIPHeader)
	}
}
