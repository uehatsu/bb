## bb repo view

View a repository

### Synopsis

Display the description and README of a repository.

```
bb repo view [<repository>] [flags]
```

### Options

```
  -b, --branch string     View a specific branch of the repository
  -q, --jq string         Filter JSON output using a jq expression
      --json strings      Output JSON with the specified fields (comma-separated)
  -R, --repo string       Select another repository using the WORKSPACE/REPO format
  -t, --template string   Format JSON output using a Go template
  -w, --web               Open the repository in the browser
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb repo](bb_repo.md)	 - Manage repositories

