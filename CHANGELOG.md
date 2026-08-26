# Changelog

All notable changes to `bb` are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.4] - 2026-08-26

### Changed
- Unknown subcommands now print a "Did you mean" suggestion, and `bb bogus`
  behaves like `bb pr bogus` (usage error, exit 1).
- The three agent `SKILL.md` files are generated from one shared body
  (`make skills`); CI rejects stale generated files.
- The skills state precisely which commands take `--yes` (`pr merge`,
  `repo delete`, `branch delete`) and that `pr decline` / `pipeline stop`
  have no such flag.
- `make install-*-skill` keeps one `SKILL.md.bak` of a locally edited copy
  instead of overwriting it silently.

### Added
- This changelog.

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

### Changed
- `go install` requires Go 1.26 (the module's `go` directive).

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
