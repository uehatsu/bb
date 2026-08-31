# Plan for the MAGI findings (review of v0.1.2..HEAD) — v0.1.4

Created: 2026-08-26 / Target: https://github.com/uehatsu/bb (HEAD `b275fed`, v0.1.3 released)

## 0. Findings to address

| # | Finding | Source | Priority |
|---|---|---|---|
| 1 | SKILL.md over-generalises "add `--yes` after approval". Only `pr merge` / `repo delete` / `branch delete` have `--yes`; adding it to `pr decline` / `pipeline stop` / `repo edit --visibility` fails with `unknown flag` | CASPER | Medium |
| 2 | Keeping the three SKILL.md files (shared body, different frontmatter) in sync is manual, and CI has no drift detection | BALTHASAR / CASPER | Medium |
| 3 | Regression tests lack the cases "run a group command with no arguments → help, exit 0" and "`--help` → exit 0" | BALTHASAR | Low |
| 4 | `user_invocable` / `trigger` in the Claude Code frontmatter are not keys that Claude Code interprets | CASPER | Low |
| 5 | Makefile: `cp` overwrites an existing SKILL.md without checking, `$(HOME)` is unquoted, and `install-skill` also creates `~/.codex` and `~/.copilot` for agents that are not in use | MELCHIOR | Low |
| 6 | Unknown-command output differs between root (`bb bogus`) and groups (`bb pr bogus`) — root prints no Usage | BALTHASAR | Low |
| 7 | The Go 1.26 requirement bump and past changes are missing from release notes. Three consecutive commits for the same fix | BALTHASAR | Low |

## 1. Approach and non-goals

- **No history rewriting** (the squash suggested in #7). The `v0.1.3` tag and `main` are public; a rewrite would break users' clones. Instead, introduce **CHANGELOG.md** as the source of truth for release notes from now on.
- Make SKILL.md a **generated artifact** (#2). The sources are a single body plus per-agent frontmatter. The generated files stay committed (for `make install-*` and for users who copy them straight from the repository). CI checks that "regenerating produces no diff" (the same approach as `docs/reference`).
- **Do not add** `--yes` to `pr decline` / `pipeline stop` (MELCHIOR warning 3 is acknowledged and deliberately left as-is). Rationale: both are reversible (decline → new PR, stop → rerun), and gh's `pr close` / `run cancel` have no confirmation either, so compatibility wins. SKILL.md states precisely that "approval is required but there is no flag" (#1).

## 2. Changes

### Step 1: Generating SKILL.md (#1, #2, #4)
- `skills/bitbucket.body.md` — shared body (the body of the current three files, with an `AGENT_NAME` placeholder)
- `skills/<agent>/bitbucket/frontmatter.md` — per-agent frontmatter (`claude_code` is trimmed to `name` / `description` only, dropping `user_invocable` / `trigger`; `codex` has `metadata.short-description`; `copilot` has `license`)
- `scripts/gen-skills.sh` — `frontmatter.md` + body (with `AGENT_NAME` substituted) → `skills/<agent>/bitbucket/SKILL.md`. POSIX sh only (no Go required). `AGENT_NAME` is taken from the leading comment line `<!-- agent: Claude Code -->` in frontmatter.md
- `make skills` generates; `make check-skills` does "generate → `git diff --exit-code -- skills/`". Add `make check-skills` to CI (the same job as the docs check in `ci.yml`)
- Body fixes (#1): make Ground rule 3 and the Destructive-operation workflow precise:
  - Only **three commands have `--yes`: `pr merge` / `repo delete` / `branch delete`**. In non-interactive environments `repo delete` / `branch delete` **require** `--yes`; `pr merge` may omit it (no confirmation prompt appears without a TTY)
  - `pr decline` / `pipeline stop` / `repo edit --visibility` run **without a flag after obtaining approval** (adding `--yes` fails with `unknown flag`)
  - Change step 3 of the "Destructive-operation workflow" to "run the command (add `--yes` only where the command has it)"
- Update the Agent skills section of README / README_JA to "edit `skills/bitbucket.body.md` and run `make skills`"

### Step 2: Unify root and groups + tests (#3, #6)
- `pkg/cmd/root/root.go`: give the root `Args: cobra.ArbitraryArgs` + `RunE: cmdutil.GroupRunE` as well, so `bb bogus` behaves like the groups: `unknown command "bogus" for "bb"` + Usage + exit 1. `bb` (no arguments) shows help with exit 0; `bb --version` / `bb --help` are unchanged
- Add to `pkg/cmd/root/root_test.go`:
  - `TestGroupWithoutArgsShowsHelp`: run the 8 groups + root with no arguments → `err == nil` and the output contains each command's `Short`
  - `TestGroupHelpFlag`: `bb pr --help` → `err == nil`
  - Add the root case `{"bogus"}` to `TestUnknownSubcommandFails`
- `cmd/bb/main_test.go`: `execute(ctx, f, []string{"pr", "bogus"})` → exit 1, `[]string{"pr"}` → exit 0

### Step 3: Hardening the Makefile (#5)
- Quote `"$(HOME)"` in every target
- `install-<agent>-skill`: when the existing `SKILL.md` differs from the repository version, back it up to `SKILL.md.bak` before overwriting and say so (detected with `cmp -s`)
- `install-skill` keeps installing all three (README notes that "directories for unused agents are created too; use the individual targets to avoid that")

### Step 4: CHANGELOG (#7)
- Add `CHANGELOG.md` (Keep a Changelog format). Backfill `0.1.0`–`0.1.3` (0.1.1: PR URL / `branch delete --yes` / store migration; 0.1.2: pipeline number resolution; 0.1.3: unknown subcommands exit 1, and the note that `go install` now needs Go 1.26). Put this plan's contents under `Unreleased`
- Keep the existing commit-based `changelog` in `.goreleaser.yml` (CHANGELOG.md is the human-facing source of truth). Add "update CHANGELOG.md before a release" to the Development section of the README

### Step 5: Verification and release
- Confirm **locally, before committing**, that `go test ./...`, `golangci-lint run`, `make check-skills`, and `make docs` produce no diff
- Run `make install-skill` to update all three locations and confirm that `bitbucket` still appears in Claude Code's skill list (verifies the frontmatter change from #4)
- One commit per step (4 commits). Tag `v0.1.4` exactly once, after all CI jobs are green

## 3. Completion criteria
- SKILL.md no longer induces misuse such as `bb pr decline --yes` (the body names the commands that support it)
- `make check-skills` runs in CI, and editing any of the three files individually makes CI fail
- `bb`, `bb <group>`, `bb <group> --help` → exit 0 / `bb bogus`, `bb <group> bogus` → exit 1 are pinned by tests
- The Makefile never loses existing local edits
- CHANGELOG.md holds the history for 0.1.0–0.1.4

## 4. Risks
- Giving the root a `RunE` might change cobra's `--version` handling order → pin the output of `bb --version` in a test
- Removing `user_invocable` / `trigger` from the frontmatter might make `/bitbucket` disappear → verify in Step 5 and, if it disappears, restore them as evidence that `name` alone is insufficient (Claude Code's official keys are `name` / `description` / `allowed-tools` / `disable-model-invocation`)

## 5. Results (2026-08-26)
- Plan review: all three APPROVE in the first round (MELCHIOR 8 / BALTHASAR 8 / CASPER 8). The WARNINGS (pass AGENT_NAME as an argument, detect untracked files with `git status --porcelain`, "Did you mean" suggestions, note the single `.bak` generation, check the CHANGELOG for the tag) were folded into the implementation.
- Commits: `9f15704` (Step 1), `88ef5e7` (Step 2), `55df9c5` (Step 3), `a2784f4` (Step 4). All CI jobs succeeded.
- Confirmed that `bitbucket` still shows up in Claude Code's skill list after removing `user_invocable` / `trigger` (Claude Code as of 2026-08).
- Released as v0.1.4.
