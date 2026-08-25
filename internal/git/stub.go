package git

import (
	"context"
	"fmt"
	"strings"
)

// Stub is a scripted Runner for tests. Every invocation is recorded in
// Calls (as "git <args>"). Outputs maps an exact argument string to the
// stdout it should return; Errors maps one to an error.
type Stub struct {
	Calls    []string
	Outputs  map[string]string
	Errors   map[string]error
	Branch   string
	RemoteVs []Remote
	Dir      string
}

// NewStub returns an empty Stub.
func NewStub() *Stub {
	return &Stub{Outputs: map[string]string{}, Errors: map[string]error{}}
}

func (s *Stub) record(args []string) string {
	key := strings.Join(args, " ")
	s.Calls = append(s.Calls, "git "+key)
	return key
}

func joinNonEmpty(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}

// Output implements Runner.
func (s *Stub) Output(_ context.Context, args ...string) (string, error) {
	key := s.record(args)
	if err, ok := s.Errors[key]; ok {
		return "", err
	}
	return s.Outputs[key], nil
}

// Run implements Runner.
func (s *Stub) Run(_ context.Context, args ...string) error {
	key := s.record(args)
	return s.Errors[key]
}

// Remotes implements Runner.
func (s *Stub) Remotes(context.Context) ([]Remote, error) { return s.RemoteVs, nil }

// CurrentBranch implements Runner.
func (s *Stub) CurrentBranch(context.Context) (string, error) {
	if s.Branch == "" {
		return "", fmt.Errorf("not on any branch (detached HEAD)")
	}
	return s.Branch, nil
}

// ConfigGet implements Runner.
func (s *Stub) ConfigGet(_ context.Context, scope, key string) (string, error) {
	k := joinNonEmpty("config", scope, "--get", key)
	s.Calls = append(s.Calls, "git "+k)
	return s.Outputs[k], nil
}

// ConfigSet implements Runner.
func (s *Stub) ConfigSet(_ context.Context, scope, key, value string) error {
	s.Calls = append(s.Calls, joinNonEmpty("git config", scope, "--replace-all", key, value))
	return nil
}

// ConfigAdd implements Runner.
func (s *Stub) ConfigAdd(_ context.Context, scope, key, value string) error {
	s.Calls = append(s.Calls, joinNonEmpty("git config", scope, "--add", key, value))
	return nil
}

// ConfigUnsetAll implements Runner.
func (s *Stub) ConfigUnsetAll(_ context.Context, scope, key string) error {
	s.Calls = append(s.Calls, joinNonEmpty("git config", scope, "--unset-all", key))
	return nil
}

// InDir implements Runner (records the directory switch, shares state).
func (s *Stub) InDir(dir string) Runner {
	s.Calls = append(s.Calls, "cd "+dir)
	return s
}
