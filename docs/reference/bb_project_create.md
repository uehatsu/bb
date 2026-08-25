## bb project create

Create a project

### Synopsis

Create a project. Requires the admin:project:bitbucket scope.

```
bb project create <key> [flags]
```

### Examples

```
  $ bb project create PROJ --name "My Project" --private
  $ bb project create TOOLS --name Tools -w acme --public
```

### Options

```
  -d, --description string   Project description
  -n, --name string          Project name
      --private              Make the project private (default)
      --public               Make the project public
```

### Options inherited from parent commands

```
      --help               Show help for command
  -w, --workspace string   Workspace (default: configured workspace or current repository's)
```

### SEE ALSO

* [bb project](bb_project.md)	 - Manage projects in a workspace

