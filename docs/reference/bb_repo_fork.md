## bb repo fork

Create a fork of a repository

### Synopsis

Create a fork of a repository. The fork is created in --workspace (or the
configured default workspace). Use --name to rename the fork, which is
required when forking into the same workspace.

```
bb repo fork [<repository>] [flags]
```

### Options

```
      --clone              Clone the fork after creating it
      --name string        Name for the fork
  -R, --repo string        Select another repository using the WORKSPACE/REPO format
  -w, --workspace string   Workspace to create the fork in
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb repo](bb_repo.md)	 - Manage repositories

