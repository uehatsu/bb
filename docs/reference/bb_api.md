## bb api

Make an authenticated Bitbucket API request

### Synopsis

Makes an authenticated HTTP request to the Bitbucket Cloud REST API 2.0 and
prints the response.

The endpoint is a path relative to https://api.bitbucket.org/2.0, e.g.
"/user" or "repositories/{workspace}/{repo_slug}". The placeholders
{workspace} and {repo_slug} (or {repo}) are replaced with the current
repository (from --repo, BB_REPO, or git remotes).

The default method is GET, or POST when any fields are given. Use -f/--raw-field
for string values and -F/--field for typed values: "true", "false", "null",
integers are converted; "@file" reads the value from a file ("@-" for stdin).
For GET requests fields become query parameters; otherwise they are sent as a
JSON body.

With --paginate, all pages are fetched by following "next" links and the
"values" arrays are concatenated into a single JSON array.

```
bb api <endpoint> [flags]
```

### Examples

```
  $ bb api /user
  $ bb api repositories/{workspace}/{repo_slug}/pullrequests --paginate --jq '.[].title'
  $ bb api repositories/{workspace}/{repo_slug}/refs/branches -f name=feat -F 'target[hash]=abc123'
  $ bb api -X PUT repositories/acme/widgets -f description="New description"
  $ bb api --input body.json -X POST repositories/acme/widgets/pullrequests
```

### Options

```
  -F, --field stringArray       Add a typed parameter in key=value format
  -H, --header stringArray      Add a HTTP request header in key:value format
  -i, --include                 Include HTTP response status line and headers in the output
      --input string            The file to use as body for the HTTP request (use "-" for stdin)
  -q, --jq string               Query to select values from the response using jq syntax
  -X, --method string           The HTTP method for the request (default GET, or POST with fields)
      --paginate                Fetch all pages of results, following next links
  -f, --raw-field stringArray   Add a string parameter in key=value format
  -R, --repo string             Select another repository using the WORKSPACE/REPO format
      --silent                  Do not print the response body
  -t, --template string         Format JSON output using a Go template
```

### Options inherited from parent commands

```
      --help   Show help for command
```

### SEE ALSO

* [bb](bb.md)	 - Bitbucket Cloud CLI

