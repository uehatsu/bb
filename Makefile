BIN     := bin/bb
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/uehatsu/bb/internal/build.Version=$(VERSION)

.PHONY: build test lint vet fmt clean docs skills check-skills install-skill install-claude-code-skill install-codex-skill install-copilot-skill

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

# The three SKILL.md files are generated from skills/bitbucket.body.md plus
# skills/<agent>/bitbucket/frontmatter.md. Edit those sources, then run
# `make skills`; CI runs `make check-skills` to reject stale generated files.
skills:
	scripts/gen-skills.sh

check-skills:
	scripts/gen-skills.sh --check

# Install the agent skills (Claude Code, Codex, GitHub Copilot) so `/bitbucket`
# is available in every project. Use the specific targets to install one of them.
install-skill: install-claude-code-skill install-codex-skill install-copilot-skill

install-claude-code-skill:
	mkdir -p $(HOME)/.claude/skills/bitbucket
	cp skills/claude_code/bitbucket/SKILL.md $(HOME)/.claude/skills/bitbucket/SKILL.md

install-codex-skill:
	mkdir -p $(HOME)/.codex/skills/bitbucket
	cp skills/codex/bitbucket/SKILL.md $(HOME)/.codex/skills/bitbucket/SKILL.md

install-copilot-skill:
	mkdir -p $(HOME)/.copilot/skills/bitbucket
	cp skills/copilot/bitbucket/SKILL.md $(HOME)/.copilot/skills/bitbucket/SKILL.md
