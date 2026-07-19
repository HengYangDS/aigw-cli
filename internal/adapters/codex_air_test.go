package adapters

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func prepareAirHostMirrorFixture(t *testing.T) (string, string, []byte) {
	t.Helper()
	root := t.TempDir()
	standalone := filepath.Join(root, "standalone", "config.toml")
	air := filepath.Join(root, "air", "config.toml")
	for _, path := range []string{standalone, air} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(standalone, []byte("model_provider = \"native\"\nmodel = \"native-model\"\nstandalone_only = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SyncCodexConfig(standalone, atomicTestRuntime()); err != nil {
		t.Fatal(err)
	}
	projection, err := os.ReadFile(standalone)
	if err != nil {
		t.Fatal(err)
	}
	airProjection := append([]byte(nil), projection...)
	airProjection = append(airProjection, []byte("\n[jetbrains]\nhost_only = true\n")...)
	if err := os.WriteFile(air, airProjection, 0o600); err != nil {
		t.Fatal(err)
	}
	return air, standalone, airProjection
}

func TestInspectAirCodexConfigAcceptsExactStandaloneHostMirror(t *testing.T) {
	air, standalone, _ := prepareAirHostMirrorFixture(t)

	inspection, err := InspectAirCodexConfig(air, standalone)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != "external-host-mirror" || inspection.AIGWManaged || inspection.SidecarPresent {
		t.Fatalf("inspection = %#v", inspection)
	}
}

func TestInspectAirCodexConfigIgnoresUnrelatedHostTableAssignments(t *testing.T) {
	air, standalone, body := prepareAirHostMirrorFixture(t)
	withHostAssignments := string(body) + "\n[jetbrains.profile]\nmodel_provider = \"host-provider\"\nmodel = \"host-model\"\n"
	if err := os.WriteFile(air, []byte(withHostAssignments), 0o600); err != nil {
		t.Fatal(err)
	}

	inspection, err := InspectAirCodexConfig(air, standalone)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != "external-host-mirror" || inspection.AIGWManaged {
		t.Fatalf("inspection = %#v", inspection)
	}
}

func TestInspectAirCodexConfigRequiresRecognizedStandaloneSidecar(t *testing.T) {
	air, standalone, _ := prepareAirHostMirrorFixture(t)
	if err := os.Remove(codexStatePath(standalone)); err != nil {
		t.Fatal(err)
	}

	inspection, err := InspectAirCodexConfig(air, standalone)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != "orphaned-aigw-marker" || inspection.AIGWManaged {
		t.Fatalf("inspection = %#v", inspection)
	}
}

func TestInspectAirCodexConfigRejectsSemanticMirrorDifferences(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(string) string
		changed string
	}{
		{
			name: "model",
			mutate: func(text string) string {
				return strings.Replace(text, `model = "gpt-5.6-terra" # managed by AIGW`, `model = "gpt-5.6-sol" # managed by AIGW`, 1)
			},
			changed: "gpt-5.6-sol",
		},
		{
			name: "endpoint",
			mutate: func(text string) string {
				return strings.Replace(text, atomicTestRuntime().Endpoint, "https://different.test/v1", 1)
			},
			changed: "different.test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			air, standalone, body := prepareAirHostMirrorFixture(t)
			mutated := tt.mutate(string(body))
			if !strings.Contains(mutated, tt.changed) {
				t.Fatalf("fixture mutation did not apply: %s", mutated)
			}
			if err := os.WriteFile(air, []byte(mutated), 0o600); err != nil {
				t.Fatal(err)
			}

			inspection, err := InspectAirCodexConfig(air, standalone)
			if err != nil {
				t.Fatal(err)
			}
			if inspection.State != "orphaned-aigw-marker" || inspection.AIGWManaged {
				t.Fatalf("inspection = %#v", inspection)
			}
		})
	}
}

func TestInspectAirCodexConfigRejectsPartialOrForeignResidue(t *testing.T) {
	air, standalone, _ := prepareAirHostMirrorFixture(t)
	partial := "model_provider = \"aigw\" # managed by AIGW\n" +
		"# >>> AIGW managed provider >>>\n" +
		"[model_providers.aigw]\n" +
		"base_url = \"https://partial.test/v1\"\n"
	if err := os.WriteFile(air, []byte(partial), 0o600); err != nil {
		t.Fatal(err)
	}

	inspection, err := InspectAirCodexConfig(air, standalone)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != "partial-or-foreign-residue" || inspection.AIGWManaged {
		t.Fatalf("inspection = %#v", inspection)
	}
}

func TestInspectAirCodexConfigNormalizesOnlyLineEndings(t *testing.T) {
	air, standalone, body := prepareAirHostMirrorFixture(t)
	crlf := strings.ReplaceAll(string(body), "\n", "\r\n")
	if err := os.WriteFile(air, []byte(crlf), 0o600); err != nil {
		t.Fatal(err)
	}

	inspection, err := InspectAirCodexConfig(air, standalone)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != "external-host-mirror" || inspection.AIGWManaged {
		t.Fatalf("inspection = %#v", inspection)
	}

	padded := strings.Replace(crlf, `model_provider = "aigw" # managed by AIGW`, `model_provider = "aigw"  # managed by AIGW`, 1)
	if err := os.WriteFile(air, []byte(padded), 0o600); err != nil {
		t.Fatal(err)
	}
	inspection, err = InspectAirCodexConfig(air, standalone)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != "orphaned-aigw-marker" {
		t.Fatalf("padded inspection = %#v", inspection)
	}

	markerPadded := strings.Replace(crlf, `model_provider = "aigw" # managed by AIGW`, `model_provider = "aigw" #   managed by AIGW`, 1)
	if err := os.WriteFile(air, []byte(markerPadded), 0o600); err != nil {
		t.Fatal(err)
	}
	inspection, err = InspectAirCodexConfig(air, standalone)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != "orphaned-aigw-marker" {
		t.Fatalf("comment-padded inspection = %#v", inspection)
	}
}

func TestInspectAirCodexConfigDoesNotOverrideAirSidecarOwnership(t *testing.T) {
	air, standalone, _ := prepareAirHostMirrorFixture(t)
	if err := os.WriteFile(air, []byte("model_provider = \"jetbrains\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SyncCodexConfig(air, atomicTestRuntime()); err != nil {
		t.Fatal(err)
	}

	inspection, err := InspectAirCodexConfig(air, standalone)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != "aigw-managed" || !inspection.AIGWManaged || !inspection.SidecarPresent {
		t.Fatalf("inspection = %#v", inspection)
	}
}

func TestInspectAirCodexConfigRejectsDriftedStandaloneReference(t *testing.T) {
	air, standalone, _ := prepareAirHostMirrorFixture(t)
	standaloneBody, err := os.ReadFile(standalone)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(standaloneBody), atomicTestRuntime().Endpoint, "https://drifted.test/v1", 1)
	if err := os.WriteFile(standalone, []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(air, []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}

	inspection, err := InspectAirCodexConfig(air, standalone)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != "orphaned-aigw-marker" || inspection.AIGWManaged {
		t.Fatalf("inspection = %#v", inspection)
	}
}

func TestPlanAirOrphanRemovalReturnsExactReadOnlySnapshots(t *testing.T) {
	air, standalone, body := prepareAirHostMirrorFixture(t)
	orphan := strings.Replace(string(body), atomicTestRuntime().Endpoint, "https://orphan.test/v1", 1) +
		"\n[jetbrains.profile]\nmodel_provider = \"host-provider\"\nmodel = \"host-model\"\n"
	if err := os.WriteFile(air, []byte(orphan), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(air, 0o640); err != nil {
		t.Fatal(err)
	}

	plan, err := PlanAirOrphanRemoval(air, standalone)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Preimage.Exists || !bytes.Equal(plan.Preimage.Data, []byte(orphan)) || plan.Preimage.Mode != 0o640 {
		t.Fatalf("preimage = %#v", plan.Preimage)
	}
	if !plan.Cleaned.Exists || plan.Cleaned.Mode != plan.Preimage.Mode || plan.ProjectionFingerprintSHA256 == "" {
		t.Fatalf("plan = %#v", plan)
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"orphan.test", "gpt-5.6-terra", "model_provider", plan.ProjectionFingerprintSHA256} {
		if bytes.Contains(encoded, []byte(forbidden)) {
			t.Fatalf("serialized removal plan leaked %q: %s", forbidden, encoded)
		}
	}
	cleaned := string(plan.Cleaned.Data)
	for _, forbidden := range []string{"managed by AIGW", "AIGW managed provider", "[model_providers.aigw]", "orphan.test", "gpt-5.6-terra"} {
		if strings.Contains(cleaned, forbidden) {
			t.Fatalf("cleaned snapshot retained %q:\n%s", forbidden, cleaned)
		}
	}
	for _, preserved := range []string{"standalone_only = true", "[jetbrains]", "host_only = true", "model_provider = \"host-provider\"", "model = \"host-model\""} {
		if !strings.Contains(cleaned, preserved) {
			t.Fatalf("cleaned snapshot lost %q:\n%s", preserved, cleaned)
		}
	}
	after, err := os.ReadFile(air)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, []byte(orphan)) {
		t.Fatal("orphan removal plan mutated Air")
	}
}

func TestPlanAirOrphanRemovalRejectsNonOrphanStates(t *testing.T) {
	t.Run("host mirror", func(t *testing.T) {
		air, standalone, _ := prepareAirHostMirrorFixture(t)
		if _, err := PlanAirOrphanRemoval(air, standalone); err == nil {
			t.Fatal("host mirror unexpectedly admitted for orphan removal")
		}
	})
	t.Run("partial residue", func(t *testing.T) {
		air, standalone, _ := prepareAirHostMirrorFixture(t)
		if err := os.WriteFile(air, []byte("# >>> AIGW managed provider >>>\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := PlanAirOrphanRemoval(air, standalone); err == nil {
			t.Fatal("partial residue unexpectedly admitted for orphan removal")
		}
	})
	t.Run("sidecar", func(t *testing.T) {
		air, standalone, _ := prepareAirHostMirrorFixture(t)
		if err := os.WriteFile(air, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := SyncCodexConfig(air, atomicTestRuntime()); err != nil {
			t.Fatal(err)
		}
		if _, err := PlanAirOrphanRemoval(air, standalone); err == nil {
			t.Fatal("sidecar-backed projection unexpectedly admitted for orphan removal")
		}
	})
}
