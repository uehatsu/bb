// Package build holds build-time metadata injected via -ldflags.
package build

// Version is the CLI version, set at build time with
// -ldflags "-X github.com/uehatsu/bb/internal/build.Version=v1.2.3".
var Version = "dev"

// Date is the build date (optional, set via -ldflags).
var Date = ""
