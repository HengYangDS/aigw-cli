package codex

import (
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func projectionCatalog(slugs ...string) []byte {
	entries := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		entries = append(entries, fmt.Sprintf(`{"slug":%q}`, slug))
	}
	return []byte(`{"models":[` + strings.Join(entries, ",") + `]}`)
}

// emptyCatalogPlan reports that AIGW decided to own nothing for a target.
func emptyCatalogPlan(plan codexCatalogPlan) bool {
	return plan.path == "" && plan.data == nil && plan.state == "" && plan.client == ExecutableIdentity{}
}

func catalogSlugList(t *testing.T, data []byte) []string {
	t.Helper()
	var document struct {
		Models []map[string]json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse generated catalog: %v", err)
	}
	slugs := make([]string, 0, len(document.Models))
	for _, entry := range document.Models {
		var slug string
		if err := json.Unmarshal(entry["slug"], &slug); err != nil {
			t.Fatalf("parse generated slug: %v", err)
		}
		slugs = append(slugs, slug)
	}
	return slugs
}

// TestCodexCatalogProjectionReusesOnlyTheSameClient is the last-known-good gate.
// A copy describes exactly one build, so reuse after an upgrade would override
// the newer bundled table: a worse outcome than the fallback it prevents.
func TestCodexCatalogProjectionReusesOnlyTheSameClient(t *testing.T) {
	dir := t.TempDir()
	target := codexHomeTarget(filepath.Join(dir, "configuration.toml"))
	target.Executable = filepath.Join(dir, "codex")
	owned := []byte(`{"models":[{"slug":"openai.gpt-5.5"}]}` + "\n")
	before := transaction.FileSnapshot{Exists: true, Data: owned, SHA256: hashBytes(owned), Mode: 0o600}
	state := codexState{
		CatalogState:         catalogStateProjected,
		CatalogHash:          hashBytes(owned),
		CatalogClientVersion: "1.0.0",
		CatalogClientSHA256:  "aaaa",
	}
	original := codexBundledCatalog
	defer func() { codexBundledCatalog = original }()

	// Same identity, regeneration unavailable: the copy still describes the
	// installed build, so it is reused.
	codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
		return ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, nil, fmt.Errorf("dump failed")
	}
	plan := codexCatalogProjection(target, "openai.gpt-5.5", "", state, before)
	if plan.state != catalogStateProjected || string(plan.data) != string(owned) {
		t.Fatalf("same client: plan = %+v", plan)
	}

	// Upgraded client, regeneration unavailable: refuse the copy and report it.
	for _, live := range []ExecutableIdentity{
		{Version: "1.1.0", SHA256: "aaaa"},
		{Version: "1.0.0", SHA256: "bbbb"},
		{},
	} {
		codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
			return live, nil, fmt.Errorf("dump failed")
		}
		plan = codexCatalogProjection(target, "openai.gpt-5.5", "", state, before)
		if plan.state != catalogStateStale || plan.data != nil || plan.path != "" {
			t.Fatalf("changed client %+v: plan = %+v", live, plan)
		}
	}

	// Same identity but the file on disk is no longer the one AIGW recorded
	// writing: it is not AIGW's to reuse.
	codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
		return ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, nil, fmt.Errorf("dump failed")
	}
	edited := transaction.FileSnapshot{Exists: true, Data: []byte("{}"), SHA256: hashBytes([]byte("{}")), Mode: 0o600}
	if plan = codexCatalogProjection(target, "openai.gpt-5.5", "", state, edited); plan.state != catalogStateStale {
		t.Fatalf("edited catalog: plan = %+v", plan)
	}

	// Never owned a catalog here: nothing was lost, so nothing is reported.
	if plan = codexCatalogProjection(target, "openai.gpt-5.5", "", codexState{}, transaction.FileSnapshot{}); !emptyCatalogPlan(plan) {
		t.Fatalf("never owned: plan = %+v", plan)
	}
}

// TestCodexCatalogProjectionYieldsToUserAuthoredCatalog protects a setting AIGW
// does not own. The key replaces the bundled table, so adopting it would delete
// whatever the user put there.
func TestCodexCatalogProjectionYieldsToUserAuthoredCatalog(t *testing.T) {
	dir := t.TempDir()
	target := codexHomeTarget(filepath.Join(dir, "configuration.toml"))
	target.Executable = filepath.Join(dir, "codex")
	original := codexBundledCatalog
	defer func() { codexBundledCatalog = original }()
	codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
		return ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, projectionCatalog("gpt-5.5"), nil
	}
	base := "model_catalog_json = \"/home/user/own-catalog.json\"\n"
	if plan := codexCatalogProjection(target, "openai.gpt-5.5", base, codexState{}, transaction.FileSnapshot{}); !emptyCatalogPlan(plan) {
		t.Fatalf("user catalog was not respected: plan = %+v", plan)
	}
	if plan := codexCatalogProjection(target, "", "", codexState{}, transaction.FileSnapshot{}); !emptyCatalogPlan(plan) {
		t.Fatalf("empty model produced a catalog: plan = %+v", plan)
	}
}

func TestCodexCatalogDesiredSnapshotRemovesOnlyOwnedFiles(t *testing.T) {
	foreign := []byte(`{"models":[]}`)
	before := transaction.FileSnapshot{Exists: true, Data: foreign, SHA256: hashBytes(foreign), Mode: 0o600}
	got, err := codexCatalogDesiredSnapshot(codexCatalogPlan{}, before, hashBytes([]byte("other")))
	if err != nil || !got.Equal(before) {
		t.Fatalf("foreign catalog would be removed: %+v, err = %v", got, err)
	}
	if got, err = codexCatalogDesiredSnapshot(codexCatalogPlan{}, before, ""); err != nil || !got.Equal(before) {
		t.Fatalf("unowned catalog would be removed: %+v, err = %v", got, err)
	}
	if got, err = codexCatalogDesiredSnapshot(codexCatalogPlan{}, before, hashBytes(foreign)); err != nil || got.Exists {
		t.Fatalf("owned catalog was not removed: %+v, err = %v", got, err)
	}
	if got, err = codexCatalogDesiredSnapshot(codexCatalogPlan{data: []byte("x")}, transaction.FileSnapshot{}, ""); err != nil || got.Mode != 0o600 {
		t.Fatalf("new catalog mode = %v, err = %v", got.Mode.Perm(), err)
	}
}

// TestCodexCatalogDesiredSnapshotConvergesOwnedPermissions pins the mode as part
// of what AIGW owns: an owned catalog whose permissions drifted wider is written
// back owner-only rather than left as found.
func TestCodexCatalogDesiredSnapshotConvergesOwnedPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not represent owner-only permissions in a file mode")
	}
	owned := []byte(`{"models":[{"slug":"gpt-5.5"}]}`)
	for _, drifted := range []os.FileMode{0o640, 0o644, 0o664, 0o600} {
		before := transaction.FileSnapshot{Exists: true, Data: owned, SHA256: hashBytes(owned), Mode: drifted}
		got, err := codexCatalogDesiredSnapshot(codexCatalogPlan{path: "/tmp/catalog.json", data: []byte("x")}, before, hashBytes(owned))
		if err != nil {
			t.Fatalf("owned catalog at %v was rejected: %v", drifted, err)
		}
		if !got.Exists || got.Mode != 0o600 || string(got.Data) != "x" {
			t.Fatalf("owned catalog at %v did not converge: %+v", drifted, got)
		}
	}
}

// TestCodexCatalogDesiredSnapshotFailsClosedOnForeignFile pins that a file at
// the managed path which AIGW cannot prove is its own is neither overwritten nor
// re-permissioned. Adopting it would destroy content AIGW never wrote.
func TestCodexCatalogDesiredSnapshotFailsClosedOnForeignFile(t *testing.T) {
	foreign := []byte(`{"models":[{"slug":"someone-else"}]}`)
	plan := codexCatalogPlan{path: "/tmp/catalog.json", data: []byte("x")}
	for name, ownedHash := range map[string]string{
		"no recorded hash":    "",
		"hash of other bytes": hashBytes([]byte("other")),
	} {
		before := transaction.FileSnapshot{Exists: true, Data: foreign, SHA256: hashBytes(foreign), Mode: 0o644}
		got, err := codexCatalogDesiredSnapshot(plan, before, ownedHash)
		if err == nil {
			t.Fatalf("%s: foreign catalog was adopted: %+v", name, got)
		}
		if !strings.Contains(err.Error(), "/tmp/catalog.json") || !strings.Contains(err.Error(), "refusing to overwrite") {
			t.Fatalf("%s: error does not name the conflict: %v", name, err)
		}
		if got.Exists {
			t.Fatalf("%s: a desired state was produced anyway: %+v", name, got)
		}
	}
}

// catalogTestRuntime selects a provider-prefixed id, which is the reproduction
// this work exists for.
func catalogTestRuntime(model string) configuration.Runtime {
	runtime := atomicTestRuntime()
	runtime.Model = model
	return runtime
}

func stubCodexBundledCatalog(t *testing.T, client ExecutableIdentity, slugs ...string) {
	t.Helper()
	original := codexBundledCatalog
	t.Cleanup(func() { codexBundledCatalog = original })
	codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
		return client, projectionCatalog(slugs...), nil
	}
}

// writeCodexTestConfig returns the canonical path of a fresh configuration, the
// identity the projection transaction resolves targets to.
func writeCodexTestConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

func readCodexSidecar(t *testing.T, path string) codexState {
	t.Helper()
	data, err := os.ReadFile(codexStatePath(path))
	if err != nil {
		t.Fatal(err)
	}
	var state codexState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestCodexCatalogProjectionKeepsNamedProfilePolicy(t *testing.T) {
	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
	const original = "# User settings\n[profiles.user]\nmodel = 'user-model'\nmodel_catalog_json = 'user-catalog.json'\n"
	path := writeCodexTestConfig(t, original)
	target := codexHomeTarget(path)
	target.Executable = filepath.Join(filepath.Dir(path), "codex")
	runtime := catalogTestRuntime("openai.gpt-5.6-sol")
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, runtime); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Catalog  string `toml:"model_catalog_json"`
		Profiles map[string]struct {
			Model   string `toml:"model"`
			Catalog string `toml:"model_catalog_json"`
		} `toml:"profiles"`
	}
	if err := toml.Unmarshal(projected, &settings); err != nil {
		t.Fatal(err)
	}
	if settings.Catalog != codexCatalogPath(path) || settings.Profiles["user"].Model != "user-model" || settings.Profiles["user"].Catalog != "user-catalog.json" {
		t.Fatalf("root and named-profile catalogue ownership merged: %+v", settings)
	}
	if err := ValidateConfig(path, runtime); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileConfigs([]TargetRef{target}, nil, runtime); err != nil {
		t.Fatal(err)
	}
	if restored, err := os.ReadFile(path); err != nil || string(restored) != original {
		t.Fatalf("withdrawal changed named-profile policy: %v\n%s", err, restored)
	}
	if _, err := os.Stat(codexCatalogPath(path)); !os.IsNotExist(err) {
		t.Fatalf("owned catalogue remains after withdrawal: %v", err)
	}
}

func TestCodexTOMLStringRefusesUnrepresentablePaths(t *testing.T) {
	quoted, err := codexTOMLString(`/tmp/a"b\c.json`)
	if err != nil {
		t.Fatal(err)
	}
	if quoted != `"/tmp/a\"b\\c.json"` {
		t.Fatalf("codexTOMLString() = %s", quoted)
	}
	if _, err := codexTOMLString("/tmp/a\nb.json"); err == nil {
		t.Fatal("codexTOMLString() accepted a control character")
	}
	if _, err := codexTOMLString("/tmp/a\x7fb.json"); err == nil {
		t.Fatal("codexTOMLString() accepted a delete character")
	}
	if _, err := codexTOMLString(string([]byte{0xff})); err == nil {
		t.Fatal("codexTOMLString() accepted invalid UTF-8")
	}
}

func TestReadCodexBundledCatalogRequiresAnExecutable(t *testing.T) {
	for _, executable := range []string{"   ", filepath.Join(t.TempDir(), "absent")} {
		if _, _, err := ReadBundledCatalog(executable); err == nil {
			t.Fatalf("catalog accepted an unobservable executable: %q", executable)
		}
	}
}

func TestCatalogProbePreservesCleanupFailure(t *testing.T) {
	primary := errors.New("catalog probe failed")
	for _, failure := range []error{nil, primary} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			scratch := t.TempDir()
			for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(name, scratch)
			}
			remove := removeCatalogProbe
			t.Cleanup(func() { removeCatalogProbe = remove })
			var removed, home string
			removeCatalogProbe = func(path string) error {
				removed = path
				return &os.PathError{Op: "remove", Path: path, Err: os.ErrPermission}
			}
			err := withCatalogProbe(func(_ context.Context, path string) error {
				home = path
				return failure
			})
			if home == "" || home == scratch || filepath.Dir(home) != scratch || removed != home {
				t.Fatalf("probe ownership = %q, cleanup = %q, parent = %q", home, removed, scratch)
			}
			if failure != nil && !errors.Is(err, failure) {
				t.Fatalf("probe lost primary cause: %v", err)
			}
			if !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), home) {
				t.Fatalf("probe lost cleanup cause: %v", err)
			}
			if _, err := os.Stat(home); err != nil {
				t.Fatalf("injected cleanup unexpectedly removed probe home: %v", err)
			}
		})
	}
}
