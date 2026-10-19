package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestObserveSelectionReportsOnlyAModelOnlyChange(t *testing.T) {
	for _, test := range []struct {
		name, from, to string
		model          string
	}{
		{name: "unchanged"},
		{name: "comment removed", from: `model = "gpt-test" # managed by AIGW`, to: `model = "gpt-test"`},
		{name: "model", from: `model = "gpt-test" # managed by AIGW`, to: `model = "gpt-other"`, model: "gpt-other"},
		{name: "native rewrite", from: `model_provider = "aigw" # managed by AIGW` + "\n" + `model = "gpt-test" # managed by AIGW`, to: `model_provider = "aigw"` + "\n" + `model = "gpt-other"`, model: "gpt-other"},
		{name: "provider", from: `model_provider = "aigw" # managed by AIGW` + "\n" + `model = "gpt-test" # managed by AIGW`, to: `model_provider = "openai"` + "\n" + `model = "gpt-other"`},
		{name: "provider table", from: `base_url = "https://example.test/v1"`, to: `base_url = "https://other.test/v1"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := SyncConfig(path, ownershipTestRuntime()); err != nil {
				t.Fatal(err)
			}
			projected, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			current := string(projected)
			if test.from != "" {
				if !strings.Contains(current, test.from) {
					t.Fatalf("projection lacks %q:\n%s", test.from, current)
				}
				current = strings.Replace(current, test.from, test.to, 1)
				if test.name == "provider table" {
					current = strings.Replace(current, `model = "gpt-test" # managed by AIGW`, `model = "gpt-other"`, 1)
				}
				if err := os.WriteFile(path, []byte(current), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			model, changed, err := ObserveSelection(path)
			if err != nil || model != test.model || changed != (test.model != "") {
				t.Fatalf("ObserveSelection() = %q, %t, %v; want %q", model, changed, err, test.model)
			}
			if after, err := os.ReadFile(path); err != nil || string(after) != current {
				t.Fatalf("observation changed the Codex configuration: %v", err)
			}
		})
	}
}

func TestObserveSelectionNeedsTheRecordedProjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model = \"gpt-native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if model, changed, err := ObserveSelection(path); err != nil || changed || model != "" {
		t.Fatalf("unprojected target = %q, %t, %v", model, changed, err)
	}
	legacy := filepath.Join(t.TempDir(), "config.toml")
	writeLegacyCodexProjection(t, legacy, atomicTestRuntime().Model)
	projected, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(projected), `model = "gpt-5.6-terra" # managed by AIGW`, `model = "gpt-other"`, 1)
	if err := os.WriteFile(legacy, []byte(drifted), 0o600); err != nil {
		t.Fatal(err)
	}
	if model, changed, err := ObserveSelection(legacy); err != nil || changed || model != "" {
		t.Fatalf("a sidecar without a projected model was attributed: %q, %t, %v", model, changed, err)
	}
}

func TestReconcileConfigsTransitionAcceptsTheModelBeingProjected(t *testing.T) {
	for _, test := range []struct {
		name, native string
		accepted     bool
	}{
		{name: "chosen", native: "gpt-next", accepted: true},
		{name: "foreign", native: "gpt-foreign"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			previous := ownershipTestRuntime()
			if err := os.WriteFile(path, []byte("model = \"gpt-original\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := SyncConfig(path, previous); err != nil {
				t.Fatal(err)
			}
			projected, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			drifted := strings.Replace(string(projected), `model = "gpt-test" # managed by AIGW`, `model = "`+test.native+`"`, 1)
			if err := os.WriteFile(path, []byte(drifted), 0o600); err != nil {
				t.Fatal(err)
			}
			next := previous
			next.RouteID, next.RouteLabel, next.Model = "next", "Next", "gpt-next"
			target := codexHomeTarget(path)
			_, err = ReconcileConfigsTransition([]TargetRef{target}, []TargetRef{target}, previous, next)
			after, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !test.accepted {
				if err == nil || !strings.Contains(err.Error(), "model selection changed") || string(after) != drifted {
					t.Fatalf("a foreign model was overwritten: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("the model being projected was refused: %v", err)
			}
			if !strings.Contains(string(after), `model = "gpt-next" # managed by AIGW`) || !strings.Contains(string(after), `name = "AIGW: Next"`) {
				t.Fatalf("transition did not project the chosen Route:\n%s", after)
			}
		})
	}
}
