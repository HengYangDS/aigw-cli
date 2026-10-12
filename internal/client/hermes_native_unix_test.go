//go:build native_hermes && !windows

package client

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hermesconfig "aigw-cli/internal/hermes/configuration"
)

// TestHermesNativeVerificationBootIsolation runs the installed Hermes boot
// maintenance in a verification home and proves its record stays private.
func TestHermesNativeVerificationBootIsolation(t *testing.T) {
	_, python := nativeHermesExecutables(t)
	osHome := t.TempDir()
	owner, project := filepath.Join(osHome, ".hermes"), filepath.Join(osHome, "payload")
	env := map[string]string{"HOME": osHome, "USERPROFILE": osHome, "LOCALAPPDATA": osHome, "HERMES_HOME": owner, "HERMES_BOOT_TEST_ROOT": project}
	output := runHermesNativeScript(t, python, osHome, env, `
import json, os
from pathlib import Path
from hermes_cli import boot_bootstrap
from pm.environments import install_state_dir
root = Path(os.environ["HERMES_BOOT_TEST_ROOT"])
root.mkdir()
(root / "install-stamp.json").write_text(json.dumps({"commit": "abcdef012345", "updateMechanism": "external", "distribution": "nix"}))
state = install_state_dir(root)
state.mkdir(parents=True)
(state / "facts.json").write_text("{}\n")
(state / ".install.lock").touch()
(state / "environments").mkdir()
record = boot_bootstrap.record_path(root)
record.parent.mkdir()
record.write_text(json.dumps({"schemaVersion": 1, "identity": "previous-revision"}))
print(json.dumps({"record": str(record)}))
`)
	var prepared struct {
		Record string `json:"record"`
	}
	if err := json.Unmarshal(output, &prepared); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(prepared.Record)
	if err != nil {
		t.Fatal(err)
	}
	home, cleanup, err := hermesconfig.CreateVerificationHome(filepath.Join(owner, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	})
	env["HERMES_HOME"], env["HERMES_DISABLE_LAZY_INSTALLS"] = home, "1"
	output = runHermesNativeScript(t, python, osHome, env, `
import json, os
from pathlib import Path
from hermes_cli import boot_bootstrap, post_update
root = Path(os.environ["HERMES_BOOT_TEST_ROOT"])
post_update.run_steps = lambda steps: {}
boot_bootstrap.run_boot_bootstrap(root)
record = boot_bootstrap.record_path(root)
print(json.dumps({"record": str(record), "identity": json.loads(record.read_text())["identity"]}))
`)
	var observed struct {
		Record   string `json:"record"`
		Identity string `json:"identity"`
	}
	if err := json.Unmarshal(output, &observed); err != nil {
		t.Fatal(err)
	}
	if observed.Identity != "abcdef012345" || !strings.HasPrefix(observed.Record, home+string(os.PathSeparator)) {
		t.Fatalf("verification wrote boot state outside its home: %+v", observed)
	}
	if actual, err := os.ReadFile(prepared.Record); err != nil || !bytes.Equal(actual, original) {
		t.Fatalf("operator boot record changed: %v", err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(observed.Record); !os.IsNotExist(err) {
		t.Fatalf("verification boot record survived cleanup: %v", err)
	}
}
