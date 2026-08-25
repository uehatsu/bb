## bb pipeline log

Print the log of a pipeline step

### Synopsis

Print the log output of a pipeline's steps. By default all steps are
printed in order; use --step N to select one. With --follow the log is
tailed (using HTTP Range requests) until the step completes.

```
bb pipeline log <number|uuid> [flags]
```

### Options

```
  -f, --follow     Keep fetching new log output until the step finishes
  -s, --step int   Step number to show (1-based)
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb pipeline](bb_pipeline.md)	 - Run and monitor Bitbucket Pipelines

