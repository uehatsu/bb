package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// AuthMethod identifies how a token is presented to the API.
type AuthMethod string

const (
	// AuthAPIToken is an Atlassian API token: Basic auth with email:token.
	AuthAPIToken AuthMethod = "api_token"
	// AuthBearer is a repository/project/workspace access token (or OAuth
	// access token) sent as a Bearer token.
	AuthBearer AuthMethod = "bearer"
)

// Credential holds an authentication secret for a host.
type Credential struct {
	Method    AuthMethod `yaml:"method"`
	Email     string     `yaml:"email,omitempty"`
	Token     string     `yaml:"token"`
	User      string     `yaml:"user,omitempty"`       // Bitbucket username / nickname
	ExpiresAt *time.Time `yaml:"expires_at,omitempty"` // user supplied, optional
}

// GitUsername returns the fixed username git should send with the token.
func (c Credential) GitUsername() string {
	if c.Method == AuthBearer {
		return "x-token-auth"
	}
	return "x-bitbucket-api-token-auth"
}

// IsExpired reports whether ExpiresAt is set and in the past.
func (c Credential) IsExpired(now time.Time) bool {
	return c.ExpiresAt != nil && now.After(*c.ExpiresAt)
}

// ExpiresWithin reports whether ExpiresAt is set and within d of now.
func (c Credential) ExpiresWithin(now time.Time, d time.Duration) bool {
	return c.ExpiresAt != nil && !c.IsExpired(now) && c.ExpiresAt.Sub(now) <= d
}

// ErrNotFound is returned when no credential exists for the host.
var ErrNotFound = errors.New("no credential found")

// CredentialStore abstracts credential persistence so a keyring backend can
// replace the file backend later.
type CredentialStore interface {
	Get(host string) (Credential, error)
	Set(host string, cred Credential) error
	Delete(host string) error
}

// FileCredentialStore stores credentials in hosts.yml (mode 0600).
type FileCredentialStore struct{ dir string }

// NewFileCredentialStore creates a file-backed store in dir.
func NewFileCredentialStore(dir string) *FileCredentialStore {
	return &FileCredentialStore{dir: dir}
}

func (s *FileCredentialStore) path() string { return filepath.Join(s.dir, "hosts.yml") }

func (s *FileCredentialStore) load() (map[string]Credential, error) {
	data, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Credential{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading hosts.yml: %w", err)
	}
	var m map[string]Credential
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing hosts.yml: %w", err)
	}
	if m == nil {
		m = map[string]Credential{}
	}
	return m, nil
}

func (s *FileCredentialStore) save(m map[string]Credential) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return writeFileAtomic(s.path(), data, 0o600)
}

// Get implements CredentialStore.
func (s *FileCredentialStore) Get(host string) (Credential, error) {
	m, err := s.load()
	if err != nil {
		return Credential{}, err
	}
	c, ok := m[host]
	if !ok || c.Token == "" {
		return Credential{}, ErrNotFound
	}
	if c.Method == "" {
		c.Method = AuthAPIToken
	}
	return c, nil
}

// Set implements CredentialStore.
func (s *FileCredentialStore) Set(host string, cred Credential) error {
	m, err := s.load()
	if err != nil {
		return err
	}
	m[host] = cred
	return s.save(m)
}

// Delete implements CredentialStore.
func (s *FileCredentialStore) Delete(host string) error {
	m, err := s.load()
	if err != nil {
		return err
	}
	delete(m, host)
	return s.save(m)
}

// EnvCredential builds a Credential from BB_TOKEN / BB_EMAIL /
// BB_AUTH_METHOD. Returns ok=false when BB_TOKEN is unset.
//
// Resolution when BB_AUTH_METHOD is empty: api_token if BB_EMAIL is set,
// bearer otherwise.
func EnvCredential(getenv func(string) string) (Credential, bool, error) {
	token := strings.TrimSpace(getenv("BB_TOKEN"))
	if token == "" {
		return Credential{}, false, nil
	}
	email := strings.TrimSpace(getenv("BB_EMAIL"))
	method := AuthMethod(strings.TrimSpace(getenv("BB_AUTH_METHOD")))
	switch method {
	case "":
		if email != "" {
			method = AuthAPIToken
		} else {
			method = AuthBearer
		}
	case AuthAPIToken, AuthBearer:
	default:
		return Credential{}, false, fmt.Errorf("invalid BB_AUTH_METHOD %q (api_token|bearer)", method)
	}
	if method == AuthAPIToken && email == "" {
		return Credential{}, false, errors.New("BB_EMAIL is required when BB_AUTH_METHOD=api_token")
	}
	return Credential{Method: method, Email: email, Token: token}, true, nil
}

// ResolveCredential returns the credential to use for host: environment
// variables take precedence over the store.
func ResolveCredential(store CredentialStore, host string, getenv func(string) string) (Credential, error) {
	if c, ok, err := EnvCredential(getenv); err != nil {
		return Credential{}, err
	} else if ok {
		return c, nil
	}
	return store.Get(host)
}
