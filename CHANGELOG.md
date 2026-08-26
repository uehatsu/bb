# Changelog

All notable changes to `bb` are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed
- `bb help <unknown>` is a usage error (exit 1) like unknown commands.
- Unknown or malformed flags print the usage to stderr (exit 1), matching
  unknown commands.
- `make check-docs` reports a failing generator separately from stale docs;
  `scripts/gen-skills.sh` validates the shared description and rejects
  unknown arguments.
- Internal: pull request commands resolve the repository inside `resolvePR`
  (no separate `baseRepoFor` call at each site).

### Fixed
- "Did you mean" suggestions also cover misspellings within a Levenshtein
  distance of 2 (`bb pr mrege` -> `merge`), not only prefixes, and an empty
  argument no longer lists every subcommand.
- An unknown root command is reported before flags are parsed again, as in
  0.1.3: `bb bogus --help` / `bb bogus --version` exit 1 instead of printing
  help or the version, and `bb rpeo list -R ws/repo` says `unknown command
  "rpeo"` (with a suggestion) instead of `unknown shorthand flag`.
- Usage errors print the usage to stderr; stdout stays empty (`bb bogus
  2>/dev/null` printed 29 lines of usage).
- Pull request commands given a URL (`bb pr view https://bitbucket.org/ws/repo/pull-requests/42`)
  no longer need a Bitbucket checkout or `-R`: the repository comes from the URL.
- `bb help [command]` works (it was rejected as an unknown command); the
  `help` command stays out of the command list.
- `make check-skills` compares freshly generated files with the checked-in
  ones instead of rewriting the working tree (which silently discarded
  uncommitted edits and then passed); `make check-docs` does the same for
  `docs/reference` and also catches leftover pages (the stale
  `bb_pr_reopen.md` is removed).
- `make install-*-skill` stops when the `SKILL.md.bak` backup cannot be
  written instead of overwriting the edited file anyway.
- Agent skills: `-R` exists only on the commands that act on one repository;
  `--body-file` is limited to `pr create/edit/review/comment`; `repo view`
  needs a repository argument to list its `--json` fields outside a checkout;
  `pr view --comments` and the step list of `pipeline view` are text-only
  (the `bb api` routes for JSON are given); the token scopes needed by
  `repo create/edit/delete` and `project create` are listed.

### Changed
- The skill descriptions are generated from one shared line
  (`skills/bitbucket.description.txt`); `AGENT_NAME` and `SKILL_DESCRIPTION`
  are substituted in the front matter as well as the body.
- The release workflow checks the changelog before installing Go and running
  the tests.

## [0.1.4] - 2026-08-26

### Added
- Agent skills for Codex and GitHub Copilot (`skills/codex/`, `skills/copilot/`)
  with `make install-codex-skill` / `make install-copilot-skill`;
  `make install-skill` installs all three.
- Unknown subcommands print a "Did you mean" suggestion.
- This changelog.

### Changed
- The Claude Code skill moved from `skills/bitbucket/SKILL.md` to
  `skills/claude_code/bitbucket/SKILL.md` and is written in English.
- The three agent `SKILL.md` files are generated from one shared body
  (`make skills`); CI rejects stale generated files.
- The skills state precisely which commands take `--yes` (`pr merge`,
  `repo delete`, `branch delete`) and that `pr decline` / `pipeline stop`
  have no such flag.
- `make install-*-skill` keeps one `SKILL.md.bak` of a locally edited copy
  instead of overwriting it silently.

### Fixed
- `bb bogus` behaves like `bb pr bogus` (usage error, exit 1) instead of
  printing the error without usage.
- README: `go install` needs Go 1.26 (the module's `go` directive since
  0.1.0), not 1.22.

## [0.1.3] - 2026-08-26

### Fixed
- Unknown subcommands of a command group (`bb pr nonsense`) failed silently
  with exit 0 after printing help; they now exit 1 with an error.

## [0.1.2] - 2026-08-26

### Fixed
- `bb pipeline view/watch/log/stop <build-number>` always resolved to the
  oldest pipeline (#1) because the Bitbucket API ignores `q=build_number`.
  Build numbers are now resolved from the newest-first listing.

## [0.1.1] - 2026-08-25

### Fixed
- Pull request URLs target the repository named in the URL instead of the
  current repository (wrong-target hazard).
- `bb branch delete` requires `--yes` when not running interactively, like
  `bb repo delete`.
- `bb config set credential_store` migrates the stored credential safely
  (copy, persist config, verify, then delete the old copy) and rolls back on
  failure.

## [0.1.0] - 2026-08-25

Initial release: `auth` (API tokens, access tokens, OAuth), `repo`, `pr`,
`pipeline`, `branch`, `workspace`, `project`, `config`, `api`, `browse`, with
`--json`/`--jq`/`--template` output and a git credential helper.

[Unreleased]: https://github.com/uehatsu/bb/compare/v0.1.4...HEAD
[0.1.4]: https://github.com/uehatsu/bb/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/uehatsu/bb/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/uehatsu/bb/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/uehatsu/bb/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/uehatsu/bb/releases/tag/v0.1.0
