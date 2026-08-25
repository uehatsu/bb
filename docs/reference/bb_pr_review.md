## bb pr review

Add a review to a pull request

### Synopsis

Approve, request changes on, or comment on a pull request.

Bitbucket records approvals and change requests separately from comments,
so a body given with --approve or --request-changes is posted as a comment
in addition to the review action.

```
bb pr review [<number> | <branch> | <url>] [flags]
```

### Examples

```
  $ bb pr review 42 --approve
  $ bb pr review --request-changes -b "Please add tests"
  $ bb pr review --comment -b "Looks reasonable"
```

### Options

```
  -a, --approve            Approve the pull request
  -b, --body string        Body of the review comment
  -F, --body-file string   Read body text from file (use "-" for stdin)
  -c, --comment            Comment on the pull request
  -r, --request-changes    Request changes on the pull request
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb pr](bb_pr.md)	 - Manage pull requests

