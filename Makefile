BIN     := bin/bb
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/uehatsu/bb/internal/build.Version=$(VERSION)

.PHONY: build test lint vet fmt clean docs install-skill

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/bb

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

lint:
	golangci-lint run ./...

clean:
	rm -rf bin dist

docs:
	go run ./cmd/gendocs docs/reference

# Install the Claude Code skill so `/bitbucket` is available in every project.
install-skill:
	mkdir -p $(HOME)/.claude/skills/bitbucket
	cp skills/bitbucket/SKILL.md $(HOME)/.claude/skills/bitbucket/SKILL.md
