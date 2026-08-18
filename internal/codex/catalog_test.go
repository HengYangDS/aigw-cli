package codex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
)

// testBundledCatalog is a stand-in for the client's own table. It keeps the
// shape that matters here: several bare slugs and entries carrying fields this
// package does not model.
func testBundledCatalog(slugs ...string) []byte {
	entries := make([]string, 0, len(slugs))
	for index, slug := range slugs {
		entries = append(entries, fmt.Sprintf(
			`{"slug":%q,"display_name":"Display %d","context_window":400000,"unknown_future_field":{"nested":[1,2]}}`,
			slug, index,
		))
	}
	return []byte(`{"models":[` + strings.Join(entries, ",") + `]}`)
}

// emptyCatalogPlan reports that AIGW decided to own nothing for a target.
func emptyCatalogPlan(plan codexCatalogPlan) bool {
	return plan.path == "" && plan.data == nil && plan.state == "" && plan.client == codexClient{}
}

func catalogSlugList(t *testing.T, data []byte) []string {
	t.Helper()
	var document codexCatalogDocument
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

// TestBuildCodexCatalogAdaptsEveryProviderPrefix pins the general adaptation: a
// prefix is learned from the selected id and then applied to the whole table, so
// no model name is hard-coded and no model the account can select is left behind.
func TestBuildCodexCatalogAdaptsEveryProviderPrefix(t *testing.T) {
	bundled := testBundledCatalog("gpt-5.6-sol", "gpt-5.5", "codex-auto-review")
	cases := []struct {
		model     string
		namespace string
	}{
		{model: "openai.gpt-5.6-sol", namespace: "openai"},
		{model: "openai.gpt-5.5", namespace: "openai"},
		{model: "anthropic.codex-auto-review", namespace: "anthropic"},
		{model: "us.openai.gpt-5.5", namespace: "us.openai"},
		{model: "eu.anthropic.gpt-5.6-sol", namespace: "eu.anthropic"},
	}
	for _, c := range cases {
		data, err := buildCodexCatalog(bundled, c.model)
		if err != nil {
			t.Fatalf("buildCodexCatalog(%q) error = %v", c.model, err)
		}
		if data == nil {
			t.Fatalf("buildCodexCatalog(%q) generated no catalog", c.model)
		}
		got := catalogSlugList(t, data)
		want := []string{
			"gpt-5.6-sol", "gpt-5.5", "codex-auto-review",
			c.namespace + ".codex-auto-review", c.namespace + ".gpt-5.5", c.namespace + ".gpt-5.6-sol",
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("buildCodexCatalog(%q) slugs = %v, want %v", c.model, got, want)
		}
		if !strings.Contains(string(data), c.model) {
			t.Fatalf("buildCodexCatalog(%q) does not resolve the selected id", c.model)
		}
	}
}

// TestBuildCodexCatalogKeepsTheCompleteBundledTable is the incremental-snapshot
// guard. The client replaces its own table with this file, so a catalog holding
// only the alias would push every other model onto fallback metadata.
func TestBuildCodexCatalogKeepsTheCompleteBundledTable(t *testing.T) {
	bundled := testBundledCatalog("gpt-5.6-sol", "gpt-5.5")
	data, err := buildCodexCatalog(bundled, "openai.gpt-5.6-sol")
	if err != nil || data == nil {
		t.Fatalf("buildCodexCatalog() = %v, %v", data, err)
	}
	var source, generated codexCatalogDocument
	if err := json.Unmarshal(bundled, &source); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &generated); err != nil {
		t.Fatal(err)
	}
	if len(generated.Models) != len(source.Models)*2 {
		t.Fatalf("generated %d entries from %d bundled entries", len(generated.Models), len(source.Models))
	}
	aliases := make(map[string]map[string]json.RawMessage, len(source.Models))
	for _, entry := range generated.Models[len(source.Models):] {
		var slug string
		if err := json.Unmarshal(entry["slug"], &slug); err != nil {
			t.Fatal(err)
		}
		aliases[slug] = entry
	}
	for _, entry := range source.Models {
		var slug string
		if err := json.Unmarshal(entry["slug"], &slug); err != nil {
			t.Fatal(err)
		}
		alias, present := aliases["openai."+slug]
		if !present {
			t.Fatalf("no alias generated for %q", slug)
		}
		if len(alias) != len(entry) {
			t.Fatalf("alias %q has %d fields, source has %d", slug, len(alias), len(entry))
		}
		for key, value := range entry {
			if key == "slug" {
				continue
			}
			if string(alias[key]) != string(value) {
				t.Fatalf("alias %q field %q = %s, want %s", slug, key, alias[key], value)
			}
		}
	}
}

// TestBuildCodexCatalogWithholdsWhatItCannotProve is the anti-silencing guard:
// an id the client already knows needs nothing, and an id whose base slug does
// not exist keeps the client's own fallback and its warning.
func TestBuildCodexCatalogWithholdsWhatItCannotProve(t *testing.T) {
	bundled := testBundledCatalog("gpt-5.6-sol", "gpt-5.5")
	for _, model := range []string{
		"gpt-5.6-sol",
		"gpt-5.5",
		"openai.no-such-model",
		"no-such-model",
		"openai.",
		".gpt-5.6-sol",
		"",
	} {
		data, err := buildCodexCatalog(bundled, model)
		if err != nil {
			t.Fatalf("buildCodexCatalog(%q) error = %v", model, err)
		}
		if data != nil {
			t.Fatalf("buildCodexCatalog(%q) generated a catalog for an id it cannot prove", model)
		}
	}
}

// TestCodexCatalogNamespaceRequiresAUniqueMatch pins the prefix split against a
// first-dot shortcut: every dot-separated suffix is matched exactly, and an
// ambiguous id is refused rather than mapped to a guess.
func TestCodexCatalogNamespaceRequiresAUniqueMatch(t *testing.T) {
	// A slug that itself contains dots can make two different splits of the same
	// id look valid. Both are refused, because either one may be the wrong model.
	ambiguous := map[string]int{"b.c": 0, "c": 1}
	if namespace, ok := codexCatalogNamespace("a.b.c", ambiguous); ok {
		t.Fatalf("ambiguous id produced namespace %q", namespace)
	}
	slugs := map[string]int{"gpt-5.6-sol": 0, "plain": 1}
	namespace, ok := codexCatalogNamespace("openai.plain", slugs)
	if !ok || namespace != "openai" {
		t.Fatalf("codexCatalogNamespace() = %q, %v", namespace, ok)
	}
	// A dotted slug is matched whole rather than cut at the first dot.
	namespace, ok = codexCatalogNamespace("us.openai.gpt-5.6-sol", slugs)
	if !ok || namespace != "us.openai" {
		t.Fatalf("multi-level namespace = %q, %v", namespace, ok)
	}
	if _, ok = codexCatalogNamespace("openai.gpt-5", slugs); ok {
		t.Fatal("a partial slug produced a namespace")
	}
}

func TestBuildCodexCatalogIsDeterministicAndIdempotent(t *testing.T) {
	bundled := testBundledCatalog("gpt-5.5", "gpt-5.6-sol", "codex-auto-review")
	first, err := buildCodexCatalog(bundled, "openai.gpt-5.6-sol")
	if err != nil || first == nil {
		t.Fatalf("buildCodexCatalog() = %v, %v", first, err)
	}
	second, err := buildCodexCatalog(bundled, "openai.gpt-5.6-sol")
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("buildCodexCatalog() is not byte-deterministic")
	}
	// Re-running against a table that already carries the aliases must add
	// nothing, so a converged target keeps producing the same bytes.
	again, err := buildCodexCatalog(first, "openai.gpt-5.6-sol")
	if err != nil {
		t.Fatal(err)
	}
	if again != nil {
		t.Fatalf("buildCodexCatalog() duplicated existing aliases:\n%s", again)
	}
}

func TestBuildCodexCatalogRejectsUnusableBundledTables(t *testing.T) {
	cases := []struct {
		name    string
		bundled string
		message string
	}{
		{name: "not json", bundled: "{", message: "parse Codex bundled model catalog"},
		{name: "empty", bundled: `{"models":[]}`, message: "empty"},
		{name: "no slug", bundled: `{"models":[{"display_name":"x"}]}`, message: "has no slug"},
		{name: "empty slug", bundled: `{"models":[{"slug":""}]}`, message: "empty slug"},
		{name: "slug not a string", bundled: `{"models":[{"slug":7}]}`, message: "slug"},
		{name: "duplicate slug", bundled: `{"models":[{"slug":"a"},{"slug":"a"}]}`, message: "twice"},
	}
	for _, c := range cases {
		_, err := buildCodexCatalog([]byte(c.bundled), "openai.a")
		if err == nil || !strings.Contains(err.Error(), c.message) {
			t.Fatalf("%s: buildCodexCatalog() error = %v, want %q", c.name, err, c.message)
		}
	}
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
	codexBundledCatalog = func(string) (codexClient, []byte, error) {
		return codexClient{Version: "1.0.0", SHA256: "aaaa"}, nil, fmt.Errorf("dump failed")
	}
	plan := codexCatalogProjection(target, "openai.gpt-5.5", "", state, before)
	if plan.state != catalogStateProjected || string(plan.data) != string(owned) {
		t.Fatalf("same client: plan = %+v", plan)
	}

	// Upgraded client, regeneration unavailable: refuse the copy and report it.
	for _, live := range []codexClient{
		{Version: "1.1.0", SHA256: "aaaa"},
		{Version: "1.0.0", SHA256: "bbbb"},
		{},
	} {
		codexBundledCatalog = func(string) (codexClient, []byte, error) {
			return live, nil, fmt.Errorf("dump failed")
		}
		plan = codexCatalogProjection(target, "openai.gpt-5.5", "", state, before)
		if plan.state != catalogStateStale || plan.data != nil || plan.path != "" {
			t.Fatalf("changed client %+v: plan = %+v", live, plan)
		}
	}

	// Same identity but the file on disk is no longer the one AIGW recorded
	// writing: it is not AIGW's to reuse.
	codexBundledCatalog = func(string) (codexClient, []byte, error) {
		return codexClient{Version: "1.0.0", SHA256: "aaaa"}, nil, fmt.Errorf("dump failed")
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
	codexBundledCatalog = func(string) (codexClient, []byte, error) {
		return codexClient{Version: "1.0.0", SHA256: "aaaa"}, testBundledCatalog("gpt-5.5"), nil
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
	if err != nil || !sameCodexSnapshot(got, before) {
		t.Fatalf("foreign catalog would be removed: %+v, err = %v", got, err)
	}
	if got, err = codexCatalogDesiredSnapshot(codexCatalogPlan{}, before, ""); err != nil || !sameCodexSnapshot(got, before) {
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

func stubCodexBundledCatalog(t *testing.T, client codexClient, slugs ...string) {
	t.Helper()
	original := codexBundledCatalog
	t.Cleanup(func() { codexBundledCatalog = original })
	codexBundledCatalog = func(string) (codexClient, []byte, error) {
		return client, testBundledCatalog(slugs...), nil
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

// managedCatalogLines returns the catalog selections AIGW claims ownership of.
func managedCatalogLines(config string) []string {
	owned := make([]string, 0)
	for _, line := range modelCatalogLine.FindAllString(config, -1) {
		if strings.Contains(line, "# managed by AIGW") {
			owned = append(owned, line)
		}
	}
	return owned
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

// TestReconcileConfigsProjectsAndWithdrawsTheModelCatalog is the end-to-end
// lifecycle: project, converge, validate, then withdraw without a trace.
func TestReconcileConfigsProjectsAndWithdrawsTheModelCatalog(t *testing.T) {
	stubCodexBundledCatalog(t, codexClient{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol", "gpt-5.5")
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
	stubCodexBundledCatalog(t, codexClient{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol", "gpt-5.5")
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
	stubCodexBundledCatalog(t, codexClient{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
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

	stubCodexBundledCatalog(t, codexClient{Version: "2.0.0", SHA256: "bbbb"}, "gpt-5.6-sol", "gpt-6")
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
	codexBundledCatalog = func(string) (codexClient, []byte, error) {
		return codexClient{Version: "3.0.0", SHA256: "cccc"}, nil, fmt.Errorf("dump failed")
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
	stubCodexBundledCatalog(t, codexClient{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
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
	if owned := managedCatalogLines(string(projected)); len(owned) != 0 {
		t.Fatalf("AIGW claimed a catalog selection %v:\n%s", owned, projected)
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
	stubCodexBundledCatalog(t, codexClient{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
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
	stubCodexBundledCatalog(t, codexClient{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
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
		transaction.FileSnapshot{}, desiredCodexSnapshot(catalogData, 0o600),
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

	catalogBefore := desiredCodexSnapshot(catalogData, 0o600)
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
	stubCodexBundledCatalog(t, codexClient{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
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
	stubCodexBundledCatalog(t, codexClient{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
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
	stubCodexBundledCatalog(t, codexClient{Version: "1.0.0", SHA256: "aaaa"}, "gpt-5.6-sol")
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
}

// TestReadCodexBundledCatalogUsesAnIsolatedClientHome is what makes the
// generated catalog trustworthy: the table is read through an empty home, so a
// catalog AIGW itself projected can never feed the next generation.
func TestReadCodexBundledCatalogUsesAnIsolatedClientHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the probe script requires a POSIX shell")
	}
	dir := t.TempDir()
	executable := filepath.Join(dir, "codex")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then echo 'codex-cli 9.9.9'; exit 0; fi\n" +
		"if [ -z \"$CODEX_HOME\" ]; then echo 'no home' >&2; exit 3; fi\n" +
		"if [ -e \"$CODEX_HOME/config.toml\" ]; then echo 'home is not empty' >&2; exit 4; fi\n" +
		"echo '{\"models\":[{\"slug\":\"gpt-5.6-sol\"}]}'\n"
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("model = \"x\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	client, catalog, err := readCodexBundledCatalog(executable)
	if err != nil {
		t.Fatalf("readCodexBundledCatalog() error = %v", err)
	}
	if client.Version != "codex-cli 9.9.9" || client.SHA256 != hashText(script) {
		t.Fatalf("client identity = %+v", client)
	}
	if !strings.Contains(string(catalog), "gpt-5.6-sol") {
		t.Fatalf("catalog = %s", catalog)
	}
	if _, _, err := readCodexBundledCatalog("   "); err == nil {
		t.Fatal("readCodexBundledCatalog() accepted an empty executable")
	}
	if _, _, err := readCodexBundledCatalog(filepath.Join(dir, "absent")); err == nil {
		t.Fatal("readCodexBundledCatalog() accepted a missing executable")
	}
}

// TestReadCodexBundledCatalogReportsIdentityWhenTheDumpFails keeps the identity
// available on the failure path: deciding whether a previous copy may be reused
// requires knowing which build is installed now.
func TestReadCodexBundledCatalogReportsIdentityWhenTheDumpFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the probe script requires a POSIX shell")
	}
	dir := t.TempDir()
	executable := filepath.Join(dir, "codex")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then echo '1.2.3'; exit 0; fi\n" +
		"exit 7\n"
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	client, catalog, err := readCodexBundledCatalog(executable)
	if err == nil {
		t.Fatal("readCodexBundledCatalog() reported no error for a failing dump")
	}
	if catalog != nil {
		t.Fatalf("catalog = %s", catalog)
	}
	if !client.same(codexClient{Version: "1.2.3", SHA256: hashText(script)}) {
		t.Fatalf("client identity = %+v", client)
	}
}
