---
name: bitbucket
description: "Operate Bitbucket Cloud from Codex through the `bb` CLI (GitHub CLI-like): repositories, pull requests, pipelines, branches, workspaces, projects, and raw API calls. Use when the user mentions Bitbucket, a bitbucket.org URL, a Bitbucket pull request or pipeline, or asks to do with Bitbucket what `gh` does for GitHub."
metadata:
  short-description: Use Bitbucket Cloud via bb
---

# Bitbucket Cloud via `bb`

`bb` is a command-line client for Bitbucket Cloud with the same ergonomics as
GitHub CLI (`gh`). This skill tells Codex how to use `bb` safely and
efficiently. Repository: https://github.com/uehatsu/bb

## 0. Preflight (do this first, every time)

```sh
bb version            # missing? ask the user to install: brew install uehatsu/tap/bb (or go install github.com/uehatsu/bb/cmd/bb@latest)
bb auth status        # non-zero exit = not logged in -> guide the user through `bb auth login` (see section 5)
```

- If `bb` is missing or not logged in, **do not fall back to curl or a
  hand-written API client**. Ask the user to install / log in.
- The target repository is inferred from the current directory's git remote
  (`upstream` > `origin`). Outside a Bitbucket checkout, **always pass
  `-R WORKSPACE/REPO`**.

## 1. Ground rules

1. **Prefer machine-readable output.** Fetch data with `--json <fields>` and
   `--jq`; never parse the table output. To list the available fields, run
   `bb <cmd> --json x` with a bogus field name.
2. **Run non-interactively.** The command runner is not a TTY. Pass explicit
   flags for anything that would prompt: `pr create --title/--body`,
   `pr merge --squash|--merge|--rebase`, `repo delete --yes`,
   `branch delete --yes`, `repo create --private|--public`.
3. **Confirm destructive actions first.** Before `pr merge`, `pr decline`,
   `branch delete`, `repo delete`, `repo edit --visibility`, or
   `pipeline stop`, show the user the exact target (repository, number,
   branch) and wait for approval. Add `--yes` only after the user explicitly
   agreed.
4. **Never print secrets.** Do not paste the output of `bb auth token`,
   `BB_TOKEN`, or the contents of `hosts.yml` into the conversation. Do not
   use `BB_DEBUG=2` (it logs response bodies, which may contain personal
   data).
5. **Pull requests may be given as URLs.**
   `bb pr view https://bitbucket.org/ws/repo/pull-requests/42` targets the
   repository named in the URL (the current repository is ignored). A bare
   number targets the current / `-R` repository.
6. **Read exit codes.** 0 success, 1 error, 2 cancelled, 4 authentication
   required, 8 still in progress (`pr checks`). On 4, ask the user to log in.

### Destructive-operation workflow (strict)

1. Collect the identifying details (repository, PR number, branch name, a
   one-line summary) and present them to the user.
2. Wait for explicit approval in the user's own words. Do not run anything
   before that.
3. Once approved, run the command with `--yes` and report the result.

Example: deleting a branch

```text
Proposal: "Delete branch feat/x from workspace/repo. Proceed?"
After approval: bb branch delete feat/x --yes
```

## 2. Command cheat sheet

### Repositories

```sh
bb repo list <workspace> --json fullName,description,updatedAt   # -L limits count; --role contributor|admin|owner
bb repo view [ws/repo] --json fullName,mainBranch,url,isPrivate
bb repo clone ws/repo [dir] [-- --depth 1]
bb repo create ws/name --private --project KEY -d "description"
bb repo fork ws/repo --workspace mine --clone
bb repo edit --default-branch main --description "..."
bb repo delete ws/repo --yes          # needs user approval
```

### Pull requests

```sh
bb pr list --state open|merged|declined|all --author @me --base main -L 20 --json id,title,state,headRefName,author,url
bb pr view 42 --json id,title,body,state,headRefName,baseRefName,reviewers,participants,url
bb pr view 42 --comments                         # include review comments
bb pr diff 42 [--stat | --name-only] --color never   # plain text, no ANSI
bb pr checks 42                                   # exit 8 = running, 1 = something failed
bb pr create --title "..." --body "..." [--base develop] [--head feat/x] [--reviewer alice,bob] [--draft] [--close-source-branch]
bb pr create --fill                               # title/body from the commits (omit --title)
bb pr checkout 42                                 # fetch and check out the branch (forks supported)
bb pr review 42 --approve | --request-changes -b "reason" | --comment -b "..."
bb pr comment 42 -b "..." [--path src/x.go --line 10]
bb pr edit 42 --title "..." --body "..." --add-reviewer carol --remove-reviewer bob --base main
bb pr ready 42 [--undo]                           # draft <-> ready for review
bb pr merge 42 --squash|--merge|--rebase [--delete-branch] [-b "commit message"] --yes   # needs user approval
bb pr decline 42 [--delete-branch]                # needs user approval (close is an alias; reopen is impossible)
bb pr status                                      # your PRs and review requests
```

- Pass multi-line bodies via `--body-file -` (stdin / heredoc).
- Bitbucket's six merge strategies are available through
  `--strategy merge_commit|squash|fast_forward|squash_fast_forward|rebase_fast_forward|rebase_merge`.

### Pipelines (Bitbucket Pipelines)

```sh
bb pipeline list -L 10 --json buildNumber,status,result,refName,createdAt,url
bb pipeline view 128 [--json buildNumber,status,result,refName]   # lists the steps
bb pipeline run --branch main [--custom deploy --var ENV=prod] [--watch]
bb pipeline watch 128 [--exit-status=false]       # wait for completion; exit 1 on failure unless --exit-status=false
bb pipeline log 128 [--step 2] [--follow]
bb pipeline stop 128                              # needs user approval
```

### Branches / workspaces / projects

```sh
bb branch list -L 20 --json name,target
bb branch create feat/x --from main
bb branch delete feat/x --yes                     # needs user approval
bb workspace list --json slug,name
bb workspace members <ws>
bb project list -w <ws> --json key,name
bb project create KEY --name "..." -w <ws> --private
```

### Raw API (only when no dedicated command fits)

```sh
bb api /user
bb api repositories/{workspace}/{repo_slug}/commits --paginate --jq '.[].hash'
bb api -X POST repositories/{workspace}/{repo_slug}/refs/tags -f name=v1.0 -F 'target[hash]=abc123'
bb api -X PUT repositories/ws/repo -f description="..."
```

- `{workspace}` / `{repo_slug}` are replaced with the target repository.
- `--paginate` follows `next` links and concatenates `values` into one JSON array.
- `-f` sends strings; `-F` sends typed values (`true`, `false`, `null`,
  integers, `@file`). With GET, fields become query parameters.

### Open in the browser

```sh
bb browse [42 | path/to/file[:line]] [--branch x] [--pull-requests | --pipelines] [-n]   # -n prints the URL only
```

## 3. Typical workflows

**Create a pull request**

1. Inspect the change: `git status`, `git log origin/<base>..HEAD`.
2. Make sure the branch is pushed (`git push -u origin HEAD` if the user agrees).
3. `bb pr create --title "..." --body-file - --base <base> [--reviewer ...] <<'EOF' ... EOF`
4. Show the user the URL that is printed.

**Review a pull request**

1. `bb pr view <n> --json title,body,headRefName,baseRefName,author,participants`
2. `bb pr diff <n>` (use `--stat` / `--name-only` first when the diff is large,
   or `bb pr checkout <n>` to read files locally).
3. `bb pr checks <n>`
4. Post findings with `bb pr comment` / `bb pr review --request-changes -b`
   after showing the user what will be posted.

**Investigate a CI failure**

1. `bb pr checks <n>` or `bb pipeline list --branch <br> -L 3 --json buildNumber,result,url`
2. `bb pipeline view <build#>` for the steps and their results.
3. `bb pipeline log <build#> --step <k>` (pipe through `tail -200` when long).

**Merge**

1. Confirm `bb pr checks <n>` is green and check approvals via `--json participants`.
2. Ask the user which strategy to use and whether to delete the branch.
3. `bb pr merge <n> --squash --delete-branch --yes`

## 4. Bitbucket specifics (differences from gh)

- **There are no issues.** Bitbucket Issues were removed in August 2026; use
  Jira or similar. `bb issue` does not exist.
- **Declined pull requests cannot be reopened.** Create a new one from the same branch.
- `pr decline --delete-branch` never deletes a branch that lives in a fork (it warns instead).
- Repositories are addressed as `workspace/slug` (the GitHub `owner/repo`
  equivalent). With `bb config set workspace <ws>` a bare `slug` works too.
- SSH traffic goes to `ssh.bitbucket.org` (`bitbucket.org` stops accepting
  SSH in November 2026); `bb` understands both remote forms.
- Authentication uses **Atlassian API tokens**, not app passwords (retired July 2026).

## 5. Guiding the user through login

```text
1. Open https://id.atlassian.com/manage-profile/security/api-tokens
2. "Create API token with scopes" -> app: Bitbucket -> grant the scopes you need
   (read/write repository, pullrequest, pipeline; read user, workspace, project) -> set an expiry
3. Run `bb auth login` and enter your Atlassian account e-mail and the token
   (`--expires-in 1y` enables expiry warnings)
4. Run `bb auth setup-git` to reuse the token for git push/fetch
```

Non-interactive environments (CI): set `BB_EMAIL` and `BB_TOKEN`.

## 6. Troubleshooting

| Symptom | Action |
|---|---|
| exit 4 / `not logged in` | guide the user through section 5 |
| `HTTP 401` | the token has probably expired; check with `bb auth status` and recreate it |
| `HTTP 403` | missing scope; create a new token with the required scopes (scopes of an existing token cannot be changed) |
| `no Bitbucket remote found` | pass `-R ws/repo` or run inside a Bitbucket checkout |
| `HTTP 429` | retries already happened; reduce calls (`-L`, narrower `--json` fields) |
| `--yes required` | destructive command in a non-interactive run; add `--yes` after the user approved |
| output too long | select fields with `--json` + `--jq`, cap with `-L` |
