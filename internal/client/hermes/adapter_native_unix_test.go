//go:build native_hermes && !windows

package hermes

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The native boot-record resolver remains real; only home-maintenance steps are
// stubbed because the synthetic install must not maintain the installed client.
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
	home, cleanup, err := CreateVerificationHome(filepath.Join(owner, "config.yaml"))
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

func TestHermesNativeVerificationRetainsDependencyMembers(t *testing.T) {
	_, python := nativeHermesExecutables(t)
	osHome := t.TempDir()
	owner := filepath.Join(osHome, ".hermes")
	plugin := filepath.Join(owner, "plugins", "native-member")
	if err := os.MkdirAll(plugin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, "pyproject.toml"), []byte("[project]\nname = 'native-member'\nversion = '1.0.0'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(owner, "config.yaml")
	original := []byte("plugins: {enabled: [native-member]}\nproviders: {personal: {api_key: private-selection}}\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	profiles := filepath.Join(owner, "profiles")
	for _, name := range []string{"work", "retired", ".work.staging-hidden", "Bad Name"} {
		path := filepath.Join(profiles, name)
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		data := []byte("plugins: [invalid]\n")
		if name == "work" {
			data = []byte("plugins: {enabled: [native-member, disabled], disabled: [disabled]}\nmemory: {provider: memory-member}\n")
			for _, member := range []string{"native-member", "memory-member"} {
				memberRoot := filepath.Join(path, "plugins", member)
				if err := os.MkdirAll(memberRoot, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(memberRoot, "pyproject.toml"), []byte("[project]\nname = '"+member+"'\nversion = '1.0.0'\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := os.WriteFile(filepath.Join(path, "config.yaml"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(profiles, ".deleted", "retired"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(owner, filepath.Join(profiles, "default")); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"HOME": osHome, "USERPROFILE": osHome, "LOCALAPPDATA": osHome, "HERMES_HOME": owner}
	probe := `
import json
from pm.workspace import enabled_member_dirs, members_stamp
members = enabled_member_dirs()
print(json.dumps({"members": [str(p.resolve()) for p in members], "stamp": members_stamp(members)}))
`
	var before, after struct {
		Members []string `json:"members"`
		Stamp   string   `json:"stamp"`
	}
	if err := json.Unmarshal(runHermesNativeScript(t, python, osHome, env, probe), &before); err != nil {
		t.Fatal(err)
	}
	if len(before.Members) != 3 {
		t.Fatal("native dependency fixture did not select its complete live-profile union")
	}
	home, cleanup, err := CreateVerificationHome(target)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	})
	env["HERMES_HOME"] = home
	if err := json.Unmarshal(runHermesNativeScript(t, python, osHome, env, probe), &after); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before.Members, after.Members) || before.Stamp != after.Stamp {
		t.Fatalf("isolated verification changed native dependency identity: before=%+v after=%+v", before, after)
	}
	if actual, err := os.ReadFile(target); err != nil || !bytes.Equal(actual, original) {
		t.Fatal("dependency borrowing changed the operator configuration")
	}
}
