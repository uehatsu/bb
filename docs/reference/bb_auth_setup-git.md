## bb auth setup-git

Configure git to use bb as a credential helper for bitbucket.org

### Synopsis

Registers 'bb auth git-credential' as the git credential helper for
https://bitbucket.org only. An empty helper entry is written first so that
previously configured global helpers (osxkeychain, manager, ...) holding stale
app passwords are bypassed for this host.

```
bb auth setup-git [flags]
```

### Options

```
      --force   Overwrite an existing helper configuration
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb auth](bb_auth.md)	 - Authenticate bb with Bitbucket Cloud

