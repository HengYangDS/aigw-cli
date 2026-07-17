package adapters

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectCodexConfigReportsFallbackOwnershipWithoutLeakingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := []byte("model_provider = \"jetbrains\"\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileCodexConfigs(nil, []CodexTargetRef{airFallbackCodexTarget(path)}, atomicTestRuntime()); err != nil {
		t.Fatal(err)
	}
	inspection, err := InspectCodexConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != "fallback-staged" || inspection.ProjectionMode != CodexProjectionNamespacedFallback || inspection.AttributionState != "recognized" || !inspection.SidecarHashMatches || !inspection.AIGWManaged {
		t.Fatalf("inspection = %#v", inspection)
	}
	encoded, err := json.Marshal(inspection)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{
		[]byte(atomicTestRuntime().Endpoint),
		original,
		[]byte("AIGW fallback:"),
	} {
		if bytes.Contains(encoded, forbidden) {
			t.Fatalf("inspection leaked %q: %s", forbidden, encoded)
		}
	}
}
