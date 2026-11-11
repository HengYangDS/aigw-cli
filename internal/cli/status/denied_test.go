package status

import (
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"aigw-cli/internal/adapter/codex"
	domainreadiness "aigw-cli/internal/app/readiness"
)

// A sandbox that denies the read of AIGW's Token folder gets the generic
// denial fix naming the folder in status, as in check.
func TestStatusNamesADeniedTokenFolder(t *testing.T) {
	runtime, cfg, buffer := configuredReadinessRuntime(t)
	home := t.TempDir()
	runtime.Host.Home = home
	tokens := filepath.Join(home, "aigw", "secrets", "tokens")
	runtime.Secrets = &observingSecretStore{existsErr: &fs.PathError{Op: "lstat", Path: tokens, Err: syscall.EPERM}}
	state := collectStatus(runtime.Dependencies, cfg).Clients[codex.ID]
	if state.State != domainreadiness.Unavailable || !strings.Contains(state.Detail, "~/aigw/secrets/tokens cannot be read") ||
		!strings.Contains(state.NextAction, "forbids this operation on ~/aigw/secrets/tokens") {
		t.Fatalf("state = %#v", state)
	}
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if text := buffer.String(); strings.Contains(text, "unlock") || strings.Contains(text, home) {
		t.Fatalf("status names an unlock or the absolute home:\n%s", text)
	}
}
