package internal

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"runtime/debug"
	"strings"
	"time"
)

type proxyHandler struct {
	cfg          config
	logger       *slog.Logger
	proxy        *httputil.ReverseProxy
	allowedIPSet map[string]struct{}
}

func newProxyHandler(cfg config, upstream *url.URL, log *slog.Logger) *proxyHandler {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 200
	transport.MaxIdleConnsPerHost = 64
	transport.IdleConnTimeout = 90 * time.Second
	transport.ResponseHeaderTimeout = cfg.UpstreamResponseHeaderTimeout
	transport.ExpectContinueTimeout = 1 * time.Second
	transport.ForceAttemptHTTP2 = true

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			pr.SetXForwarded()
			pr.Out.Host = upstream.Host
		},
		Transport: transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Error(
				"upstream request failed",
				"error", err,
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, "bad gateway", http.StatusBadGateway)
		},
	}

	allowedIPSet := make(map[string]struct{}, len(cfg.AllowedIPs))
	for _, ip := range cfg.AllowedIPs {
		allowedIPSet[ip] = struct{}{}
	}

	return &proxyHandler{
		cfg:          cfg,
		logger:       log,
		proxy:        proxy,
		allowedIPSet: allowedIPSet,
	}
}

func (a *proxyHandler) proxyRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(a.handleProxy))

	return a.recoveryMiddleware(mux)
}

func (a *proxyHandler) healthRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.handleHealthz)
	mux.HandleFunc("HEAD /healthz", a.handleHealthz)

	return a.recoveryMiddleware(mux)
}

func (a *proxyHandler) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (a *proxyHandler) handleProxy(w http.ResponseWriter, r *http.Request) {
	clientIP, err := clientIPFromRequest(r, a.cfg.ClientIPHeader)
	if err != nil {
		a.logger.Warn(
			"unable to determine client IP",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path,
		)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if !ipAllowed(clientIP, a.allowedIPSet) {
		a.logger.Warn(
			"request denied by IP allowlist",
			"client_ip", clientIP.String(),
			"method", r.Method,
			"path", r.URL.Path,
		)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	a.proxy.ServeHTTP(w, r)
}

func ipAllowed(ip netip.Addr, allowlist map[string]struct{}) bool {
	ipString := ip.String()
	_, ok := allowlist[ipString]
	return ok
}

func clientIPFromRequest(r *http.Request, header string) (netip.Addr, error) {
	rawHeaderValue := r.Header.Get(header)
	if rawHeaderValue != "" {
		for item := range strings.SplitSeq(rawHeaderValue, ",") {
			if ip, ok := parseIPCandidate(item); ok {
				return ip, nil
			}
		}
	}

	return netip.Addr{}, fmt.Errorf("no valid client IP found in %s", header)
}

func parseIPCandidate(raw string) (netip.Addr, bool) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return netip.Addr{}, false
	}

	if ip, err := netip.ParseAddr(candidate); err == nil {
		return ip, true
	}

	host, _, err := net.SplitHostPort(candidate)
	if err == nil {
		if ip, err := netip.ParseAddr(strings.TrimSpace(host)); err == nil {
			return ip, true
		}
	}

	return netip.Addr{}, false
}

// We want to log if the http panics
func (a *proxyHandler) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.logger.Error(
					"panic recovered",
					"panic", recovered,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
