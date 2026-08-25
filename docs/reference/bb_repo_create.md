## bb repo create

Create a new repository

### Synopsis

Create a new repository in a workspace.

The name may be given as WORKSPACE/NAME or NAME (with --workspace or the
configured default workspace). Every Bitbucket repository belongs to a
project; when --project is omitted, Bitbucket assigns the repository to the
workspace's oldest project.

```
bb repo create [<name>] [flags]
```

### Examples

```
  $ bb repo create widgets --workspace acme --private --project PROJ
  $ bb repo create acme/widgets --public --clone
```

### Options

```
  -c, --clone                Clone the new repository to the current directory
  -d, --description string   Description of the repository
      --fork-policy string   Fork policy: {allow_forks|no_public_forks|no_forks}
      --language string      Primary programming language
      --private              Make the new repository private
      --project string       Project key to place the repository in
      --public               Make the new repository public
  -w, --workspace string     Workspace to create the repository in
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb repo](bb_repo.md)	 - Manage repositories

