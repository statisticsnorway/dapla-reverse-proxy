package internal

import (
	"net/url"
	"strings"
	"testing"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
)

func TestValidUpstreamUrl(t *testing.T) {
	parseUrl := func(rawUrl string) *url.URL {
		parsed, _ := url.Parse(rawUrl)
		return parsed
	}
	tests := map[string]struct {
		input              *url.URL
		expectedErrMessage string
	}{
		"url is required": {
			input:              nil,
			expectedErrMessage: "UPSTREAM_URL must be set",
		},
		"http is allowed": {
			input:              parseUrl("http://ssb.no"),
			expectedErrMessage: "",
		},
		"https is allowed": {
			input:              parseUrl("https://ssb.no"),
			expectedErrMessage: "",
		},
		"schema is required": {
			input:              parseUrl("missing-schema.no"),
			expectedErrMessage: "UPSTREAM_URL scheme must be http or https",
		},
		"host is required": {
			input:              parseUrl("http://"),
			expectedErrMessage: "UPSTREAM_URL must include host",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateUpstreamUrl(test.input)
			if err == nil {
				if test.expectedErrMessage != "" {
					t.Fatalf("expected error but got nil: %s", test.expectedErrMessage)
				}
			}
			if err != nil {
				if test.expectedErrMessage == "" {
					t.Fatalf("did not expect error, but got %s", err.Error())
				}
				if err.Error() != test.expectedErrMessage {
					t.Fatalf("expected error message %s, but got %s", err.Error(), test.expectedErrMessage)
				}

			}
		})
	}
}

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
		"UPSTREAM_URL": "https://ssb.no",
		"ALLOWED_IPS":  "203.0.113.10",
	}))
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if cfg.ListenAddr != ":8080" {
		t.Fatalf("unexpected listen address: %s", cfg.ListenAddr)
	}
	if cfg.HealthListenAddr != ":8081" {
		t.Fatalf("unexpected health listen address: %s", cfg.HealthListenAddr)
	}
	if cfg.ReadTimeout != 15*time.Second {
		t.Fatalf("unexpected read timeout: %s", cfg.ReadTimeout)
	}
	if cfg.ClientIPHeader != "X-Forwarded-For" {
		t.Fatalf("unexpected client ip header: %s", cfg.ClientIPHeader)
	}
}
