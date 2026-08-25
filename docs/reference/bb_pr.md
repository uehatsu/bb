## bb pr

Manage pull requests

### Synopsis

Work with Bitbucket pull requests.

### Examples

```
  $ bb pr list
  $ bb pr create --title "Fix bug" --body "Details"
  $ bb pr checkout 42
  $ bb pr merge 42 --squash --delete-branch
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
* [bb pr approve](bb_pr_approve.md)	 - Approve a pull request
* [bb pr checkout](bb_pr_checkout.md)	 - Check out a pull request in git
* [bb pr checks](bb_pr_checks.md)	 - Show build statuses for a pull request
* [bb pr comment](bb_pr_comment.md)	 - Add a comment to a pull request
* [bb pr create](bb_pr_create.md)	 - Create a pull request
* [bb pr decline](bb_pr_decline.md)	 - Decline (close) a pull request
* [bb pr diff](bb_pr_diff.md)	 - View changes in a pull request
* [bb pr edit](bb_pr_edit.md)	 - Edit a pull request
* [bb pr list](bb_pr_list.md)	 - List pull requests in a repository
* [bb pr merge](bb_pr_merge.md)	 - Merge a pull request
* [bb pr ready](bb_pr_ready.md)	 - Mark a pull request as ready for review
* [bb pr reopen](bb_pr_reopen.md)	 - Reopen a pull request (not supported by Bitbucket)
* [bb pr review](bb_pr_review.md)	 - Add a review to a pull request
* [bb pr status](bb_pr_status.md)	 - Show status of relevant pull requests
* [bb pr unapprove](bb_pr_unapprove.md)	 - Remove your approval from a pull request
* [bb pr view](bb_pr_view.md)	 - View a pull request

