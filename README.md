# dapla-reverse-proxy

A reverse proxy in go. Designed to be exposed on public ingress and route to upstream to internal application (dapla
api).

## Features

- Allows only traffic from configured IPs. For non-allowed ips `403 Forbidden` will be returned
- Health checks endpoint (/healthz`) on a dedicated listener
- All other endpoints are forwarded to the upstream host.

## PS:

- Set `CLIENT_IP_HEADER` to a header your ingress controls (for example `X-Forwarded-For` or `X-Real-IP`).
- `ALLOWED_IPS` supports IPv4/IPv6 addresses. Only specific adresses. CIDRs are rejected.
- The proxy only handles HTTP over TCP  (see [ReverseProxy docs](https://pkg.go.dev/net/http/httputil#ReverseProxy))

## Configuration

| Variable                           | Required | Default           | Description                                          |
|------------------------------------|----------|-------------------|------------------------------------------------------|
| `UPSTREAM_URL`                     | Yes      | -                 | Upstream URL, e.g. `https://ssb.no`                  |
| `ALLOWED_IPS`                      | Yes      | -                 | Comma-separated IPs, e.g. `203.0.0.10,198.51.100.20` |
| `CLIENT_IP_HEADER`                 | No       | `X-Forwarded-For` | Header used for IP allowlist checks                  |
| `LISTEN_ADDR`                      | No       | `:8080`           | Address the proxy listens on                         |
| `HEALTH_LISTEN_ADDR`               | No       | `:8081`           | Address where `GET/HEAD /healthz` is served          |
| `SERVER_READ_TIMEOUT`              | No       | `15s`             | Maximum total request read time                      |
| `SERVER_READ_HEADER_TIMEOUT`       | No       | `5s`              | Maximum time to read request headers                 |
| `SERVER_WRITE_TIMEOUT`             | No       | `30s`             | Maximum time to write response                       |
| `SERVER_IDLE_TIMEOUT`              | No       | `120s`            | Keep-alive connection idle timeout                   |
| `SERVER_SHUTDOWN_TIMEOUT`          | No       | `10s`             | Graceful shutdown deadline                           |
| `UPSTREAM_RESPONSE_HEADER_TIMEOUT` | No       | `15s`             | Max wait for upstream response headers               |

## Run

```bash
export UPSTREAM_URL="https://ssb.no"
export ALLOWED_IPS="127.0.0.1"
export CLIENT_IP_HEADER="X-Forwarded-For"

go run ./cmd
```

## Build

```bash
make build
```

## Test

```bash
make test
```

## Format code

```bash
make fmt
```