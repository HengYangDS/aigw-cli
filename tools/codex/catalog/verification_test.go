package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"aigw-cli/internal/codex"
)

const fakeBundledCatalog = `{"models":[{"slug":"gpt-5.6-sol","display_name":"Sol","priority":1},{"slug":"gpt-5.5","display_name":"Five","priority":2}]}`

const catalogFixturePath = "AIGW_TEST_CODEX_CATALOG"
const catalogFixtureLoad = "AIGW_TEST_CODEX_CATALOG_LOAD"

func TestMain(m *testing.M) {
	if path := os.Getenv(catalogFixturePath); path != "" {
		os.Exit(runCatalogFixture(path, os.Args[1:]))
	}
	os.Exit(m.Run())
}

func runCatalogFixture(path string, args []string) int {
	if len(args) == 1 && args[0] == "--version" {
		_, _ = os.Stdout.WriteString("codex-cli 0.0.0-fake\n")
		return 0
	}
	if len(args) < 2 || args[0] != "debug" || args[1] != "models" {
		return 64
	}
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		return 65
	}
	if _, err := os.Stat(filepath.Join(home, "config.toml")); !os.IsNotExist(err) {
		return 66
	}
	if trace := os.Getenv("AIGW_TEST_CODEX_PROBE_HOME"); trace != "" {
		if err := os.WriteFile(trace, []byte(home), 0o600); err != nil {
			return 70
		}
	}
	if os.Getenv("AIGW_TEST_CODEX_PROBE_FAILURE") == "true" {
		return 7
	}
	for _, arg := range args[2:] {
		value, configured := strings.CutPrefix(arg, "model_catalog_json=")
		if configured && os.Getenv(catalogFixtureLoad) == "true" {
			var err error
			path, err = strconv.Unquote(value)
			if err != nil {
				return 67
			}
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 68
	}
	if os.Getenv("AIGW_TEST_CODEX_PROJECTION_FAILURE") == "true" && path != os.Getenv(catalogFixturePath) {
		return 7
	}
	if _, err := os.Stdout.Write(data); err != nil {
		return 69
	}
	return 0
}

func fakeCodexClient(t *testing.T, useConfiguredCatalog bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bundled.json")
	if err := os.WriteFile(path, []byte(fakeBundledCatalog), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(catalogFixturePath, path)
	t.Setenv(catalogFixtureLoad, strconv.FormatBool(useConfiguredCatalog))
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return executable
}

func isolatedCatalogClient(t *testing.T) (executable, trace string) {
	t.Helper()
	executable = fakeCodexClient(t, true)
	ambient, scratch := t.TempDir(), t.TempDir()
	trace = filepath.Join(t.TempDir(), "probe-home")
	settings := filepath.Join(ambient, "config.toml")
	const original = "model = \"operator-model\"\n"
	if err := os.WriteFile(settings, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", ambient)
	t.Setenv("AIGW_TEST_CODEX_PROBE_HOME", trace)
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, scratch)
	}
	t.Cleanup(func() {
		directory, err := os.ReadFile(trace)
		if err != nil || string(directory) == ambient {
			t.Errorf("probe did not use an isolated directory: %s, %v", directory, err)
		}
		if content, err := os.ReadFile(settings); err != nil || string(content) != original {
			t.Errorf("probe changed operator settings: %q, %v", content, err)
		}
	})
	return executable, trace
}

func TestCatalogProbesOwnIsolationAndCleanup(t *testing.T) {
	for _, test := range []struct{ mode, failure string }{
		{"bundled", "none"},
		{"bundled", "command"},
		{"effective", "none"},
		{"effective", "command"},
	} {
		mode, failure := test.mode, test.failure
		t.Run(mode+"/"+failure, func(t *testing.T) {
			executable, trace := isolatedCatalogClient(t)
			t.Setenv("AIGW_TEST_CODEX_PROBE_FAILURE", strconv.FormatBool(failure != "none"))
			var data []byte
			var err error
			if mode == "bundled" {
				var identity codex.ExecutableIdentity
				identity, data, err = codex.ReadBundledCatalog(executable)
				program, readErr := os.ReadFile(executable)
				if readErr != nil || identity.Version != "codex-cli 0.0.0-fake" || identity.SHA256 != fmt.Sprintf("%x", sha256.Sum256(program)) {
					t.Fatalf("catalog probe lost executable identity: %+v, %v", identity, readErr)
				}
			} else {
				data, err = codex.ReadEffectiveCatalog(executable, "")
			}
			if failure == "none" {
				if err != nil || string(data) != fakeBundledCatalog {
					t.Fatalf("catalog output = %s, %v", data, err)
				}
			} else {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 7 || data != nil {
					t.Fatalf("catalog failure lost original exit status: %s, %v", data, err)
				}
			}
			home, readErr := os.ReadFile(trace)
			if readErr != nil {
				t.Fatalf("probe did not use an isolated home: %s, %v", home, readErr)
			}
			if _, statErr := os.Stat(string(home)); !os.IsNotExist(statErr) {
				t.Fatalf("probe home survived: %s, %v", home, statErr)
			}
		})
	}
}

func TestCatalogVerificationPreservesProbeAndProjectionCleanupFailures(t *testing.T) {
	executable, _ := isolatedCatalogClient(t)
	t.Setenv("AIGW_TEST_CODEX_PROJECTION_FAILURE", "true")
	remove := removeVerificationDirectory
	t.Cleanup(func() { removeVerificationDirectory = remove })
	var directory string
	removeVerificationDirectory = func(path string) error {
		directory = path
		return &os.PathError{Op: "remove", Path: path, Err: os.ErrPermission}
	}
	_, err := verifyCatalog(executable, "openai.gpt-5.6-sol")
	var exit *exec.ExitError
	if directory == "" || !errors.As(err, &exit) || exit.ExitCode() != 7 || !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), directory) {
		t.Fatalf("verification lost probe or cleanup cause: %v; directory %s", err, directory)
	}
	if _, err := os.Stat(filepath.Join(directory, "model-catalog.json")); err != nil {
		t.Fatalf("cleanup target did not own the projection: %v", err)
	}
}

func TestVerifyCatalogObservesTheClientsEffectiveCatalog(t *testing.T) {
	verification, err := verifyCatalog(fakeCodexClient(t, true), "openai.gpt-5.6-sol")
	if err != nil {
		t.Fatalf("verifyCatalog() error = %v", err)
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

func TestVerifyCatalogPreservesLargeModelMetadata(t *testing.T) {
	executable := fakeCodexClient(t, true)
	// A real client catalog contains model instructions, not just identifiers.
	// Its structured response is larger than a diagnostic-output budget.
	catalog := `{"models":[{"slug":"gpt-5.6-sol","instructions":"` + strings.Repeat("model metadata ", 40_000) + `"}]}`
	if err := os.WriteFile(os.Getenv(catalogFixturePath), []byte(catalog), 0o600); err != nil {
		t.Fatal(err)
	}
	verification, err := verifyCatalog(executable, "openai.gpt-5.6-sol")
	if err != nil {
		t.Fatal(err)
	}
	if err := verification.Check(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCatalogRejectsAClientThatIgnoresTheConfiguredCatalog(t *testing.T) {
	verification, err := verifyCatalog(fakeCodexClient(t, false), "openai.gpt-5.6-sol")
	if err != nil {
		t.Fatalf("verifyCatalog() error = %v", err)
	}
	err = verification.Check()
	if err == nil || !strings.Contains(err.Error(), "is absent from the effective catalog") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestCheckRejectsAliasMetadataDrift(t *testing.T) {
	reference := modelProbe{Model: "gpt-5.6-sol", Present: true, MetadataSHA256: "base"}
	verification := verificationResult{
		Model:     "openai.gpt-5.6-sol",
		BaseSlug:  "gpt-5.6-sol",
		Reference: reference,
		Unadapted: modelProbe{Model: "openai.gpt-5.6-sol"},
		Adapted:   modelProbe{Model: "openai.gpt-5.6-sol", Present: true, MetadataSHA256: "different"},
		Unknown:   modelProbe{Model: unknownProbeModel},
	}
	err := verification.Check()
	if err == nil || !strings.Contains(err.Error(), "metadata digest") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestCheckRejectsAnAliasAlreadyPresentInTheBundledCatalog(t *testing.T) {
	reference := modelProbe{Model: "gpt-5.6-sol", Present: true, MetadataSHA256: "base"}
	verification := verificationResult{
		Model:     "openai.gpt-5.6-sol",
		BaseSlug:  "gpt-5.6-sol",
		Reference: reference,
		Unadapted: modelProbe{Model: "openai.gpt-5.6-sol", Present: true, MetadataSHA256: "base"},
		Adapted:   modelProbe{Model: "openai.gpt-5.6-sol", Present: true, MetadataSHA256: "base"},
		Unknown:   modelProbe{Model: unknownProbeModel},
	}
	err := verification.Check()
	if err == nil || !strings.Contains(err.Error(), "already exists in the bundled catalog") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestCheckRejectsAnUnknownModelInTheEffectiveCatalog(t *testing.T) {
	reference := modelProbe{Model: "gpt-5.6-sol", Present: true, MetadataSHA256: "base"}
	verification := verificationResult{
		Model:     "openai.gpt-5.6-sol",
		BaseSlug:  "gpt-5.6-sol",
		Reference: reference,
		Unadapted: modelProbe{Model: "openai.gpt-5.6-sol"},
		Adapted:   modelProbe{Model: "openai.gpt-5.6-sol", Present: true, MetadataSHA256: "base"},
		Unknown:   modelProbe{Model: unknownProbeModel, Present: true, MetadataSHA256: "unknown"},
	}
	err := verification.Check()
	if err == nil || !strings.Contains(err.Error(), "unexpectedly exists in the effective catalog") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestCheckRejectsAMissingBundledBase(t *testing.T) {
	verification := verificationResult{
		Model:     "openai.gpt-5.6-sol",
		BaseSlug:  "gpt-5.6-sol",
		Reference: modelProbe{Model: "gpt-5.6-sol"},
	}
	err := verification.Check()
	if err == nil || !strings.Contains(err.Error(), "absent from the bundled catalog") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestVerifyCatalogReportsWhatItCannotVerify(t *testing.T) {
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
			_, err := verifyCatalog(testCase.executable, testCase.model)
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("verifyCatalog() error = %v, want %q", err, testCase.wantError)
			}
		})
	}
}

func TestProbeCodexCatalogRejectsMalformedOutput(t *testing.T) {
	executable := fakeCodexClient(t, true)
	if err := os.WriteFile(os.Getenv(catalogFixturePath), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := probeCodexCatalog(executable, ""); err == nil || !strings.Contains(err.Error(), "read Codex effective model catalog") {
		t.Fatalf("probeCodexCatalog() error = %v", err)
	}
}

func TestProbeCodexCatalogCanReadTheClientsDefaultCatalog(t *testing.T) {
	document, err := probeCodexCatalog(fakeCodexClient(t, true), "")
	if err != nil {
		t.Fatal(err)
	}
	probe := catalogProbe(document, "gpt-5.6-sol")
	if !probe.Present {
		t.Fatalf("catalogProbe() = %+v", probe)
	}
}
