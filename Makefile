.DEFAULT_GOAL := all
.PHONY := run build clean test deps

all: run

run:
	go run ./cmd/dot

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/linux/dot ./cmd/dot
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/darwin/dot ./cmd/dot
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/windows/dot ./cmd/dot

test:
	go test ./...

deps:
	go mod tidy
