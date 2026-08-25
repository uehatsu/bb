package api

import (
	"net/http"

	"github.com/uehatsu/bb/internal/config"
)

// Authenticator applies credentials to an outgoing request. The Client is
// agnostic of the concrete scheme.
type Authenticator interface {
	Apply(req *http.Request)
}

// NewAuthenticator builds an Authenticator from a Credential.
func NewAuthenticator(c config.Credential) Authenticator {
	switch c.Method {
	case config.AuthBearer:
		return bearerAuth{token: c.Token}
	default:
		return basicAuth{user: c.Email, pass: c.Token}
	}
}

type basicAuth struct{ user, pass string }

func (a basicAuth) Apply(r *http.Request) { r.SetBasicAuth(a.user, a.pass) }

type bearerAuth struct{ token string }

func (a bearerAuth) Apply(r *http.Request) { r.Header.Set("Authorization", "Bearer "+a.token) }

// NoAuth performs no authentication.
type NoAuth struct{}

// Apply implements Authenticator.
func (NoAuth) Apply(*http.Request) {}
