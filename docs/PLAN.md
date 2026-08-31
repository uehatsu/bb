# bb — Bitbucket Cloud CLI (GitHub CLI-like UX) implementation plan

Created: 2026-08-25 / Revision: v2 (incorporates the first MAGI review round) / Target: https://bitbucket.org (Bitbucket Cloud REST API 2.0)

## 0. Research summary (assumptions behind the plan)

### 0.1 Authentication (facts as of 2026-08)
| Method | Status | Usage |
|---|---|---|
| App Password | **Removed** (creation stopped 2025-09-09 → brownouts 2026-06-09 to 07-27 → **fully removed 2026-07-28**) | Unusable. Not supported |
| **API Token (scoped)** | **Current standard** | REST: `Authorization: Basic base64(<Atlassian account email>:<token>)`. Bearer also works (no email needed). git over HTTPS: username is the Bitbucket username or the fixed value **`x-bitbucket-api-token-auth`** |
| Repository / Project / Workspace Access Token | Current | REST: `Authorization: Bearer <token>`. git over HTTPS: username is the fixed value **`x-token-auth`** |
| OAuth 2.0 | Current | authorize `https://bitbucket.org/site/oauth2/authorize`, token `https://bitbucket.org/site/oauth2/access_token`. Authorization Code / Client Credentials / JWT exchange. **No Device Flow, no PKCE, client_secret required**. Access tokens live 2h; refresh_token available. Consumers are created per workspace and require a Callback URL |

- Creating an API Token: `id.atlassian.com/manage-profile/security/api-tokens` → "Create API token with scopes" → App=Bitbucket → pick scopes. **An expiry is mandatory** (an expired token shows up as 401).
- API Token scope names (new format): `read|write|admin|delete:repository:bitbucket`, `read|write:pullrequest:bitbucket`, `read|admin:project:bitbucket`, `read|admin:workspace:bitbucket`, `read|write:user:bitbucket`, `read|write|admin:pipeline:bitbucket`, `read|write|delete:webhook:bitbucket`, `read|write|delete:snippet:bitbucket`, `read|write|delete:ssh-key:bitbucket`, and more.
- OAuth scopes in the OpenAPI document use the old format (`repository`, `repository:write`, `pullrequest:write`, `pipeline`, `account`, …). Put a mapping table between the two in the README and in the `bb auth login` guidance. Scope discovery from response headers is not guaranteed for API Tokens, so `bb auth status` must not guess and display scopes.

### 0.2 API basics
- Base URL: `https://api.bitbucket.org/2.0` (OpenAPI v3 definition: `https://developer.atlassian.com/cloud/bitbucket/swagger.v3.json`, 173 paths. **It still lists removed endpoints, so do not trust it blindly**).
- Pagination: `{size, page, pagelen, next, previous, values}`. **Follow the `next` URL as-is** (never build page numbers yourself). `pagelen` maximum is roughly 100.
- Filter/sort: `q=<BBQL>` (e.g. `state="OPEN" AND author.uuid="{...}"`; UUIDs keep their `{}`), `sort=-updated_on` (single field only), `fields=` for partial responses (always set on list endpoints).
- Error format: `{"type":"error","error":{"message":"...","detail":"...","data":{...}}}`
- Rate limits: roughly 1,000/h authenticated (repository data) and up, 60/h anonymous. Exceeding it yields **429**, possibly with a `Retry-After` header → exponential backoff.
- Identifiers: workspace is a slug or `{uuid}`, repository is a slug or `{uuid}`.
- **Removed (do not implement)**
  - Issue Tracker / Wiki: UI and API removed on 2026-08-20 → no `bb issue`.
  - **CHANGE-2770 (cross-workspace API removal, deleted 2026-02 to 04)**: `GET /user/permissions/repositories`, `GET /repositories/{ws}?role=member`, and `GET /repositories` (all public repositories) are unavailable. "My repositories" is emulated in two stages: `GET /user/workspaces` → `GET /repositories/{ws}` for each. `--role` accepts only `contributor|admin|owner`.
- **SSH host migration**: `bitbucket.org` → **`ssh.bitbucket.org`** (SSH to `bitbucket.org` is refused from 2026-11-12). Remote parsing accepts both hosts; generated SSH URLs use `ssh.bitbucket.org`.

### 0.3 Main endpoints (implementation targets)
- User/Workspace: `GET /user`, `GET /user/workspaces`, `GET /workspaces/{ws}`, `GET /workspaces/{ws}/members`, `GET /workspaces/{ws}/projects`, `POST /workspaces/{ws}/projects`, `GET /workspaces/{ws}/projects/{key}`
- Repositories: `GET /repositories/{ws}?role=contributor|admin|owner&q=&sort=&fields=`, `GET|POST|PUT|DELETE /repositories/{ws}/{slug}` (POST body: `scm:"git"`, `is_private`, `project.key`, `description`, `fork_policy`), `POST /repositories/{ws}/{slug}/forks`, `GET .../forks`, `GET .../src/{commit}/{path}`, `GET .../commits`, `GET|POST|DELETE .../refs/branches[/{name}]`, `GET|POST|DELETE .../refs/tags[/{name}]`
- Pull requests: `GET /repositories/{ws}/{slug}/pullrequests?state=OPEN|MERGED|DECLINED|SUPERSEDED (repeatable)&q=&sort=&fields=`, `POST` (required `title`, `source.branch.name`; optional `destination.branch.name`, `description`, `reviewers[{uuid}]`, `close_source_branch`, `draft`), `GET|PUT .../{id}`, `POST|DELETE .../{id}/approve`, `POST|DELETE .../{id}/request-changes`, `POST .../{id}/decline`, `POST .../{id}/merge` (body: `merge_strategy` = `merge_commit|squash|fast_forward|squash_fast_forward|rebase_fast_forward|rebase_merge`, `close_source_branch`, `message`; `?async=true` returns 202 + a `Location` task-status URL to poll. A synchronous completion returns 200 with the PR), `GET|POST .../{id}/comments` (body `content.raw`; inline comments use `inline.path/to`), `GET .../{id}/diff|diffstat|commits|activity|statuses|conflicts`, `GET /workspaces/{ws}/pullrequests/{user}`
- Pipelines: `GET /repositories/{ws}/{slug}/pipelines?sort=-created_on`, `GET .../pipelines/{uuid}`, `POST .../pipelines` (body `target: {type:"pipeline_ref_target", ref_type:"branch", ref_name:"main", selector?:{type:"custom",pattern:"..."}}`, `variables[]`), `POST .../pipelines/{uuid}/stopPipeline`, `GET .../steps`, `GET .../steps/{step}/log` (supports Range)
- Default reviewers: `GET .../effective-default-reviewers`
- Generic: `bb api <path>` covers the remaining paths

## 1. Design principles

- Language/toolchain: **Go 1.22+** (Go is not installed in the current environment → `brew install go` is Step 0). Module name `github.com/ueno/bb` (provisional).
- CLI framework: `spf13/cobra` + `spf13/pflag`. Same `bb <noun> <verb>` structure as gh.
- The API client is **hand-written** (`net/http` + `encoding/json`). Rationale: fine-grained control over auth methods, `next`-following pagination, 429 retries, and `fields` handling in a thin layer. Third-party SDKs (ktrysmt/go-bitbucket) assume App Passwords and depend on removed APIs, so they are not adopted.
- Type definitions in `internal/bitbucket` (a sibling of `api`): **lenient structs that define only the fields we use** (unknown fields are ignored) to stay robust against API changes. The allowed `--json` fields are maintained in the same package as a per-type "display name → JSON path" map (gh's `exportable` approach).
- Output: table + colour on a TTY, tab-separated plain text otherwise. Every command implements `--json [fields]`, `--jq <expr>` (`itchyny/gojq`), and `--template` (Go template; `timeago`/`color`/`tablerow`/`truncate` etc. are ported from gh (MIT)).
- Interactive UI is unified on `charmbracelet/huh`, with `lipgloss` from the same family for colouring (`fatih/color` is not adopted).
- Repository resolution: `cmdutil.Factory.BaseRepo()` resolves `-R/--repo <workspace>/<slug>` → `BB_REPO` → git remote, in that order. `internal/gitctx` is limited to a pure (read-only) remote URL parser accepting `https://bitbucket.org/{ws}/{slug}(.git)`, `git@bitbucket.org:{ws}/{slug}`, `git@ssh.bitbucket.org:{ws}/{slug}`, and `ssh://git@(ssh.)bitbucket.org/{ws}/{slug}`. Writes to git (fetch/checkout/remote add/credential setup) live in `internal/git` (`exec.Command` with argument arrays, no shell, `--` before positional arguments).
- Configuration: `$XDG_CONFIG_HOME/bb/config.yml` (default workspace, editor, pager, git_protocol, merge_strategy, …) and `hosts.yml` (credentials). Directory 0700, files 0600, writes are atomic via temp file (`O_EXCL`) + rename. Permission bits are meaningless on Windows, so the README recommends the keyring there.
  - `config.CredentialStore` interface: `Get(host) (Credential, error)` / `Set(host, Credential)` / `Delete(host)`. `Credential{Method: api_token|bearer, Email, Token, ExpiresAt?}`. File implementation in v0.1, keyring implementation (`zalando/go-keyring`) later.
  - An `api.Authenticator` interface (`Apply(*http.Request)`) is built from a Credential; the Client knows nothing about auth methods (SRP).
- Environment variables: `BB_TOKEN` (API Token / Access Token), `BB_EMAIL`, `BB_AUTH_METHOD=api_token|bearer`, `BB_REPO`, `BB_WORKSPACE`, `NO_COLOR`, `BB_PAGER`, `BB_DEBUG`. **No flag for passing a token** (avoids exposure in `ps`).
- Errors: show Bitbucket's `error.message`/`detail` in a human-friendly way; exit codes follow gh (1: general, 2: cancelled/Ctrl-C, 4: authentication required). 401 → point to `bb auth login` (mention possible expiry); 403 → link to the scope table in the README (never infer scopes from the body).
- Security:
  - `--verbose`/`BB_DEBUG` logs mask the whole `Authorization` header, headers passed via `bb api -H`, redirect target URLs, and token-like strings in raw error responses. `bb auth token` warns on a TTY.
  - BBQL values are embedded only through `api.BBQLQuote(s)` (escapes `\` and `"`); when a user-supplied `--search` is combined with internally generated filters, wrap it in `( … )`.
  - `bb browse` validates ws/slug against `^[A-Za-z0-9_.{}-]+$`, applies `url.PathEscape`, and joins onto `https://bitbucket.org/`. Arbitrary schemes are never passed through.
  - `Retry-After` is clipped to 60s. Retries only for idempotent methods (GET/HEAD/PUT/DELETE) and 429. POST 5xx is never retried (prevents double merge/pipeline run/pr create).
- Polling abstraction `internal/api/poll.go`: `Poll(ctx, fn, PollOptions{Initial: 2s, Max: 30s, Factor: 1.5, Timeout})`. Shared by the `pr merge --async` task-status and `pipeline watch`. 429 is left to the retry layer; polling only stretches the interval. Ctrl-C exits 2.
- Pagination `Paginate(ctx, path, query, fields, limit, fn)`: `--limit` defaults to 30, `pagelen` = min(limit, 50). List commands always pass `fields=values.<needed fields>,next`.
- Testing: unit tests with `httptest.Server` (**a mandatory completion criterion for every step**) + golden files under `testdata/` (non-TTY output and JSON only, updated with `-update`). Real-API integration tests run under `BB_INTEGRATION=1` and are mandatory in nightly CI (manual smoke tests are an optional checklist).

## 2. Command structure (gh mapping)

| gh | bb | Notes |
|---|---|---|
| `gh auth login/logout/status/token/refresh/setup-git` | `bb auth login/logout/status/token/setup-git/git-credential` | login: interactive (email + API Token + optional expiry) / `--with-token` (stdin) / `--bearer` (Access Token). `refresh` arrives with OAuth |
| `gh repo list/view/create/clone/fork/delete/edit` | `bb repo list/view/create/clone/fork/delete/edit` | `list` takes `--workspace` (scans every workspace when omitted), `--role contributor|admin|owner`, `--limit` |
| `gh pr list/view/create/checkout/merge/close/reopen/review/comment/diff/status/edit/ready/checks` | `bb pr list/view/create/checkout/merge/decline(close alias)/approve/unapprove/review/comment/diff/status/edit/ready/checks` | `reopen` is unsupported by the API → the command exists but explains "Bitbucket cannot reopen a declined pull request; create a new one" and exits 1 |
| `gh run list/view/watch/rerun/cancel` | `bb pipeline list/view/watch/run/stop/log` (`bb run` as alias) | `run` = POST /pipelines |
| `gh browse` | `bb browse` | `bitbucket.org/{ws}/{slug}[/pull-requests/{id}]` |
| `gh api` | `bb api <path> [-X] [-f/-F] [--paginate] [--input] [-H] [-i] [--silent]` | `--paginate` follows `next` and concatenates `values` |
| `gh issue` | **Not supported** | Issue tracker was removed |
| `gh org` | `bb workspace list/view/members` | |
| — | `bb project list/view/create` | Bitbucket-specific |
| — | `bb branch list/create/delete` | refs API |
| `gh config get/set/list` | `bb config get/set/list` | can override the default `merge_strategy` |
| `gh completion` / `gh version` | same names | |

`bb pr merge` flags: `--merge` (=merge_commit, default) / `--squash` / `--rebase` (=rebase_merge) use the same names as gh; the six Bitbucket-specific strategies are reachable via `--strategy <name>`.

## 3. Directory layout

```
bb/
├── cmd/bb/main.go
├── internal/
│   ├── api/            # client.go, auth.go(Authenticator), pagination.go, retry.go, poll.go, errors.go, bbql.go
│   ├── bitbucket/      # type definitions + --json field maps (workspace, repository, pullrequest, pipeline, user, ref)
│   ├── config/         # config.yml / hosts.yml / CredentialStore(file, keyring)
│   ├── gitctx/         # remote URL parser (read-only)
│   ├── git/            # git exec wrapper (fetch/checkout/remote/credential)
│   ├── iostreams/      # TTY detection, colour, pager
│   ├── output/         # table, --json/--jq/--template
│   ├── cmdutil/        # Factory{IOStreams, Config, HTTPClient, BaseRepo, Git}, shared flags, exit codes
│   └── browser/
├── pkg/cmd/            # root/ auth/ repo/ pr/ pipeline/ workspace/ project/ branch/ api/ browse/ config/ version/
├── testdata/
├── docs/
├── .goreleaser.yml, .github/workflows/{ci,nightly,release}.yml
├── Makefile, go.mod, README.md
```

## 4. Step-by-step implementation plan

Each step is complete when "automated tests (mandatory)" + "manual checks (optional checklist)" pass. Estimates include a 1.5× buffer.
**Milestones**: v0.1 = Steps 0–8 (auth / api / basic repo / main pr / CI & release). v0.2 = Steps 9–11. v0.3 = Step 12.

### Step 0: Environment setup (0.5 day)
- `brew install go` (1.22+), `golangci-lint`, `goreleaser`. `git init`, `.gitignore`, `go mod init`.
- Done when: `go version` works and an empty `main.go` builds.

### Step 1: Skeleton, shared foundation, CI (1 day)
- cobra root, `bb version`, `bb completion`, global flag definitions.
- `internal/iostreams`, `internal/cmdutil` (Factory: lazily constructs IOStreams / Config / HTTPClient / BaseRepo / Git).
- GitHub Actions `ci.yml` (lint/test/build on macOS/Linux; Windows is build-only, best effort) is set up **at this point**.
- Done when: `bb --help` looks like gh, and `go test ./...` and golangci-lint are green in CI.

### Step 2: Configuration and credential store (1 day)
- `internal/config`: config.yml / hosts.yml (`gopkg.in/yaml.v3`), 0700/0600, atomic writes, `CredentialStore` (file), `Credential{Method, Email, Token, ExpiresAt}`. Environment precedence `BB_TOKEN` (+`BB_EMAIL`/`BB_AUTH_METHOD`) > hosts.yml.
- Done when: unit tests cover save/load/permissions/atomicity/environment precedence.

### Step 3: API client (1.5 days)
- `api.Client{Do, Paginate}`, `Authenticator` (Basic email:token / Bearer), `User-Agent: bb/<ver>`, `Accept: application/json`.
- Pagination (`next` following, `fields`, `limit`/`pagelen` rounding), retries (429: honour Retry-After seconds/HTTP-date, clip to 60s, exponential backoff up to 3 attempts; one retry for 5xx on idempotent methods; none for POST), `HTTPError{Status, Message, Detail, Raw}`, `BBQLQuote`, `Poll`.
- HTTP logging with `--verbose`/`BB_DEBUG=1` (masking rules in §1).
- Done when: httptest covers pagination, fields, 429/Retry-After, POST non-retry, error formatting, BBQL escaping, and Poll (timeout/cancel).

### Step 4: Output layer (1 day)
- `internal/output`: TablePrinter (TTY: aligned + colour, non-TTY: TSV), validation of allowed `--json fields` (tied to the `internal/bitbucket` maps), `--jq` (gojq), `--template` (ported gh-compatible functions).
- Done when: non-TTY golden tests (with `-update`).

### Step 5: `bb auth` (1.5 days)
- `login`: interactive (email → API Token with no echo → optional expiry) → verify with `GET /user` → save. `--with-token` (stdin: `email:token` or bare token + `--email`), `--bearer`. Prints the creation URL and the recommended scope table.
- `status`: `GET /user`, `GET /user/workspaces`, token type, saved expiry (warn from 7 days before). `logout`, `token` (with TTY warning).
- `setup-git`: registers a host-scoped helper such as `git config --global credential.https://bitbucket.org.helper '!bb auth git-credential'`. `git-credential`: implements the credential helper protocol (`get`/`store`/`erase`, key=value on stdin) and only responds for `protocol=https` and `host=bitbucket.org`. The username depends on Method: `x-bitbucket-api-token-auth` (API Token) / `x-token-auth` (Access Token).
- Done when: httptest covers login/status/helper protocol (silent on host mismatch). Manual: login → status → `git ls-remote https://…` with a real account.

### Step 6: Repository resolution, `bb api`, `bb browse` (1 day)
- `gitctx` parser (https / git@ / ssh:// × bitbucket.org / ssh.bitbucket.org), `Factory.BaseRepo()`.
- `bb api`: gh-compatible flags, `--paginate`, `--jq`. `bb browse [path|pr#] [--branch]` (input validation in §1).
- Done when: parser table tests and `bb api` httptest tests (paginate concatenation, -i, -f/-F, --input).

### Step 7: `bb repo` (1.5 days)
- `list [--workspace] [--role contributor|admin|owner] [--limit] [--language] [--visibility] [--sort]` (scans `/user/workspaces` when workspace is omitted), `view [--web] [--branch]` (README from `src/{mainbranch}/README.md`), `create <name> [--workspace] [--project KEY] [--private/--public] [--description] [--clone]` (explains that the oldest project is auto-assigned when none is given), `clone <ws/slug|slug>` (`git_protocol=ssh` generates `git@ssh.bitbucket.org:`), `fork [--workspace] [--name] [--clone]`, `delete [--yes]`, `edit`.
- Done when: httptest tests + non-TTY golden files for every subcommand.

### Step 8: `bb pr` + v0.1 release (3.5 days)
- `list [--state] [--author @me] [--search BBQL] [--limit]` (`@me` resolves the `/user` uuid → `author.uuid="{…}"`), `view [id|branch] [--web] [--comments]` (defaults to the current branch via `q=source.branch.name=<quoted>`), `create` (`-t/-b/--body-file/--base/--head/--reviewer/--draft/--close-source-branch/--fill/--web`; interactive mode uses huh; reviewers resolve nickname → uuid, optionally adding effective-default-reviewers), `checkout <id>` (adds a remote via `internal/git` for fork sources), `merge [id] [--merge|--squash|--rebase|--strategy] [--delete-branch] [--message]` (200 completes immediately; 202 follows `Location` with `Poll`; default timeout 5 minutes), `decline` (`close`), `reopen` (guidance only), `approve/unapprove`, `review`, `comment`, `diff [--name-only]`, `status`, `checks`, `edit`, `ready`.
- `release.yml` + goreleaser (brew tap, `-ldflags -X version`), README (API Token creation steps, scope table, differences from gh). **v0.1 tag**.
- Done when: httptest tests for every subcommand (including merge 200/202/timeout/Ctrl-C) + golden files. Manual: create → approve → merge.

### Step 9: `bb pipeline`, `bb branch` (1.5 days)
- `pipeline list/view/run/stop/watch/log` (`watch` shares `Poll` and ties the exit code to the result; `log` appends via Range), `branch list/create/delete`.
- Done when: httptest tests (watch state transitions, log Range).

### Step 10: `bb workspace`, `bb project`, `bb config` (1 day)
- Done when: httptest tests.

### Step 11: Nightly integration tests and documentation (1 day)
- Make `nightly.yml` (`BB_INTEGRATION=1`, a dedicated test workspace and secrets) **mandatory** to detect removed APIs and spec changes. Generate `docs/` with `cobra/doc`. **v0.2 tag**.

### Step 12 (v0.3): OAuth 2.0 login, keyring
- `bb auth login --web`: Authorization Code flow with **an OAuth Consumer created by the user** (key/secret from config or environment variables, Callback `http://127.0.0.1:<port>/callback`). `state` (crypto/rand) required, bind to `127.0.0.1`, one-shot + timeout, close immediately after receiving the code. The refresh_token is kept as secret as the access token and refreshed automatically before expiry.
- Keyring implementation of `CredentialStore`, `BB_CREDENTIAL_STORE=file|keyring`.

## 5. Risks and mitigations
- Confusion about using the email as the Basic username for API Tokens (git uses the username / a fixed value) → absorbed by the login explanation and the credential helper.
- **Ongoing API removals** (App Password, Issues/Wiki, cross-workspace API, SSH host) → lenient structs, mandatory nightly integration tests, and a README maintenance procedure to check the Atlassian changelog regularly.
- Rate limits → automatic retries, `--limit` default 30, mandatory `fields`, polling caps.
- Feature gaps versus gh (no issues, no reopen, no device flow) → explained both in the README and in in-command error messages.
- Windows → best effort in v0.1 (build only); hosts.yml permissions are compensated by recommending the keyring.

## 6. Dependencies (minimal)
`spf13/cobra`, `spf13/pflag`, `gopkg.in/yaml.v3`, `itchyny/gojq`, `mattn/go-isatty`, `charmbracelet/huh` + `charmbracelet/lipgloss`, `golang.org/x/term`, `pkg/browser`. Later: `zalando/go-keyring`. Tests: standard `testing` + `google/go-cmp`.

## 7. References
- API Token: https://support.atlassian.com/bitbucket-cloud/docs/using-api-tokens/ , https://support.atlassian.com/bitbucket-cloud/docs/api-token-permissions/
- App Password removal: https://www.atlassian.com/blog/bitbucket/bitbucket-cloud-transitions-to-api-tokens-enhancing-security-with-app-password-deprecation , https://community.atlassian.com/forums/Bitbucket-articles/Deprecation-notice-Bitbucket-Cloud-app-password-brownout/ba-p/3237429
- CHANGE-2770 (cross-workspace API removal): https://community.atlassian.com/forums/Bitbucket-questions/replacements-for-deprecated-user-scoped-permission-endpoints/qaq-p/3166685
- SSH host migration: https://community.atlassian.com/forums/Bitbucket-articles/Upcoming-change-to-Bitbucket-Cloud-SSH-access-move-from/ba-p/3234032
- OAuth 2.0: https://developer.atlassian.com/cloud/bitbucket/oauth-2/ , https://support.atlassian.com/bitbucket-cloud/docs/use-oauth-on-bitbucket-cloud/
- REST intro / OpenAPI: https://developer.atlassian.com/cloud/bitbucket/rest/intro/ , https://developer.atlassian.com/cloud/bitbucket/swagger.v3.json
- Rate limits: https://support.atlassian.com/bitbucket-cloud/docs/api-request-limits/
- Issues/Wiki removal: https://community.atlassian.com/forums/Bitbucket-articles/Announcing-sunset-of-Bitbucket-Issues-and-Wikis/ba-p/3193882
- Similar OSS (reference): https://github.com/avivsinai/bitbucket-cli (MIT)

## 8. Implementation notes (WARNINGS from the second MAGI review round — leftovers after full APPROVE)
- Server-provided URLs (pagination `next`, the merge 202 `Location`, `bb api --paginate`) get `Authenticator.Apply` only when the host is `https://api.bitbucket.org`. Other hosts are rejected.
- `Poll` keeps going with a longer interval on HTTPError(429) and aborts on any other error.
- `git-credential` `store`/`erase` are no-ops (the helper never rewrites hosts.yml). The `host=bitbucket.org:443` form is accepted too. `setup-git` resets existing helpers with an empty `helper=` entry before registering (like gh).
- With only `BB_TOKEN` set and `BB_AUTH_METHOD` unset: treat as api_token when `BB_EMAIL` is present, otherwise bearer (consistent with the git-credential username choice).
- `--verbose` never prints response bodies. Body output requires the explicit `BB_DEBUG=2` opt-in.
- A `Retry-After` HTTP-date in the past means 0 seconds. `BB_NO_RETRY=1` offers immediate failure for CI use (optional).
- The OAuth consumer secret is stored in hosts.yml/keyring, not config.yml.
- `repo list` without a workspace: use `config.workspace` as the default if set, otherwise scan (`--limit` caps the cross-workspace total; keep `fields` minimal).
- `pr view <arg>`: digits only → id, anything else → branch name.
- `Credential.ExpiresAt` is self-reported. When unset, `auth status` shows "no expiry recorded".
- The `--json` field maps in `internal/bitbucket` have table tests verifying that every key exists on the struct.
- Step 8 is split into commits 8a (list/view/create/checkout) / 8b (merge/decline/approve/review/comment) / 8c (the rest + release).
- Nightly failures do not block main; file an issue instead (operational).

## 9. Progress (2026-08-25)
- Steps 0–11 implemented (v0.1 + v0.2 equivalent). Every command has httptest-based unit tests; golangci-lint reports 0 issues.
- Step 12 implemented: `bb auth login --web` (Authorization Code, state validation, fixed port on 127.0.0.1, one-shot), `bb auth refresh`, automatic refresh in the Factory, `credential_store=keyring` (zalando/go-keyring).
- The smoke test against the real API (`bb auth login` → `pr create` → `merge`) needs manual verification with an API Token.

## 10. MAGI code review follow-up (2026-08-25)
All 9 categories raised by the MAGI review of the full implementation (MELCHIOR=APPROVE / BALTHASAR=CONDITIONAL / CASPER=CONDITIONAL) have been addressed:
the `pr checkout` fix and the git Runner interface (with regression tests), consolidating OAuth refresh into `config.ResolveFreshCredential` (shared by the API client, the git credential helper, and `auth token`), ignoring forged OAuth callback requests and continuing to wait, implementing `pipeline watch --exit-status`, `pr diff --color {always|never|auto}`, Ctrl-C = exit 2 via `signal.NotifyContext`, `pr checks --watch` on top of Poll, validation of remote/server-provided URLs and names, sending the title with `pr ready/edit`, surfacing previously swallowed errors, the `--commit` target format, consolidation into `OpenBrowser`/`OptionalArg`/`MainBranch`/`gitctx.CloneURL`, `fields` on list calls, fewer redundant refetches, cleaning up short flags that clashed with gh, `bb api` support for `-H --paginate`/`?`, credential migration when switching `credential_store`, and isolating environment variables in tests.

### MAGI re-review (2026-08-25, finished in 3 rounds)
- Round 2: MELCHIOR=APPROVE / BALTHASAR=CONDITIONAL (`decline --delete-branch` wrongly deleting fork branches) / CASPER=APPROVE → fixed in `b2b7836` (refuse forks, `pr merge -b/--body`, review ordering, refreshingAuth, checks redraw/--timeout, final log fetch, secret masking, browser allowlist)
- Round 3: **all three APPROVE** (confidence 9/9/9). Remaining warnings were UX/maintainability suggestions only (consolidating URL literals, `fields` for fetchPR, suppressing the refresh-failure warning, fork decline wording).

### Improvement cycle (2026-08-25, all remaining MAGI warnings)
Groups 1–5 (11 UX, 8 maintainability, 6 test, 2 documentation items) were all implemented. The README states that Data Center is unsupported (B5). Verification against the real API (C1 and A5: whether `q=` filters are supported) was added as integration tests but not yet run because it needs an API Token and a test repository.

### Codex adversarial review follow-up (v0.1.1)
All 3 findings fixed: PR URLs are parsed as URLs and target the repository in the URL (other hosts and malformed paths are rejected), `branch delete` requires `--yes` when non-interactive, and the `credential_store` migration is rollback-safe via "copy → save config → read back and verify → delete old" (with write-failure injection tests).

### Real-API verification results (2026-08-25, read-only)
Integration tests run with `BB_INTEGRATION=1`: `/user`, `/user/workspaces`, repository fetch, and PR list PASS. `/workspaces/{ws}/members?q=user.nickname="…"` **is supported** (the A5 fast path is effective). `/user/permissions/repositories` returns non-2xx (as assumed for CHANGE-2770). Write paths (partial PUT, `pipeline_commit_target`) are still waiting for a run with `BB_INTEGRATION_WRITE=1`.

### v0.1.2 (2026-08-26)
Fixed `bb pipeline view/watch/log/stop <build_number>` always returning the oldest pipeline (#1). Cause: the pipelines list API ignores `q=build_number=N` (confirmed against the real API). Fix: search the newest-first list (`sort=-created_on`) for the requested number, predicting the page from the sequential numbering and falling back to a descending scan with early termination when there are gaps. Added a regression test with a mock server that ignores `q`.
