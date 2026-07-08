.PHONY: run build test

run:
	go run ./cmd/nxsandbox

build:
	go build -ldflags="-s -w -X main.Version=$$(git describe --tags --always 2>NUL || echo dev)" -o nxsandbox ./cmd/nxsandbox

test:
	go test ./...
