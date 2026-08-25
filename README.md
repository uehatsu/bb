# bb — Bitbucket Cloud CLI

`bb` brings the ergonomics of [GitHub CLI](https://cli.github.com/) to
[Bitbucket Cloud](https://bitbucket.org). Written in Go, single binary,
scriptable with `--json`/`--jq`/`--template`.

```
$ bb auth login
$ bb repo clone acme/widgets
$ bb pr create --title "Fix login" --fill
$ bb pr checkout 42
$ bb pr merge 42 --squash --delete-branch
$ bb pipeline watch 128
```

## Installation

```sh
# Homebrew (macOS / Linux)
brew install uehatsu/tap/bb

# From source (Go 1.22+)
go install github.com/uehatsu/bb/cmd/bb@latest
```

Pre-built binaries for macOS, Linux, and Windows are attached to each
[release](https://github.com/uehatsu/bb/releases).

## Authentication

Bitbucket Cloud retired **app passwords** in July 2026. `bb` authenticates with
**Atlassian API tokens with scopes**:

1. Open <https://id.atlassian.com/manage-profile/security/api-tokens>
2. Choose **Create API token with scopes**, select the **Bitbucket** app, and
   grant the scopes below. Set an expiry (required by Atlassian).
3. Run `bb auth login` and paste the token together with your Atlassian
   account email.

| Scope | Needed for |
|---|---|
| `read:user:bitbucket` | `bb auth status`, `@me` filters |
| `read:workspace:bitbucket` | listing workspaces / members (reviewer lookup) |
| `read:project:bitbucket` / `admin:project:bitbucket` | `bb project` |
| `read:repository:bitbucket` / `write:repository:bitbucket` | repos, branches, source, clone |
| `admin:repository:bitbucket` / `delete:repository:bitbucket` | `bb repo create/edit/delete` |
| `read:pullrequest:bitbucket` / `write:pullrequest:bitbucket` | `bb pr *` |
| `read:pipeline:bitbucket` / `write:pipeline:bitbucket` | `bb pipeline *` |

Non-interactive / CI usage:

```sh
echo "you@example.com:ATATT3x..." | bb auth login --with-token
# or
export BB_EMAIL=you@example.com BB_TOKEN=ATATT3x...
```

Repository, project, and workspace **access tokens** are also supported; they
are sent as Bearer tokens and need no email:

```sh
bb auth login --bearer --with-token < token.txt
# or: export BB_TOKEN=... (without BB_EMAIL)
```

### OAuth 2.0 (`--web`)

Bitbucket has no device flow and requires a client secret, so `bb auth login
--web` uses an OAuth consumer **you** register: workspace settings → *OAuth
consumers* → *Add consumer*, callback URL `http://127.0.0.1:8976/callback`
(change the port with `--port` / `bb config set oauth_port`), scopes as
needed (at least `account`). Then:

```sh
export BB_OAUTH_CLIENT_ID=... BB_OAUTH_CLIENT_SECRET=...
bb auth login --web            # opens the browser, receives the callback on 127.0.0.1
bb auth refresh                # force a refresh (normally automatic)
```

Access tokens last 2 hours; `bb` refreshes them automatically with the stored
refresh token. The consumer secret is stored with the credential (hosts.yml or
keyring), never in `config.yml`.

### Credential storage

By default credentials live in `$XDG_CONFIG_HOME/bb/hosts.yml` (mode 0600).
To use the OS keychain (macOS Keychain, Windows Credential Manager, Secret
Service on Linux) instead:

```sh
bb config set credential_store keyring   # or BB_CREDENTIAL_STORE=keyring
bb auth login
```

### Git over HTTPS

`bb auth setup-git` registers `bb` as the git credential helper for
`https://bitbucket.org` only, so `git clone`/`push` reuse the same token.
(API tokens use the fixed git username `x-bitbucket-api-token-auth`; access
tokens use `x-token-auth` — `bb` picks the right one automatically.)

On Windows, where file modes are not enforced, prefer the keyring store or
environment variables.

### SSH

Bitbucket is moving SSH traffic from `bitbucket.org` to `ssh.bitbucket.org`
(deadline 2026-11-12). `bb` accepts both hosts in remotes and generates SSH
clone URLs for `ssh.bitbucket.org` when `git_protocol` is `ssh`.

## Commands

| bb | gh equivalent | Notes |
|---|---|---|
| `bb auth login/logout/status/token/refresh/setup-git` | `gh auth …` | API tokens; `--bearer` for access tokens; `--web` for OAuth |
| `bb repo list/view/create/clone/fork/delete/edit` | `gh repo …` | `list` needs a workspace (or iterates yours); `--role` is `contributor\|admin\|owner` |
| `bb pr list/view/create/checkout/merge/decline/approve/unapprove/review/comment/diff/status/checks/edit/ready` | `gh pr …` | `close` is an alias of `decline`; `reopen` is not possible on Bitbucket |
| `bb pipeline list/view/run/stop/watch/log` (`bb run` alias) | `gh run …` | |
| `bb branch list/create/delete` | — | |
| `bb workspace list/view/members` | `gh org …` | |
| `bb project list/view/create` | — | |
| `bb api <path>` | `gh api` | `{workspace}`/`{repo_slug}` placeholders, `--paginate`, `-f`/`-F`, `--jq` |
| `bb browse` | `gh browse` | |
| `bb config get/set/list` | `gh config …` | `workspace`, `git_protocol`, `merge_strategy`, `credential_store`, `oauth_client_id`, `oauth_port`, `editor`, `pager`, `browser` |
| `bb issue` | `gh issue` | **not available** — Bitbucket removed its issue tracker on 2026-08-20 |

Every listing/view command supports `--json <fields>`, `--jq <expr>`, and
`--template <go-template>` exactly like `gh`. Run `bb <cmd> --json` with an
invalid field to see the available fields.

### Repository selection

Commands operate on the repository of the current directory's git remote
(`upstream` > `origin` > first Bitbucket remote). Override with
`-R WORKSPACE/REPO` or `BB_REPO=WORKSPACE/REPO`. A bare `REPO` uses the
`workspace` config value.

### Environment

| Variable | Purpose |
|---|---|
| `BB_TOKEN`, `BB_EMAIL`, `BB_AUTH_METHOD` | credentials (`api_token` \| `bearer`; inferred from `BB_EMAIL` when unset) |
| `BB_OAUTH_CLIENT_ID`, `BB_OAUTH_CLIENT_SECRET` | OAuth consumer for `bb auth login --web` |
| `BB_CREDENTIAL_STORE` | `file` (default) \| `keyring` |
| `BB_REPO` | repository override |
| `BB_CONFIG_DIR` | config directory (default `$XDG_CONFIG_HOME/bb`) |
| `BB_PAGER`, `PAGER` | pager for long output |
| `BROWSER` | browser command for `--web` |
| `BB_DEBUG=1` | log HTTP requests (secrets masked); `=2` also logs response bodies (may contain personal data such as e-mail addresses) |
| `BB_NO_RETRY=1` | fail immediately on 429/5xx instead of retrying |
| `NO_COLOR` | disable colors |

## Differences from GitHub CLI

- No `bb issue` (Bitbucket Issues were removed by Atlassian in August 2026).
- `bb pr reopen` explains that declined pull requests cannot be reopened.
- `bb pr merge` exposes Bitbucket's six strategies via `--strategy`;
  `--merge`, `--squash`, `--rebase` map to `merge_commit`, `squash`,
  `rebase_merge`. The default comes from `bb config set merge_strategy`.
- OAuth login needs your own OAuth consumer (Bitbucket has no device flow and
  no public-client/PKCE support); API tokens are the zero-setup path.

## Development

```sh
make build      # bin/bb
make test
make lint       # requires golangci-lint
```

Integration tests against the real API run only with `BB_INTEGRATION=1`.

## License

MIT
