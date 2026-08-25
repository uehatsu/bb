## bb pr checkout

Check out a pull request in git

### Synopsis

Fetch the pull request's source branch and check it out locally.

For pull requests from forks, a remote named after the fork's workspace is
added and the branch is fetched from it.

```
bb pr checkout {<number> | <branch> | <url>} [flags]
```

### Options

```
  -b, --branch string   Local branch name to use (default: the source branch name)
      --detach          Checkout PR with a detached HEAD
  -f, --force           Reset the existing local branch to the latest state of the pull request
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb pr](bb_pr.md)	 - Manage pull requests

