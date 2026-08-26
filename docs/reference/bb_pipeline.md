## bb pipeline

Run and monitor Bitbucket Pipelines

```
bb pipeline <command> [flags]
```

### Examples

```
  $ bb pipeline list
  $ bb pipeline run --branch main
  $ bb pipeline watch 128
  $ bb pipeline log 128
```

### Options

```
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb](bb.md)	 - Bitbucket Cloud CLI
* [bb pipeline list](bb_pipeline_list.md)	 - List recent pipeline runs
* [bb pipeline log](bb_pipeline_log.md)	 - Print the log of a pipeline step
* [bb pipeline run](bb_pipeline_run.md)	 - Trigger a pipeline
* [bb pipeline stop](bb_pipeline_stop.md)	 - Stop a running pipeline
* [bb pipeline view](bb_pipeline_view.md)	 - View a pipeline run and its steps
* [bb pipeline watch](bb_pipeline_watch.md)	 - Watch a pipeline until it completes

