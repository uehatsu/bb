## bb repo list

List repositories in a workspace

### Synopsis

List repositories in a workspace.

When no workspace is given, the configured default workspace is used; if none
is configured, every workspace the user belongs to is listed in turn.

Note: Bitbucket removed the cross-workspace repository listing API in 2026, so
--role member is no longer available; use contributor, admin, or owner.

```
bb repo list [<workspace>] [flags]
```

### Options

```
  -q, --jq string           Filter JSON output using a jq expression
      --json strings        Output JSON with the specified fields (comma-separated)
  -l, --language string     Filter by primary coding language
  -L, --limit int           Maximum number of repositories to list (default 30)
      --role string         Filter by the user's role: {contributor|admin|owner}
      --search string       Additional BBQL filter, e.g. 'name ~ "api"'
      --sort string         Sort field, prefix with - for descending (default "-updated_on")
  -t, --template string     Format JSON output using a Go template
      --visibility string   Filter by visibility: {public|private}
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb repo](bb_repo.md)	 - Manage repositories

