## bb pr list

List pull requests in a repository

```
bb pr list [flags]
```

### Examples

```
  $ bb pr list
  $ bb pr list --state merged --limit 50
  $ bb pr list --author @me
  $ bb pr list --search 'title ~ "fix"' --json id,title
```

### Options

```
  -A, --author string     Filter by author nickname (@me for yourself)
  -B, --base string       Filter by destination branch
  -H, --head string       Filter by source branch
  -q, --jq string         Filter JSON output using a jq expression
      --json strings      Output JSON with the specified fields (comma-separated)
  -L, --limit int         Maximum number of items to fetch (default 30)
  -S, --search string     Additional BBQL filter expression
      --sort string       Sort field, prefix with - for descending (default "-updated_on")
  -s, --state string      Filter by state: {open|merged|declined|superseded|all} (default "open")
  -t, --template string   Format JSON output using a Go template
  -w, --web               Open the pull request list in the browser
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb pr](bb_pr.md)	 - Manage pull requests

