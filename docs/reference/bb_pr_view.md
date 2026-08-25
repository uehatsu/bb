## bb pr view

View a pull request

### Synopsis

Display the title, body, and other information about a pull request.

Without an argument, the pull request for the current branch is shown.

```
bb pr view [<number> | <branch> | <url>] [flags]
```

### Options

```
  -c, --comments          View pull request comments
  -q, --jq string         Filter JSON output using a jq expression
      --json strings      Output JSON with the specified fields (comma-separated)
  -t, --template string   Format JSON output using a Go template
  -w, --web               Open the pull request in the browser
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb pr](bb_pr.md)	 - Manage pull requests

