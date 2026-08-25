## bb auth login

Log in to Bitbucket Cloud with an API token

### Synopsis

Authenticate with Bitbucket Cloud.

Create an API token with scopes at:
  https://id.atlassian.com/manage-profile/security/api-tokens
Choose "Create API token with scopes", select the Bitbucket app, and grant the
scopes bb needs (see 'bb auth login --help' output below). Copy the token; it is
shown only once.

By default the token is read interactively together with your Atlassian
account email. The token is stored in $XDG_CONFIG_HOME/bb/hosts.yml (mode 0600).

Use --with-token to read the token from standard input, e.g.
  echo "you@example.com:ATATT3x..." | bb auth login --with-token
  echo "ATATT3x..." | bb auth login --with-token --email you@example.com

Use --bearer for repository/project/workspace access tokens, which are sent as
Bearer tokens and do not need an email.

Use --web to log in through OAuth 2.0 in the browser. Bitbucket has no device
flow and requires a client secret, so you must first register an OAuth
consumer (workspace settings → OAuth consumers) with callback URL
http://127.0.0.1:8976/callback and the scopes you need. Provide the consumer
via --client-id / BB_OAUTH_CLIENT_ID and BB_OAUTH_CLIENT_SECRET (prompted when
unset). Access tokens expire after 2 hours and are refreshed automatically.

Recommended scopes:
  read:user:bitbucket            identify the logged in user (auth status)
  read:workspace:bitbucket       list workspaces and members
  read:project:bitbucket         list projects
  admin:project:bitbucket        create projects (optional)
  read:repository:bitbucket      list/view/clone repositories, branches, source
  write:repository:bitbucket     create branches, push
  admin:repository:bitbucket     create/edit repositories (optional)
  delete:repository:bitbucket    delete repositories (optional)
  read:pullrequest:bitbucket     list/view pull requests and comments
  write:pullrequest:bitbucket    create/approve/merge/decline pull requests
  read:pipeline:bitbucket        list/view pipelines and logs
  write:pipeline:bitbucket       run/stop pipelines


```
bb auth login [flags]
```

### Options

```
      --bearer              Token is a repository/project/workspace access token (Bearer)
      --client-id string    OAuth consumer key (or BB_OAUTH_CLIENT_ID / oauth_client_id config)
      --email string        Atlassian account email (for API tokens)
      --expires-in string   Token lifetime for expiry warnings, e.g. 90d, 1y (optional)
      --port int            Loopback port for the OAuth callback (default 8976 or oauth_port config)
      --web                 Log in via OAuth 2.0 in a browser (requires your own OAuth consumer)
      --with-token          Read token from standard input
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb auth](bb_auth.md)	 - Authenticate bb with Bitbucket Cloud

