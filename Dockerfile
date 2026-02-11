ARG GO_VERSION=""
FROM golang:${GO_VERSION}alpine AS builder
WORKDIR /src
COPY go.* /src/
COPY . /src
RUN go build -o bin/reverse-proxy ./cmd/

FROM gcr.io/distroless/base
WORKDIR /app
COPY --from=builder /src/bin/reverse-proxy /app/reverse-proxy
ENTRYPOINT ["/app/reverse-proxy"]
