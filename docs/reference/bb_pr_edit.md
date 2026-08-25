## bb pr edit

Edit a pull request

```
bb pr edit [<number> | <branch> | <url>] [flags]
```

### Examples

```
  $ bb pr edit 42 --title "New title" --body "New body"
  $ bb pr edit --add-reviewer alice --remove-reviewer bob
  $ bb pr edit --base develop
```

### Options

```
      --add-reviewer strings      Add reviewers by nickname
  -B, --base string               Change the destination branch
  -b, --body string               Set the new body
  -F, --body-file string          Read body text from file
      --remove-reviewer strings   Remove reviewers by nickname
  -t, --title string              Set the new title
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb pr](bb_pr.md)	 - Manage pull requests

