package codex

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeCodexClient writes a client that answers the read-only questions the
// verification asks. It resolves a model the same way the real client does — from
// its own table, or from a projected catalog when one is named — and then both
// reports shorter instructions with fewer input items, which are the differences
// the verification measures.
func fakeCodexClient(t *testing.T, resolution string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake client is a POSIX shell script")
	}
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then echo 'codex-cli 0.0.0-fake'; exit 0; fi
if [ "$1" = "debug" ] && [ "$2" = "models" ]; then
  echo '{"models":[{"slug":"gpt-5.6-sol","display_name":"Sol"},{"slug":"gpt-5.5","display_name":"Five"}]}'
  exit 0
fi
model=""
catalog=""
export_dir=""
while [ $# -gt 0 ]; do
  case "$1" in
    model=*) model=${1#model=} ;;
    model_catalog_json=*) catalog=${1#model_catalog_json=} ;;
    debug.config_lockfile.export_dir=*) export_dir=${1#debug.config_lockfile.export_dir=} ;;
  esac
  shift
done
[ -n "$CODEX_HOME" ] || exit 65
resolved=no
case "$model" in gpt-5.6-sol|gpt-5.5) resolved=yes ;; esac
RESOLUTION
[ -n "$export_dir" ] || exit 64
if [ "$resolved" = yes ]; then
  body='the model'"'"'s own instructions, which are longer'
  items='[{"a":1},{"b":2},{"c":"multi_agent"},{"d":"multi_agent"},{"e":5}]'
else
  body='placeholder'
  items='[{"a":1},{"b":2},{"c":3}]'
fi
printf 'version = 1\nmodel = "%s"\ninstructions = """\n%s\n"""\n' "$model" "$body" > "$export_dir/session.config.lock.toml"
echo "$items"
`
	script = strings.Replace(script, "RESOLUTION", resolution, 1)
	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return executable
}

// resolvesThroughCatalog is how a correct client behaves: an id it does not know
// is resolved only when the named catalog actually declares it.
const resolvesThroughCatalog = `if [ "$resolved" = no ] && [ -n "$catalog" ] && grep -q "\"$model\"" "$catalog"; then resolved=yes; fi`

func TestVerifyModelCatalogMeasuresTheClientsOwnResolution(t *testing.T) {
	executable := fakeCodexClient(t, resolvesThroughCatalog)
	verification, err := VerifyModelCatalog(executable, "openai.gpt-5.6-sol")
	if err != nil {
		t.Fatalf("VerifyModelCatalog() error = %v", err)
	}
	if err := verification.Check(); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if verification.BaseSlug != "gpt-5.6-sol" {
		t.Fatalf("base slug = %q", verification.BaseSlug)
	}
	if verification.ClientVersion != "codex-cli 0.0.0-fake" || verification.ClientSHA256 == "" {
		t.Fatalf("client identity = %q / %q", verification.ClientVersion, verification.ClientSHA256)
	}
	// The prefixed id must reach the base slug's own resolution, and the
	// unadapted and unknown selections must both stay on the client's fallback.
	if !verification.Adapted.same(verification.Reference) {
		t.Fatalf("adapted = %s, reference = %s", verification.Adapted, verification.Reference)
	}
	if verification.Unadapted.same(verification.Reference) {
		t.Fatalf("unadapted = %s, which does not differ from the reference", verification.Unadapted)
	}
	if !verification.Unknown.same(verification.Unadapted) {
		t.Fatalf("unknown = %s, fallback = %s", verification.Unknown, verification.Unadapted)
	}
	if verification.Reference.Items != 5 || verification.Reference.MultiAgent != 2 || verification.Unadapted.Items != 3 || verification.Unadapted.MultiAgent != 0 {
		t.Fatalf("probes did not separate the states: %+v", verification)
	}
	if verification.Reference.Instructions <= verification.Unadapted.Instructions {
		t.Fatalf("instruction lengths did not separate the states: %+v", verification)
	}
}

// TestVerifyModelCatalogRejectsAClientThatAnswersForAnythingIsTheWholePoint:
// a catalog that made unknown models resolve would have silenced the client's
// warning rather than fixed the prefixed id, and the check must say so.
func TestVerifyModelCatalogRejectsASilencedWarning(t *testing.T) {
	permissive := `if [ -n "$catalog" ]; then resolved=yes; fi`
	verification, err := VerifyModelCatalog(fakeCodexClient(t, permissive), "openai.gpt-5.6-sol")
	if err != nil {
		t.Fatalf("VerifyModelCatalog() error = %v", err)
	}
	err = verification.Check()
	if err == nil || !strings.Contains(err.Error(), "instead of the client's fallback") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestVerifyModelCatalogRejectsACatalogThatChangesNothing(t *testing.T) {
	ignored := ``
	verification, err := VerifyModelCatalog(fakeCodexClient(t, ignored), "openai.gpt-5.6-sol")
	if err != nil {
		t.Fatalf("VerifyModelCatalog() error = %v", err)
	}
	err = verification.Check()
	if err == nil || !strings.Contains(err.Error(), "through the generated catalog, but") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestCheckRejectsAMeasurementThatCannotSeparateTheStates(t *testing.T) {
	resolved := ModelCatalogProbe{Model: "gpt-5.6-sol", Instructions: 100, Items: 5, MultiAgent: 2}
	verification := ModelCatalogVerification{
		Model:     "openai.gpt-5.6-sol",
		BaseSlug:  "gpt-5.6-sol",
		Reference: resolved,
		Unadapted: resolved,
		Adapted:   resolved,
		Unknown:   resolved,
	}
	err := verification.Check()
	if err == nil || !strings.Contains(err.Error(), "already resolves like") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestVerifyModelCatalogReportsWhatItCannotVerify(t *testing.T) {
	executable := fakeCodexClient(t, resolvesThroughCatalog)
	for _, testCase := range []struct {
		name       string
		executable string
		model      string
		wantError  string
	}{
		{"client is absent", filepath.Join(t.TempDir(), "absent"), "openai.gpt-5.6-sol", "read Codex executable"},
		{"model has no unique base", executable, "openai.not-a-model", "no unique Codex model matches"},
		{"model needs no catalog", executable, "gpt-5.6-sol", "no unique Codex model matches"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := VerifyModelCatalog(testCase.executable, testCase.model)
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("VerifyModelCatalog() error = %v, want %q", err, testCase.wantError)
			}
		})
	}
}

// TestProbeCodexModelRequiresAnExportedConfiguration keeps the measurement
// honest: an instruction length of zero would compare equal across states and
// report a pass, so a client that exports nothing is an error instead.
func TestProbeCodexModelRequiresAnExportedConfiguration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake client is a POSIX shell script")
	}
	dir := t.TempDir()
	executable := filepath.Join(dir, "codex")
	silent := "#!/bin/sh\necho '[]'\n"
	if err := os.WriteFile(executable, []byte(silent), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := probeCodexModel(executable, "gpt-5.6-sol", ""); err == nil || !strings.Contains(err.Error(), "exported no configuration") {
		t.Fatalf("probeCodexModel() error = %v", err)
	}

	empty := filepath.Join(dir, "codex-no-instructions")
	script := "#!/bin/sh\nfor a in \"$@\"; do case \"$a\" in debug.config_lockfile.export_dir=*) d=${a#debug.config_lockfile.export_dir=} ;; esac; done\nprintf 'version = 1\\n' > \"$d/s.config.lock.toml\"\necho '[]'\n"
	if err := os.WriteFile(empty, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := probeCodexModel(empty, "gpt-5.6-sol", ""); err == nil || !strings.Contains(err.Error(), "declares no instructions") {
		t.Fatalf("probeCodexModel() error = %v", err)
	}
}

func TestResolvedInstructionsAcceptsEitherTOMLDelimiter(t *testing.T) {
	for _, body := range []string{
		"version = 1\ninstructions = \"\"\"\nabcd\n\"\"\"\n",
		"version = 1\ninstructions = '''\nabcd\n'''\n",
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "s.config.lock.toml"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		length, err := resolvedInstructionLength(dir)
		if err != nil {
			t.Fatal(err)
		}
		if length != len("\nabcd\n") {
			t.Fatalf("instruction length = %d for %q", length, body)
		}
	}
}
