## bb pipeline list

List recent pipeline runs

```
bb pipeline list [flags]
```

### Options

```
  -b, --branch string     Filter by branch
  -q, --jq string         Filter JSON output using a jq expression
      --json strings      Output JSON with the specified fields (comma-separated)
  -L, --limit int         Maximum number of pipelines to list (default 20)
  -s, --status string     Filter by status: pending|in_progress|completed|successful|failed|error|stopped
  -t, --template string   Format JSON output using a Go template
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb pipeline](bb_pipeline.md)	 - Run and monitor Bitbucket Pipelines

