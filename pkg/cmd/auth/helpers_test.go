package auth

import (
	"os"
	"testing"
)

func mustEnv(t *testing.T, k string) string {
	t.Helper()
	v := os.Getenv(k)
	if v == "" {
		t.Fatalf("%s not set", k)
	}
	return v
}
