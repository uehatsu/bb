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

# install_skill <source> <dest dir>: keeps one .bak of a locally edited copy.
define install_skill
	mkdir -p "$(2)"
	@if [ -f "$(2)/SKILL.md" ] && ! cmp -s "$(1)" "$(2)/SKILL.md"; then \
		cp "$(2)/SKILL.md" "$(2)/SKILL.md.bak"; \
		echo "note: existing $(2)/SKILL.md differed; previous copy saved as SKILL.md.bak"; \
	fi
	cp "$(1)" "$(2)/SKILL.md"
endef

install-claude-code-skill:
	$(call install_skill,skills/claude_code/bitbucket/SKILL.md,$(HOME)/.claude/skills/bitbucket)

install-codex-skill:
	$(call install_skill,skills/codex/bitbucket/SKILL.md,$(HOME)/.codex/skills/bitbucket)

install-copilot-skill:
	$(call install_skill,skills/copilot/bitbucket/SKILL.md,$(HOME)/.copilot/skills/bitbucket)
