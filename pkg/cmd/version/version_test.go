package version

import "testing"

func TestFormat(t *testing.T) {
	if got := Format("v1.0.0", ""); got != "bb version v1.0.0\n" {
		t.Errorf("unexpected: %q", got)
	}
	if got := Format("v1.0.0", "2026-08-25"); got != "bb version v1.0.0 (2026-08-25)\n" {
		t.Errorf("unexpected: %q", got)
	}
}
