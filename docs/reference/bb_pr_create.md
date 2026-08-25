## bb pr create

Create a pull request

### Synopsis

Create a pull request on Bitbucket.

The source branch defaults to the current git branch and the destination to
the repository's main branch. When --title is omitted in a terminal, the
title and body are prompted for; --fill derives them from the commits.

Reviewers are given by nickname and resolved against workspace members. The
repository's effective default reviewers are added automatically unless
--no-default-reviewers is set.

```
bb pr create [flags]
```

### Examples

```
  $ bb pr create --title "Fix login" --body "Closes JIRA-1"
  $ bb pr create --fill --reviewer alice --reviewer bob
  $ bb pr create --base develop --draft
  $ bb pr create --web
```

### Options

```
  -B, --base string            The branch into which you want your code merged
  -b, --body string            Body for the pull request
  -F, --body-file string       Read body text from file (use "-" to read from standard input)
      --close-source-branch    Delete the source branch after merge
  -d, --draft                  Mark pull request as a draft
  -f, --fill                   Use commit info for title and body
  -H, --head string            The branch that contains commits for your pull request (default: current branch)
      --no-default-reviewers   Do not add the repository's default reviewers
  -r, --reviewer strings       Request reviews from people by nickname
  -t, --title string           Title for the pull request
  -w, --web                    Open the browser to create a pull request
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb pr](bb_pr.md)	 - Manage pull requests

