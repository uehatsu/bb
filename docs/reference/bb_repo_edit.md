## bb repo edit

Edit repository settings

```
bb repo edit [<repository>] [flags]
```

### Examples

```
  $ bb repo edit --description "New description"
  $ bb repo edit acme/widgets --visibility private --default-branch main
```

### Options

```
      --default-branch string   Set the default (main) branch
  -d, --description string      Description of the repository
      --fork-policy string      Fork policy: {allow_forks|no_public_forks|no_forks}
      --language string         Primary programming language
      --name string             Rename the repository (changes the slug)
  -R, --repo string             Select another repository using the WORKSPACE/REPO format
      --visibility string       Change visibility: {public|private}
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb repo](bb_repo.md)	 - Manage repositories

