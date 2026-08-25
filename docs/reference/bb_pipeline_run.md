## bb pipeline run

Trigger a pipeline

### Synopsis

Trigger a pipeline for a branch (default: the current branch), tag, or
commit. Use --custom to run a custom pipeline defined under "custom:" in
bitbucket-pipelines.yml, and --var KEY=VALUE to pass pipeline variables.

```
bb pipeline run [flags]
```

### Examples

```
  $ bb pipeline run
  $ bb pipeline run --branch main --custom deploy --var ENV=prod
  $ bb pipeline run --tag v1.2.0 --watch
```

### Options

```
  -b, --branch string     Branch to run the pipeline for (default: current branch)
  -c, --commit string     Specific commit hash to build
      --custom string     Name of a custom pipeline to run
  -t, --tag string        Tag to run the pipeline for
      --var stringArray   Pipeline variable KEY=VALUE (repeatable)
  -w, --watch             Watch the pipeline until it completes
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb pipeline](bb_pipeline.md)	 - Run and monitor Bitbucket Pipelines

