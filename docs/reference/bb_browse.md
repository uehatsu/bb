## bb browse

Open the repository in the browser

### Synopsis

Open the current repository on bitbucket.org in the web browser.

A numeric argument opens that pull request; a path opens the file or
directory on the selected branch.

```
bb browse [<number> | <path>] [flags]
```

### Examples

```
  $ bb browse
  $ bb browse 42
  $ bb browse src/main.go --branch develop
  $ bb browse --pull-requests
  $ bb browse -n   # print the URL only
```

### Options

```
  -b, --branch string   Select another branch by passing in the branch name
  -c, --commit string   Open the commit page for the given hash
      --commits         Open the commits page
  -n, --no-browser      Print destination URL instead of opening the browser
      --pipelines       Open the pipelines page
  -p, --pull-requests   Open the pull requests page
  -R, --repo string     Select another repository using the WORKSPACE/REPO format
  -s, --settings        Open repository settings
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb](bb.md)	 - Bitbucket Cloud CLI

