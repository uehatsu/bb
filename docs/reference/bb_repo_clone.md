## bb repo clone

Clone a repository locally

### Synopsis

Clone a Bitbucket repository locally. Pass additional git clone flags
after "--".

The protocol is taken from the git_protocol config (https by default). SSH
clones use ssh.bitbucket.org. When cloning a fork, an "upstream" remote
pointing at the parent repository is added.

```
bb repo clone <repository> [<directory>] [-- <gitflags>...] [flags]
```

### Examples

```
  $ bb repo clone acme/widgets
  $ bb repo clone widgets            # uses the default workspace
  $ bb repo clone acme/widgets mydir -- --depth 1
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb repo](bb_repo.md)	 - Manage repositories

