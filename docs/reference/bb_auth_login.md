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
account email. The token is stored in /Users/ueno/.config/bb/hosts.yml (mode 0600).

Use --with-token to read the token from standard input, e.g.
  echo "you@example.com:ATATT3x..." | bb auth login --with-token
  echo "ATATT3x..." | bb auth login --with-token --email you@example.com

Use --bearer for repository/project/workspace access tokens, which are sent as
Bearer tokens and do not need an email.

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
      --email string        Atlassian account email (for API tokens)
      --expires-in string   Token lifetime for expiry warnings, e.g. 90d, 1y (optional)
      --web                 Log in via OAuth in a browser (not yet supported)
      --with-token          Read token from standard input
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb auth](bb_auth.md)	 - Authenticate bb with Bitbucket Cloud

