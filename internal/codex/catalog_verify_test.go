package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const fakeBundledCatalog = `{"models":[{"slug":"gpt-5.6-sol","display_name":"Sol","priority":1},{"slug":"gpt-5.5","display_name":"Five","priority":2}]}`

// fakeCodexClient writes the smallest executable that implements the public
// Codex surfaces the verifier consumes. Production code is portable; this
// shell fixture is skipped on Windows, whose release lane uses a real client.
func fakeCodexClient(t *testing.T, useConfiguredCatalog bool) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake client fixture is a POSIX shell script")
	}
	configured := `cat "$catalog"`
	if !useConfiguredCatalog {
		configured = `printf '%s\n' "$bundled"`
	}
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then echo 'codex-cli 0.0.0-fake'; exit 0; fi
if [ "$1" != "debug" ] || [ "$2" != "models" ]; then exit 64; fi
bundled='BUNDLED'
catalog=''
for arg in "$@"; do
  case "$arg" in
    model_catalog_json=*) catalog=${arg#model_catalog_json=} ;;
  esac
done
catalog=${catalog#\"}
catalog=${catalog%\"}
if [ -n "$catalog" ]; then
  CONFIGURED
else
  printf '%s\n' "$bundled"
fi
`
	script = strings.Replace(script, "BUNDLED", fakeBundledCatalog, 1)
	script = strings.Replace(script, "CONFIGURED", configured, 1)
	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return executable
}

func TestVerifyModelCatalogObservesTheClientsEffectiveCatalog(t *testing.T) {
	verification, err := VerifyModelCatalog(fakeCodexClient(t, true), "openai.gpt-5.6-sol")
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
	if !verification.Reference.Present || verification.Unadapted.Present || !verification.Adapted.Present || verification.Unknown.Present {
		t.Fatalf("unexpected catalog membership: %+v", verification)
	}
	if verification.Reference.MetadataSHA256 == "" || verification.Adapted.MetadataSHA256 != verification.Reference.MetadataSHA256 {
		t.Fatalf("alias metadata differs from base metadata: %+v", verification)
	}
}

func TestVerifyModelCatalogRejectsAClientThatIgnoresTheConfiguredCatalog(t *testing.T) {
	verification, err := VerifyModelCatalog(fakeCodexClient(t, false), "openai.gpt-5.6-sol")
	if err != nil {
		t.Fatalf("VerifyModelCatalog() error = %v", err)
	}
	err = verification.Check()
	if err == nil || !strings.Contains(err.Error(), "is absent from the effective catalog") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestCheckRejectsAliasMetadataDrift(t *testing.T) {
	reference := ModelCatalogProbe{Model: "gpt-5.6-sol", Present: true, MetadataSHA256: "base"}
	verification := ModelCatalogVerification{
		Model:     "openai.gpt-5.6-sol",
		BaseSlug:  "gpt-5.6-sol",
		Reference: reference,
		Unadapted: ModelCatalogProbe{Model: "openai.gpt-5.6-sol"},
		Adapted:   ModelCatalogProbe{Model: "openai.gpt-5.6-sol", Present: true, MetadataSHA256: "different"},
		Unknown:   ModelCatalogProbe{Model: unknownProbeModel},
	}
	err := verification.Check()
	if err == nil || !strings.Contains(err.Error(), "metadata digest") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestCheckRejectsAnAliasAlreadyPresentInTheBundledCatalog(t *testing.T) {
	reference := ModelCatalogProbe{Model: "gpt-5.6-sol", Present: true, MetadataSHA256: "base"}
	verification := ModelCatalogVerification{
		Model:     "openai.gpt-5.6-sol",
		BaseSlug:  "gpt-5.6-sol",
		Reference: reference,
		Unadapted: ModelCatalogProbe{Model: "openai.gpt-5.6-sol", Present: true, MetadataSHA256: "base"},
		Adapted:   ModelCatalogProbe{Model: "openai.gpt-5.6-sol", Present: true, MetadataSHA256: "base"},
		Unknown:   ModelCatalogProbe{Model: unknownProbeModel},
	}
	err := verification.Check()
	if err == nil || !strings.Contains(err.Error(), "already exists in the bundled catalog") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestCheckRejectsAnUnknownModelInTheEffectiveCatalog(t *testing.T) {
	reference := ModelCatalogProbe{Model: "gpt-5.6-sol", Present: true, MetadataSHA256: "base"}
	verification := ModelCatalogVerification{
		Model:     "openai.gpt-5.6-sol",
		BaseSlug:  "gpt-5.6-sol",
		Reference: reference,
		Unadapted: ModelCatalogProbe{Model: "openai.gpt-5.6-sol"},
		Adapted:   ModelCatalogProbe{Model: "openai.gpt-5.6-sol", Present: true, MetadataSHA256: "base"},
		Unknown:   ModelCatalogProbe{Model: unknownProbeModel, Present: true, MetadataSHA256: "unknown"},
	}
	err := verification.Check()
	if err == nil || !strings.Contains(err.Error(), "unexpectedly exists in the effective catalog") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestModelCatalogProbeStringReportsPresence(t *testing.T) {
	if got := (ModelCatalogProbe{Model: "missing"}).String(); got != "absent" {
		t.Fatalf("absent probe = %q", got)
	}
	if got := (ModelCatalogProbe{Model: "present", Present: true, MetadataSHA256: "abc"}).String(); got != "present, metadata sha256 abc" {
		t.Fatalf("present probe = %q", got)
	}
}

func TestCheckRejectsAMissingBundledBase(t *testing.T) {
	verification := ModelCatalogVerification{
		Model:     "openai.gpt-5.6-sol",
		BaseSlug:  "gpt-5.6-sol",
		Reference: ModelCatalogProbe{Model: "gpt-5.6-sol"},
	}
	err := verification.Check()
	if err == nil || !strings.Contains(err.Error(), "absent from the bundled catalog") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestVerifyModelCatalogReportsWhatItCannotVerify(t *testing.T) {
	executable := fakeCodexClient(t, true)
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

func TestProbeCodexCatalogRejectsMalformedOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake client fixture is a POSIX shell script")
	}
	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\necho 'not json'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := probeCodexCatalog(executable, ""); err == nil || !strings.Contains(err.Error(), "parse Codex effective model catalog") {
		t.Fatalf("probeCodexCatalog() error = %v", err)
	}
}

func TestProbeCodexCatalogCanReadTheClientsDefaultCatalog(t *testing.T) {
	document, err := probeCodexCatalog(fakeCodexClient(t, true), "")
	if err != nil {
		t.Fatal(err)
	}
	probe, err := catalogProbe(document, "gpt-5.6-sol")
	if err != nil || !probe.Present {
		t.Fatalf("catalogProbe() = %+v, %v", probe, err)
	}
}

func TestDecodeCodexCatalogRejectsInvalidEntries(t *testing.T) {
	for _, data := range []string{
		`{"models":[{"display_name":"missing slug"}]}`,
		`{"models":[{"slug":"same"},{"slug":"same"}]}`,
	} {
		if _, err := decodeCodexCatalog([]byte(data), "effective"); err == nil || !strings.Contains(err.Error(), "validate Codex effective model catalog") {
			t.Fatalf("decodeCodexCatalog(%s) error = %v", data, err)
		}
	}
}

func TestCatalogProbeRejectsMalformedEntries(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		entry map[string]json.RawMessage
		want  string
	}{
		{"missing slug", map[string]json.RawMessage{"name": json.RawMessage(`"x"`)}, "has no slug"},
		{"invalid slug", map[string]json.RawMessage{"slug": json.RawMessage(`{`)}, "parse Codex model catalog slug"},
		{"invalid metadata", map[string]json.RawMessage{"slug": json.RawMessage(`"target"`), "metadata": json.RawMessage(`{`)}, "encode Codex metadata"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := catalogProbe(codexCatalogDocument{Models: []map[string]json.RawMessage{testCase.entry}}, "target")
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("catalogProbe() error = %v", err)
			}
		})
	}
}
