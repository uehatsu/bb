## bb pr comment

Add a comment to a pull request

```
bb pr comment [<number> | <branch> | <url>] [flags]
```

### Examples

```
  $ bb pr comment 42 --body "Thanks!"
  $ bb pr comment --body-file note.md
  $ bb pr comment 42 --path src/main.go --line 10 -b "typo"
```

### Options

```
  -b, --body string        The comment body text
  -F, --body-file string   Read body text from file (use "-" for stdin)
  -e, --editor             Open an editor to write the comment
      --line int           Line number (in the new file) for an inline comment
      --path string        File path for an inline comment
```

### Options inherited from parent commands

```
      --help          Show help for command
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
```

### SEE ALSO

* [bb pr](bb_pr.md)	 - Manage pull requests

