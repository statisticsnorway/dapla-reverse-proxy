.PHONY: build test fmt staticcheck vulncheck

build:
	go build -o build/reverse-proxy ./cmd/

test:
	go test -cover --race ./...

fmt:
	go tool mvdan.cc/gofumpt -w ./

staticcheck:
	go tool honnef.co/go/tools/cmd/staticcheck ./...

vulncheck:
	go tool golang.org/x/vuln/cmd/govulncheck ./...