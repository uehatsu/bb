## bb repo delete

Delete a repository

### Synopsis

Delete a Bitbucket repository. This cannot be undone.

Without --yes you must type the repository's full name to confirm. The token
needs the delete:repository:bitbucket scope.

```
bb repo delete [<repository>] [flags]
```

### Options

```
  -R, --repo string   Select another repository using the WORKSPACE/REPO format
      --yes           Confirm deletion without prompting
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb repo](bb_repo.md)	 - Manage repositories

