## bb branch list

List branches

```
bb branch list [flags]
```

### Options

```
  -q, --jq string         Filter JSON output using a jq expression
      --json strings      Output JSON with the specified fields (comma-separated)
  -L, --limit int         Maximum number of branches to list (default 30)
      --search string     Filter branches whose name contains this text
      --sort string       Sort field (e.g. name, -target.date) (default "-target.date")
  -t, --template string   Format JSON output using a Go template
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb branch](bb_branch.md)	 - Manage branches on Bitbucket

