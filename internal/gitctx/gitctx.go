// Package gitctx parses Bitbucket remote URLs. It is read-only: anything
// that mutates a repository lives in internal/git.
package gitctx

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Hosts accepted as Bitbucket Cloud. ssh.bitbucket.org is the SSH endpoint
// that replaces bitbucket.org for SSH traffic from November 2026.
var bitbucketHosts = map[string]bool{
	"bitbucket.org":        true,
	"www.bitbucket.org":    true,
	"ssh.bitbucket.org":    true,
	"altssh.bitbucket.org": true,
}

// SSHHost is the host to use when generating SSH clone URLs.
const SSHHost = "ssh.bitbucket.org"

// Repo is a parsed workspace/slug pair.
type Repo struct {
	Workspace string
	Slug      string
}

// FullName returns "workspace/slug".
func (r Repo) FullName() string { return r.Workspace + "/" + r.Slug }

var scpLike = regexp.MustCompile(`^(?:([^@]+)@)?([^:/]+):(.+)$`)

// ParseRemoteURL extracts workspace/slug from any Bitbucket remote URL form:
//
//	https://bitbucket.org/ws/repo.git
//	https://user@bitbucket.org/ws/repo
//	git@bitbucket.org:ws/repo.git
//	git@ssh.bitbucket.org:ws/repo.git
//	ssh://git@ssh.bitbucket.org/ws/repo.git
//	ssh://git@altssh.bitbucket.org:443/ws/repo.git
//
// It returns ok=false for non-Bitbucket remotes.
func ParseRemoteURL(raw string) (Repo, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Repo{}, false
	}
	var host, path string
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return Repo{}, false
		}
		host, path = u.Hostname(), u.Path
	} else if m := scpLike.FindStringSubmatch(raw); m != nil {
		host, path = m[2], m[3]
	} else {
		return Repo{}, false
	}
	if !bitbucketHosts[strings.ToLower(host)] {
		return Repo{}, false
	}
	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || !ValidName(parts[0]) || !ValidName(parts[1]) {
		return Repo{}, false
	}
	return Repo{Workspace: parts[0], Slug: parts[1]}, true
}

var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$|^\{[0-9a-fA-F-]{36}\}$`)

// ValidName reports whether s is a plausible workspace or repository
// identifier (slug or {uuid}). Used before building URLs.
func ValidName(s string) bool { return namePattern.MatchString(s) }

// ParseRepoArg parses "workspace/slug" (or a full URL). When only "slug" is
// given, defaultWorkspace is used.
func ParseRepoArg(arg, defaultWorkspace string) (Repo, error) {
	if r, ok := ParseRemoteURL(arg); ok {
		return r, nil
	}
	parts := strings.Split(strings.Trim(arg, "/"), "/")
	var r Repo
	switch len(parts) {
	case 1:
		if defaultWorkspace == "" {
			return Repo{}, fmt.Errorf("repository %q must be in WORKSPACE/REPO form (or set a default workspace with `bb config set workspace <slug>`)", arg)
		}
		r = Repo{Workspace: defaultWorkspace, Slug: parts[0]}
	case 2:
		r = Repo{Workspace: parts[0], Slug: parts[1]}
	default:
		return Repo{}, fmt.Errorf("invalid repository %q: expected WORKSPACE/REPO", arg)
	}
	if !ValidName(r.Workspace) || !ValidName(r.Slug) {
		return Repo{}, fmt.Errorf("invalid repository %q: expected WORKSPACE/REPO", arg)
	}
	return r, nil
}

// ErrNoRemote is returned when no Bitbucket remote could be found.
var ErrNoRemote = errors.New("no Bitbucket remote found; use --repo WORKSPACE/REPO or set BB_REPO")

// Remote pairs a git remote name with its parsed repo.
type Remote struct {
	Name string
	Repo Repo
}

// PickRemote chooses the best Bitbucket remote: upstream > origin > first.
func PickRemote(remotes []Remote) (Remote, error) {
	if len(remotes) == 0 {
		return Remote{}, ErrNoRemote
	}
	for _, want := range []string{"upstream", "origin"} {
		for _, r := range remotes {
			if r.Name == want {
				return r, nil
			}
		}
	}
	return remotes[0], nil
}

// CloneURL builds a canonical clone URL for repo. SSH URLs use SSHHost.
func CloneURL(r Repo, protocol string) string {
	if protocol == "ssh" {
		return fmt.Sprintf("git@%s:%s/%s.git", SSHHost, r.Workspace, r.Slug)
	}
	return fmt.Sprintf("https://bitbucket.org/%s/%s.git", r.Workspace, r.Slug)
}

// NormalizeCloneURL validates that href points at Bitbucket and rewrites it
// to the canonical form for protocol (dropping embedded usernames and moving
// SSH to SSHHost). It returns ok=false for non-Bitbucket URLs.
func NormalizeCloneURL(href, protocol string) (string, bool) {
	r, ok := ParseRemoteURL(href)
	if !ok || !ValidName(r.Workspace) || !ValidName(r.Slug) {
		return "", false
	}
	return CloneURL(r, protocol), true
}
