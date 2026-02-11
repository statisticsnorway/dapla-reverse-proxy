package internal

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"sync/atomic"
	"testing"
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

func TestIPAllowed(t *testing.T) {
	allowedIPs := map[string]struct{}{
		"10.10.2.2":    {},
		"192.168.1.10": {},
	}

	allowedIP := netip.MustParseAddr("10.10.2.2")
	if !ipAllowed(allowedIP, allowedIPs) {
		t.Fatalf("expected %s to be allowed", allowedIP)
	}

	deniedIP := netip.MustParseAddr("172.16.1.1")
	if ipAllowed(deniedIP, allowedIPs) {
		t.Fatalf("expected %s to be denied", deniedIP)
	}
}

func TestClientIPFromRequest(t *testing.T) {
	tests := map[string]struct {
		headerName    string
		headerValue   string
		expectedIP    string
		expectErr     bool
		expectValidIP bool
	}{
		"uses configured header": {
			headerName:    "X-Forwarded-For",
			headerValue:   "198.51.100.10, 10.0.0.1",
			expectedIP:    "198.51.100.10",
			expectErr:     false,
			expectValidIP: true,
		},
		"header with port": {
			headerName:    "X-Real-IP",
			headerValue:   "198.51.100.10:443",
			expectedIP:    "198.51.100.10",
			expectErr:     false,
			expectValidIP: true,
		},
		"returns error when header has no valid ip": {
			headerName:    "X-Forwarded-For",
			headerValue:   "not-an-ip",
			expectErr:     true,
			expectValidIP: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "http://ssb.no", nil)
			if err != nil {
				t.Fatalf("unable to build request: %v", err)
			}
			req.Header.Set(test.headerName, test.headerValue)

			ip, err := clientIPFromRequest(req, test.headerName)
			if test.expectErr {
				if err == nil {
					t.Fatalf("expected error, got IP %s", ip)
				}
			} else {
				if err != nil {
					t.Fatalf("clientIPFromRequest returned error: %v", err)
				}
				if ip.String() != test.expectedIP {
					t.Fatalf("unexpected ip: %s", ip)
				}
			}

			if test.expectValidIP {
				if !ip.IsValid() {
					t.Fatalf("unexpected IP validity: got %t for %s", ip.IsValid(), ip)
				}
			}

		})
	}
}

func TestRoutes_HealthzIsNotProxiedToUpstream(t *testing.T) {
	var upstreamHits int64
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&upstreamHits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstreamServer.Close()

	upstreamURL, err := url.Parse(upstreamServer.URL)
	if err != nil {
		t.Fatalf("unable to parse upstream URL: %v", err)
	}

	cfg := config{
		AllowedIPs:     []string{"198.51.100.10"},
		ClientIPHeader: "X-Forwarded-For",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := newProxyHandler(cfg, upstreamURL, logger).routes()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "GET healthz", method: http.MethodGet, path: "/healthz"},
		{name: "HEAD healthz", method: http.MethodHead, path: "/healthz"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
			}
		})
	}

	if hits := atomic.LoadInt64(&upstreamHits); hits != 0 {
		t.Fatalf("expected no upstream hits for /healthz, got %d", hits)
	}
}

func TestRoutes_ProxiesNonHealthzPaths(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
	}{
		{name: "root get path", method: http.MethodGet, target: "/"},
		{name: "nested get path", method: http.MethodGet, target: "/foo/bar"},
		{name: "post path with query", method: http.MethodPost, target: "/api/v1/items?limit=10"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var upstreamHits int64
			var resultMethod string
			var resultPath string
			var resultQuery string

			upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt64(&upstreamHits, 1)
				resultMethod = r.Method
				resultPath = r.URL.Path
				resultQuery = r.URL.RawQuery
				w.WriteHeader(http.StatusOK)
			}))
			defer upstreamServer.Close()

			upstreamURL, err := url.Parse(upstreamServer.URL)
			if err != nil {
				t.Fatalf("unable to parse upstream URL: %v", err)
			}

			cfg := config{
				AllowedIPs:     []string{"198.51.100.10"},
				ClientIPHeader: "X-Forwarded-For",
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler := newProxyHandler(cfg, upstreamURL, logger).routes()

			req := httptest.NewRequest(test.method, test.target, nil)
			req.Header.Set("X-Forwarded-For", "198.51.100.10")
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
			}
			if hits := atomic.LoadInt64(&upstreamHits); hits != 1 {
				t.Fatalf("expected one upstream hit, got %d", hits)
			}

			if resultMethod != test.method {
				t.Fatalf("expected upstream method %s, got %s", test.method, resultMethod)
			}

			expectedURL, err := url.Parse(test.target)
			if err != nil {
				t.Fatalf("unable to parse test target %q: %v", test.target, err)
			}

			if resultPath != expectedURL.Path {
				t.Fatalf("expected upstream path %s, got %s", expectedURL.Path, resultPath)
			}
			if resultQuery != expectedURL.RawQuery {
				t.Fatalf("expected upstream query %s, got %s", expectedURL.RawQuery, resultQuery)
			}
		})
	}
}
