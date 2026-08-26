BIN     := bin/bb
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/uehatsu/bb/internal/build.Version=$(VERSION)

.PHONY: build test lint vet fmt clean docs install-skill install-claude-code-skill install-codex-skill

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
install-skill: install-claude-code-skill

install-claude-code-skill:
	mkdir -p $(HOME)/.claude/skills/bitbucket
	cp skills/claude_code/bitbucket/SKILL.md $(HOME)/.claude/skills/bitbucket/SKILL.md

install-codex-skill:
	mkdir -p $(HOME)/.codex/skills/bitbucket
	cp skills/codex/bitbucket/SKILL.md $(HOME)/.codex/skills/bitbucket/SKILL.md
