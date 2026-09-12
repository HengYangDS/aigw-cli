package codex

import (
	"aigw-cli/internal/transaction"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestReconcileConfigsProjectsAndWithdrawsTheModelCatalog is the end-to-end
// lifecycle: project, converge, validate, then withdraw without a trace.
func TestReconcileConfigsProjectsAndWithdrawsTheModelCatalog(t *testing.T) {
	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol", "gpt-5.5")
	path := writeCodexTestConfig(t, "model_provider = \"native\"\nuser_setting = true\n")
	target := codexHomeTarget(path)
	target.Executable = filepath.Join(filepath.Dir(path), "codex")
	runtimeConfig := catalogTestRuntime("openai.gpt-5.6-sol")

	if _, err := ReconcileConfigs(nil, []TargetRef{target}, runtimeConfig); err != nil {
		t.Fatal(err)
	}
	catalogPath := codexCatalogPath(path)
	catalog, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("catalog was not written: %v", err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	quotedCatalogPath, err := codexTOMLString(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(projected), "model_catalog_json = "+quotedCatalogPath+" # managed by AIGW") {
		t.Fatalf("config does not reference the catalog:\n%s", projected)
	}
	// The wire id keeps its provider prefix: it is the routing identifier, and a
	// client-side metadata gap is not a reason to rewrite it.
	if !strings.Contains(string(projected), `model = "openai.gpt-5.6-sol" # managed by AIGW`) {
		t.Fatalf("config does not keep the prefixed model id:\n%s", projected)
	}
	if slugs := catalogSlugList(t, catalog); len(slugs) != 4 {
		t.Fatalf("catalog slugs = %v", slugs)
	}
	state := readCodexSidecar(t, path)
	if state.CatalogState != catalogStateProjected || state.CatalogHash != hashBytes(catalog) {
		t.Fatalf("sidecar catalog attribution = %+v", state)
	}
	if state.CatalogClientVersion != "1.0.0" || state.CatalogClientSHA256 != "aaaa" {
		t.Fatalf("sidecar client identity = %+v", state)
	}
	if err := ValidateConfig(path, runtimeConfig); err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}

	plans, err := PlanReconciliation([]TargetRef{target}, []TargetRef{target}, runtimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Action != "already-converged" {
		t.Fatalf("second projection is not converged: %+v", plans)
	}

	if _, err := ReconcileConfigs([]TargetRef{target}, nil, runtimeConfig); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(restored), "model_catalog_json") {
		t.Fatalf("restored config still references a catalog:\n%s", restored)
	}
	if !strings.Contains(string(restored), "user_setting = true") {
		t.Fatalf("restore lost a user setting:\n%s", restored)
	}
	if _, err := os.Stat(catalogPath); !os.IsNotExist(err) {
		t.Fatalf("catalog remains after restore: %v", err)
	}
}

// TestReconcileConfigsLeavesBareModelSelectionsAlone is the no-regression guard
// for every profile whose id the client already knows.
func TestReconcileConfigsLeavesBareModelSelectionsAlone(t *testing.T) {
	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol", "gpt-5.5")
	path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
	target := codexHomeTarget(path)
	target.Executable = filepath.Join(filepath.Dir(path), "codex")
	runtimeConfig := catalogTestRuntime("gpt-5.6-sol")
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, runtimeConfig); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(projected), "model_catalog_json") {
		t.Fatalf("a bare model id was given a catalog:\n%s", projected)
	}
	if _, err := os.Stat(codexCatalogPath(path)); !os.IsNotExist(err) {
		t.Fatalf("catalog file exists for a bare model id: %v", err)
	}
	state := readCodexSidecar(t, path)
	if state.CatalogState != "" || state.CatalogHash != "" {
		t.Fatalf("sidecar records a catalog for a bare model id: %+v", state)
	}
	if err := ValidateConfig(path, runtimeConfig); err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
}

// TestReconcileConfigsRebuildsTheCatalogAfterAClientUpgrade covers the upgrade
// path in both directions: a newer client rebuilds, and a newer client whose
// table cannot be read withdraws instead of reusing the old snapshot.
func TestReconcileConfigsRebuildsTheCatalogAfterAClientUpgrade(t *testing.T) {
	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
	path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
	target := codexHomeTarget(path)
	target.Executable = filepath.Join(filepath.Dir(path), "codex")
	runtimeConfig := catalogTestRuntime("openai.gpt-5.6-sol")
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, runtimeConfig); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(codexCatalogPath(path))
	if err != nil {
		t.Fatal(err)
	}

	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "2.0.0", SHA256: "bbbb"}, "gpt-5.6-sol", "gpt-6")
	if _, err := ReconcileConfigs([]TargetRef{target}, []TargetRef{target}, runtimeConfig); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(codexCatalogPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if string(second) == string(first) {
		t.Fatal("catalog was not rebuilt for the upgraded client")
	}
	if !strings.Contains(string(second), "openai.gpt-6") {
		t.Fatalf("rebuilt catalog lacks the new model:\n%s", second)
	}
	if state := readCodexSidecar(t, path); state.CatalogClientVersion != "2.0.0" || state.CatalogHash != hashBytes(second) {
		t.Fatalf("sidecar was not rebound to the installed client: %+v", state)
	}

	// The upgraded client's table becomes unreadable. The old snapshot describes
	// a build that is gone, so the reference is withdrawn and reported.
	original := codexBundledCatalog
	t.Cleanup(func() { codexBundledCatalog = original })
	codexBundledCatalog = func(string) (ExecutableIdentity, []byte, error) {
		return ExecutableIdentity{Version: "3.0.0", SHA256: "cccc"}, nil, fmt.Errorf("dump failed")
	}
	if _, err := ReconcileConfigs([]TargetRef{target}, []TargetRef{target}, runtimeConfig); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(projected), "model_catalog_json") {
		t.Fatalf("a stale catalog is still referenced:\n%s", projected)
	}
	if _, err := os.Stat(codexCatalogPath(path)); !os.IsNotExist(err) {
		t.Fatalf("stale catalog file remains: %v", err)
	}
	state := readCodexSidecar(t, path)
	if state.CatalogState != catalogStateStale || state.CatalogHash != "" {
		t.Fatalf("sidecar does not record the withdrawal: %+v", state)
	}
	err = ValidateConfig(path, runtimeConfig)
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("ValidateConfig() error = %v, want a stale report", err)
	}
}

// TestReconcileConfigsKeepsUserAuthoredCatalogSelections is the disable-and-switch
// case for a target AIGW must not adopt.
func TestReconcileConfigsKeepsUserAuthoredCatalogSelections(t *testing.T) {
	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
	own := "/home/user/own-catalog.json"
	path := writeCodexTestConfig(t, "model_provider = \"native\"\nmodel_catalog_json = \""+own+"\"\n")
	target := codexHomeTarget(path)
	target.Executable = filepath.Join(filepath.Dir(path), "codex")
	runtimeConfig := catalogTestRuntime("openai.gpt-5.6-sol")
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, runtimeConfig); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(projected), "model_catalog_json = \""+own+"\"\n") {
		t.Fatalf("user catalog selection was rewritten:\n%s", projected)
	}
	if state := readCodexSidecar(t, path); state.CatalogHash != "" || state.CatalogState != "" {
		t.Fatalf("AIGW claimed ownership of the user catalogue: %+v", state)
	}
	if _, err := os.Stat(codexCatalogPath(path)); !os.IsNotExist(err) {
		t.Fatalf("AIGW wrote a catalog beside a user-authored one: %v", err)
	}
	if _, err := ReconcileConfigs([]TargetRef{target}, nil, runtimeConfig); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(restored), "model_catalog_json = \""+own+"\"\n") {
		t.Fatalf("restore removed the user's catalog selection:\n%s", restored)
	}
}

// TestReconcileConfigsConvergesDriftedCatalogPermissions is the end-to-end guard
// for the mode contract. Writing the desired mode is not enough on its own: the
// ordinary atomic write inherits whatever mode the file on disk already has, so
// this pins that a widened owned catalog really is written back owner-only, and
// that `check` reports the drift while it lasts.
func TestReconcileConfigsConvergesDriftedCatalogPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not represent owner-only permissions in a file mode")
	}
	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
	path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
	target := codexHomeTarget(path)
	target.Executable = filepath.Join(filepath.Dir(path), "codex")
	runtimeConfig := catalogTestRuntime("openai.gpt-5.6-sol")
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, runtimeConfig); err != nil {
		t.Fatal(err)
	}
	catalogPath := codexCatalogPath(path)
	projected, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, drifted := range []os.FileMode{0o640, 0o644} {
		if err := os.Chmod(catalogPath, drifted); err != nil {
			t.Fatal(err)
		}
		err := ValidateConfig(path, runtimeConfig)
		if err == nil || !strings.Contains(err.Error(), "owner-only") {
			t.Fatalf("ValidateConfig() at %v error = %v, want an owner-only report", drifted, err)
		}
		plans, err := PlanReconciliation([]TargetRef{target}, []TargetRef{target}, runtimeConfig)
		if err != nil {
			t.Fatal(err)
		}
		if len(plans) != 1 || plans[0].Action == "already-converged" {
			t.Fatalf("drift at %v reads as converged: %+v", drifted, plans)
		}
		if _, err := ReconcileConfigs(nil, []TargetRef{target}, runtimeConfig); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(catalogPath)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("catalog stayed at %v after sync", info.Mode().Perm())
		}
		current, err := os.ReadFile(catalogPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(current, projected) {
			t.Fatalf("converging the mode changed the catalog contents")
		}
		if err := ValidateConfig(path, runtimeConfig); err != nil {
			t.Fatalf("ValidateConfig() after convergence error = %v", err)
		}
	}
}

// TestReconcileConfigsRefusesToAdoptAForeignCatalogFile pins the fail-closed
// direction: a file at the managed path that the sidecar cannot account for is
// somebody else's, and overwriting it would destroy content AIGW never wrote.
func TestReconcileConfigsRefusesToAdoptAForeignCatalogFile(t *testing.T) {
	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
	original := "model_provider = \"native\"\nuser_setting = true\n"
	path := writeCodexTestConfig(t, original)
	target := codexHomeTarget(path)
	target.Executable = filepath.Join(filepath.Dir(path), "codex")
	catalogPath := codexCatalogPath(path)
	foreign := []byte(`{"models":[{"slug":"not-ours"}]}`)
	if err := os.WriteFile(catalogPath, foreign, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReconcileConfigs(nil, []TargetRef{target}, catalogTestRuntime("openai.gpt-5.6-sol"))
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("ReconcileConfigs() error = %v, want a catalog ownership conflict", err)
	}
	current, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, foreign) {
		t.Fatalf("the foreign catalog was overwritten:\n%s", current)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Lstat(catalogPath)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("the foreign catalog was re-permissioned to %v", info.Mode().Perm())
		}
	}
	config, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(config) != original {
		t.Fatalf("the configuration was projected against a refused catalog:\n%s", config)
	}
}

// TestCodexArtifactsFollowTheCatalogDependencyOrder pins the crash-safe order.
// A configuration naming a catalog that does not exist yet stops the client from
// starting at all, so the file is created first and deleted last.
func TestCodexArtifactsFollowTheCatalogDependencyOrder(t *testing.T) {
	target := codexHomeTarget(filepath.Join(t.TempDir(), "configuration.toml"))
	configBefore := transaction.FileSnapshot{Exists: true, Data: []byte("old"), SHA256: hashBytes([]byte("old")), Mode: 0o600}
	catalogData := []byte("{}")

	created := codexArtifactsForDesiredState(
		target, configBefore, []byte("new"), transaction.FileSnapshot{}, []byte("state"),
		transaction.FileSnapshot{}, transaction.NewFileSnapshot(catalogData, 0o600),
	)
	wantCreate := []string{codexCatalogPath(target.Path), target.Path, codexStatePath(target.Path)}
	for index, artifact := range created {
		if index >= len(wantCreate) || artifact.path != wantCreate[index] {
			t.Fatalf("create order = %v, want %v", artifactPaths(created), wantCreate)
		}
	}
	if len(created) != len(wantCreate) {
		t.Fatalf("create order = %v, want %v", artifactPaths(created), wantCreate)
	}

	catalogBefore := transaction.NewFileSnapshot(catalogData, 0o600)
	removed := codexArtifactsForDesiredState(
		target, configBefore, []byte("new"), transaction.FileSnapshot{Exists: true, Data: []byte("state"), SHA256: hashBytes([]byte("state")), Mode: 0o600}, nil,
		catalogBefore, transaction.FileSnapshot{},
	)
	wantRemove := []string{target.Path, codexStatePath(target.Path), codexCatalogPath(target.Path)}
	if strings.Join(artifactPaths(removed), ",") != strings.Join(wantRemove, ",") {
		t.Fatalf("remove order = %v, want %v", artifactPaths(removed), wantRemove)
	}

	// An unchanged catalog is not an artifact at all, so a converged target
	// performs no writes.
	unchanged := codexArtifactsForDesiredState(
		target, configBefore, configBefore.Data, transaction.FileSnapshot{Exists: true, Data: []byte("state"), SHA256: hashBytes([]byte("state")), Mode: 0o600}, []byte("state"),
		catalogBefore, catalogBefore,
	)
	if len(unchanged) != 0 {
		t.Fatalf("converged target produced artifacts %v", artifactPaths(unchanged))
	}
}

func artifactPaths(artifacts []codexPreparedArtifact) []string {
	paths := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		paths = append(paths, artifact.path)
	}
	return paths
}

// TestReconcileConfigsRollsBackConfigStateAndCatalogTogether keeps the three
// artifacts one transaction: a partial projection would leave a configuration
// and a catalog that disagree about which models exist.
func TestReconcileConfigsRollsBackConfigStateAndCatalogTogether(t *testing.T) {
	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
	original := "model_provider = \"native\"\nuser_setting = true\n"
	path := writeCodexTestConfig(t, original)
	target := codexHomeTarget(path)
	target.Executable = filepath.Join(filepath.Dir(path), "codex")

	originalWrite := writeFileAtomicIfUnchanged
	originalExactWrite := writeFileAtomicExactModeIfUnchanged
	t.Cleanup(func() {
		writeFileAtomicIfUnchanged = originalWrite
		writeFileAtomicExactModeIfUnchanged = originalExactWrite
	})
	var written []string
	writeFileAtomicIfUnchanged = func(target string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		written = append(written, target)
		if strings.HasSuffix(target, ".aigw-state.json") {
			return transaction.FileSnapshot{}, fmt.Errorf("injected sidecar failure")
		}
		return originalWrite(target, expected, data, mode)
	}
	// The catalog is the one artifact written mode-exactly, so both writers have
	// to be observed for the assertion below to see it join the transaction.
	writeFileAtomicExactModeIfUnchanged = func(target string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		written = append(written, target)
		return originalExactWrite(target, expected, data, mode)
	}
	_, err := ReconcileConfigs(nil, []TargetRef{target}, catalogTestRuntime("openai.gpt-5.6-sol"))
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("ReconcileConfigs() error = %v", err)
	}
	// The catalog has to have been written for its absence below to mean it was
	// rolled back. Without this, a projection that left the catalog out of the
	// transaction entirely would satisfy every assertion that follows.
	if !slices.Contains(written, codexCatalogPath(path)) {
		t.Fatalf("the catalog never joined the transaction; writes = %v", written)
	}
	current, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(current) != original {
		t.Fatalf("config was not restored:\n%s", current)
	}
	if _, statErr := os.Stat(codexCatalogPath(path)); !os.IsNotExist(statErr) {
		t.Fatalf("catalog survived the rollback: %v", statErr)
	}
	if _, statErr := os.Stat(codexStatePath(path)); !os.IsNotExist(statErr) {
		t.Fatalf("sidecar survived the rollback: %v", statErr)
	}
}

func TestValidateConfigReportsCatalogDrift(t *testing.T) {
	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
	path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
	target := codexHomeTarget(path)
	target.Executable = filepath.Join(filepath.Dir(path), "codex")
	runtimeConfig := catalogTestRuntime("openai.gpt-5.6-sol")
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, runtimeConfig); err != nil {
		t.Fatal(err)
	}
	catalogPath := codexCatalogPath(path)
	if err := os.WriteFile(catalogPath, []byte(`{"models":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	err := ValidateConfig(path, runtimeConfig)
	if err == nil || !strings.Contains(err.Error(), "model catalog changed") {
		t.Fatalf("ValidateConfig() error = %v, want a catalog conflict", err)
	}
	if err := os.Remove(catalogPath); err != nil {
		t.Fatal(err)
	}
	err = ValidateConfig(path, runtimeConfig)
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("ValidateConfig() error = %v, want a missing catalog", err)
	}
	// A symlink at the managed path resolves to bytes AIGW may well have written,
	// so the check has to look at the path itself rather than what it points to.
	elsewhere := filepath.Join(t.TempDir(), "elsewhere.json")
	if err := os.WriteFile(elsewhere, []byte(`{"models":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, catalogPath); err != nil {
		t.Skipf("this platform does not allow symlinks here: %v", err)
	}
	err = ValidateConfig(path, runtimeConfig)
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("ValidateConfig() error = %v, want a regular-file report", err)
	}
}

// TestValidateConfigRejectsUnownedManagedCatalogLine catches a marker-bearing
// line the sidecar does not account for, which is the shape a foreign writer or
// a partially reverted projection leaves behind.
func TestValidateConfigRejectsUnownedManagedCatalogLine(t *testing.T) {
	stubCodexBundledCatalog(t, ExecutableIdentity{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
	path := writeCodexTestConfig(t, "model_provider = \"native\"\n")
	target := codexHomeTarget(path)
	target.Executable = filepath.Join(filepath.Dir(path), "codex")
	runtimeConfig := catalogTestRuntime("gpt-5.6-sol")
	if _, err := ReconcileConfigs(nil, []TargetRef{target}, runtimeConfig); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	injected := "model_catalog_json = \"/tmp/foreign.json\" # managed by AIGW\n" + string(current)
	if err := os.WriteFile(path, []byte(injected), 0o600); err != nil {
		t.Fatal(err)
	}
	err = ValidateConfig(path, runtimeConfig)
	if err == nil || !strings.Contains(err.Error(), "does not own") {
		t.Fatalf("ValidateConfig() error = %v, want an ownership report", err)
	}
}
