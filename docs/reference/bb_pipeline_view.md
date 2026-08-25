## bb pipeline view

View a pipeline run and its steps

```
bb pipeline view <number|uuid> [flags]
```

### Options

```
  -q, --jq string         Filter JSON output using a jq expression
      --json strings      Output JSON with the specified fields (comma-separated)
  -t, --template string   Format JSON output using a Go template
  -w, --web               Open the pipeline in the browser
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb pipeline](bb_pipeline.md)	 - Run and monitor Bitbucket Pipelines

