## bb pipeline watch

Watch a pipeline until it completes

### Synopsis

Poll a pipeline until it completes. The exit code is 1 when the pipeline did not succeed.

```
bb pipeline watch <number|uuid> [flags]
```

### Options

```
      --exit-status         Exit with non-zero status if the pipeline fails (default true)
  -i, --interval duration   Initial polling interval (default 3s, grows to 30s)
      --timeout duration    Give up after this long (default 2h0m0s)
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb pipeline](bb_pipeline.md)	 - Run and monitor Bitbucket Pipelines

