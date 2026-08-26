## bb auth

Authenticate bb with Bitbucket Cloud

### Synopsis

Manage authentication state for Bitbucket Cloud.

bb uses Atlassian API tokens (with scopes). App passwords were removed by
Atlassian in July 2026 and are not supported.

```
bb auth <command> [flags]
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb](bb.md)	 - Bitbucket Cloud CLI
* [bb auth login](bb_auth_login.md)	 - Log in to Bitbucket Cloud with an API token
* [bb auth logout](bb_auth_logout.md)	 - Remove stored Bitbucket credentials
* [bb auth refresh](bb_auth_refresh.md)	 - Refresh the stored OAuth access token
* [bb auth setup-git](bb_auth_setup-git.md)	 - Configure git to use bb as a credential helper for bitbucket.org
* [bb auth status](bb_auth_status.md)	 - Show authentication status
* [bb auth token](bb_auth_token.md)	 - Print the authentication token bb is configured to use

