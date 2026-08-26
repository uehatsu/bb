## bb repo

Manage repositories

### Synopsis

Work with Bitbucket repositories.

```
bb repo <command> [flags]
```

### Examples

```
  $ bb repo list acme
  $ bb repo view acme/widgets
  $ bb repo create widgets --workspace acme --private
  $ bb repo clone acme/widgets
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb](bb.md)	 - Bitbucket Cloud CLI
* [bb repo clone](bb_repo_clone.md)	 - Clone a repository locally
* [bb repo create](bb_repo_create.md)	 - Create a new repository
* [bb repo delete](bb_repo_delete.md)	 - Delete a repository
* [bb repo edit](bb_repo_edit.md)	 - Edit repository settings
* [bb repo fork](bb_repo_fork.md)	 - Create a fork of a repository
* [bb repo list](bb_repo_list.md)	 - List repositories in a workspace
* [bb repo view](bb_repo_view.md)	 - View a repository

