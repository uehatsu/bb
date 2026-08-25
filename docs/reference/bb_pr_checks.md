## bb pr checks

Show build statuses for a pull request

### Synopsis

List the commit statuses (pipelines and other builds) reported on a pull
request. Exit code 8 indicates checks are still in progress, and 1 that some
failed.

```
bb pr checks [<number> | <branch> | <url>] [flags]
```

### Options

```
  -i, --interval duration   Refresh interval when watching (default 10s)
      --timeout duration    Give up watching after this duration (0 = no limit)
      --watch               Watch checks until they finish
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb pr](bb_pr.md)	 - Manage pull requests

