## bb pr merge

Merge a pull request

### Synopsis

Merge a pull request on Bitbucket.

The strategy defaults to the merge_strategy config value (merge_commit).
--merge, --squash and --rebase mirror GitHub CLI; Bitbucket's full set of
strategies is available through --strategy: merge_commit, squash,
fast_forward, squash_fast_forward, rebase_fast_forward, rebase_merge.

Bitbucket may perform the merge asynchronously; in that case bb polls the
merge task until it completes (see --timeout).

```
bb pr merge [<number> | <branch> | <url>] [flags]
```

### Examples

```
  $ bb pr merge 42 --squash --delete-branch
  $ bb pr merge --strategy rebase_fast_forward -m "Rebase and merge"
```

### Options

```
  -d, --delete-branch      Delete the source branch after merge
  -m, --merge              Merge with a merge commit
      --message string     Commit message for the merge commit
  -r, --rebase             Rebase the commits onto the base branch and merge (rebase_merge)
  -s, --squash             Squash the commits into one commit and merge
      --strategy string    Bitbucket merge strategy (see help)
      --timeout duration   How long to wait for an asynchronous merge (default 5m0s)
  -y, --yes                Skip the confirmation prompt
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb pr](bb_pr.md)	 - Manage pull requests

