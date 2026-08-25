## bb project list

List projects in a workspace

```
bb project list [flags]
```

### Options

```
  -q, --jq string         Filter JSON output using a jq expression
      --json strings      Output JSON with the specified fields (comma-separated)
  -L, --limit int         Maximum number of projects to list (default 50)
  -t, --template string   Format JSON output using a Go template
```

### Options inherited from parent commands

```
      --help               Show help for command
  -w, --workspace string   Workspace (default: configured workspace or current repository's)
```

### SEE ALSO

* [bb project](bb_project.md)	 - Manage projects in a workspace

