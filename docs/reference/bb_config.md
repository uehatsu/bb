## bb config

Manage configuration for bb

### Synopsis

Display or change configuration settings for bb.

Configuration is stored in $XDG_CONFIG_HOME/bb/config.yml.

Keys:
  workspace       default workspace for repository names without a workspace
  git_protocol    protocol for clone/checkout: https (default) | ssh
  merge_strategy  default for 'bb pr merge': merge_commit (default) | squash |
                  fast_forward | squash_fast_forward | rebase_fast_forward | rebase_merge
  editor          editor for multi-line prompts
  pager           pager for long output (e.g. "less -R")
  browser         browser command for --web
  prompt          enabled (default) | disabled
  credential_store  file (default) | keyring   (where tokens are stored)
  oauth_client_id   OAuth consumer key for 'bb auth login --web'
  oauth_port        loopback port for the OAuth callback (default 8976)

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb](bb.md)	 - Bitbucket Cloud CLI
* [bb config get](bb_config_get.md)	 - Print the value of a configuration key
* [bb config list](bb_config_list.md)	 - Print a list of configuration keys and values
* [bb config set](bb_config_set.md)	 - Update a configuration value

