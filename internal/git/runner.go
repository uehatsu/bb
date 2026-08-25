package git

import "context"

// Runner is the interface commands depend on, so tests can inject a stub.
type Runner interface {
	Output(ctx context.Context, args ...string) (string, error)
	Run(ctx context.Context, args ...string) error
	Remotes(ctx context.Context) ([]Remote, error)
	CurrentBranch(ctx context.Context) (string, error)
	ConfigGet(ctx context.Context, scopeFlag, key string) (string, error)
	ConfigSet(ctx context.Context, scopeFlag, key, value string) error
	ConfigAdd(ctx context.Context, scopeFlag, key, value string) error
	ConfigUnsetAll(ctx context.Context, scopeFlag, key string) error
	// InDir returns a Runner operating in another working directory.
	InDir(dir string) Runner
}

// InDir implements Runner.
func (c *Client) InDir(dir string) Runner {
	cp := *c
	cp.Dir = dir
	return &cp
}

var _ Runner = (*Client)(nil)
