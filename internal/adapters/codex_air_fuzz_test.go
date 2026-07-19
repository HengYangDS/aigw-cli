package adapters

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func airProjectionFuzzFixture(providerFirst bool, newline string) string {
	provider := `model_provider = "aigw" # managed by AIGW`
	model := `model = "gpt-5.6-terra" # managed by AIGW`
	selection := provider + newline + model + newline
	if !providerFirst {
		selection = model + newline + provider + newline
	}
	return selection +
		codexBegin + newline +
		`[model_providers.aigw]` + newline +
		`name = "AIGW: fuzz"` + newline +
		`base_url = "https://gateway.test/v1"` + newline +
		`wire_api = "responses"` + newline +
		`requires_openai_auth = true` + newline +
		codexEnd + newline
}

func TestAirProjectionFingerprintPreservesSelectionOrder(t *testing.T) {
	providerFirst, ok := exactAirManagedProjection(airProjectionFuzzFixture(true, "\n"))
	if !ok {
		t.Fatal("provider-first fixture is not an exact managed projection")
	}
	modelFirst, ok := exactAirManagedProjection(airProjectionFuzzFixture(false, "\n"))
	if !ok {
		t.Fatal("model-first fixture is not an exact managed projection")
	}
	if providerFirst.fingerprint == modelFirst.fingerprint {
		t.Fatal("fingerprint reordered top-level selection fields")
	}
}

func FuzzExactAirManagedProjection(f *testing.F) {
	for _, seed := range []string{
		airProjectionFuzzFixture(true, "\n"),
		airProjectionFuzzFixture(true, "\r\n"),
		airProjectionFuzzFixture(false, "\n"),
		codexBegin + "\n[model_providers.aigw]\n",
		"model_provider = \"aigw\" # managed by AIGW\n" + codexBegin + "\n" + codexBegin,
		"[model_providers.aigw_fallback]\nmanaged by AIGW\n",
		"Bearer secret-token https://secret.example/private/session-id",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		text := string(raw)
		projection, exact := exactAirManagedProjection(text)
		if exact {
			if projection == nil || len(projection.fingerprint) != 64 {
				t.Fatalf("exact projection returned an invalid fingerprint shape: %#v", projection)
			}
			if strings.Count(text, codexBegin) != 1 || strings.Count(text, codexEnd) != 1 ||
				strings.Count(text, "[model_providers.aigw") != 1 ||
				strings.Contains(text, codexFallbackBegin) || strings.Contains(text, codexFallbackEnd) ||
				strings.Contains(text, "[model_providers.aigw_fallback]") {
				t.Fatal("malformed or fallback projection was accepted as exact")
			}
		}

		root := t.TempDir()
		airPath := filepath.Join(root, "air", "config.toml")
		if err := os.MkdirAll(filepath.Dir(airPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(airPath, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		inspection, err := InspectAirCodexConfig(airPath, "")
		if err != nil {
			for _, forbidden := range []string{root, airPath, "secret.example", "secret-token", "session-id"} {
				if strings.Contains(err.Error(), forbidden) {
					t.Fatalf("inspection error leaked private input %q: %v", forbidden, err)
				}
			}
			return
		}
		encoded, err := json.Marshal(inspection)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{root, airPath, "https://", "secret.example", "secret-token", "session-id"} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatalf("inspection leaked private input %q: %s", forbidden, encoded)
			}
		}
		if hasAirAIGWResidue(text) && !exact && inspection.State != AirStatePartialOrForeignResidue {
			t.Fatalf("non-exact AIGW residue did not fail closed: %#v", inspection)
		}
	})
}

func FuzzAirProjectionCRLFOnlyNormalization(f *testing.F) {
	f.Add(airProjectionFuzzFixture(true, "\n"))
	f.Fuzz(func(t *testing.T, text string) {
		lf, ok := exactAirManagedProjection(text)
		if !ok {
			return
		}
		crlfText := strings.ReplaceAll(text, "\n", "\r\n")
		crlf, ok := exactAirManagedProjection(crlfText)
		if !ok {
			t.Fatal("CRLF-only rewrite changed exact projection admission")
		}
		if lf.fingerprint != crlf.fingerprint {
			t.Fatal("CRLF-only rewrite changed projection fingerprint")
		}
	})
}
