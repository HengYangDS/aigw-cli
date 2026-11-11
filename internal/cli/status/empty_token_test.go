package status

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/secret"
)

// A client whose provider holds an empty Token is deferred with the fix that
// stores one, never configured, whether or not its files are in place.
func TestStatusReportsAnEmptyTokenAsDeferred(t *testing.T) {
	runtime, cfg, buffer := configuredReadinessRuntime(t)
	root := filepath.Join(t.TempDir(), "secrets")
	files, err := secret.Select(secret.Selection{Backend: "file", GOOS: goruntime.GOOS, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Set("one", "sk-one-0001"); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = files
	cfg.SetClientActivation(codex.ID, true, "codex", nil)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	inspectProjections(t, &runtime, everyProjectionReady)
	if state := collectStatus(runtime.Dependencies, cfg).Clients[codex.ID]; state.State != domainreadiness.Configured {
		t.Fatalf("a stored Token = %#v", state)
	}
	slot := filepath.Join(root, "tokens", "one")
	if _, err := os.Stat(slot); err != nil {
		slot = filepath.Join(root, "one")
	}
	if err := os.WriteFile(slot, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	state := collectStatus(runtime.Dependencies, cfg).Clients[codex.ID]
	if state.State != domainreadiness.Deferred || state.NextAction != "run `aigw provider rotate one`" {
		t.Fatalf("an empty Token = %#v", state)
	}
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if text := buffer.String(); strings.Contains(text, "Configured") {
		t.Fatalf("status reports an empty Token as configured:\n%s", text)
	}
}
