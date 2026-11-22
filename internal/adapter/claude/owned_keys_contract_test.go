package claude

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/platform"
)

// The published 0.8.3 reads the first record with unknown fields refused and
// computes its digest from the four environment values, apiKeyHelper and
// model. Each case here is a mutation that would have brought the 2026-11-21
// failure back: a field in the first record's shape, a digest taken over the
// file's bytes rather than the value, or a record that drops an entry it does
// not know.
func TestPublishedPredecessorContractHoldsThroughTheRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := overridesRuntime()
	if _, err := ReconcileSettings(path, false, runtime, testExecutable(), ""); err != nil {
		t.Fatal(err)
	}

	t.Run("a new field in the first record is refused", func(t *testing.T) {
		record := readFileBytes(t, path+adapter.StateSuffix)
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(record, &fields); err != nil {
			t.Fatal(err)
		}
		var original map[string]json.RawMessage
		if err := json.Unmarshal(fields["original"], &original); err != nil {
			t.Fatal(err)
		}
		original["model_overrides"] = json.RawMessage(`{"present":false}`)
		mutatedOriginal, err := json.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		fields["original"] = mutatedOriginal
		mutated, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		if err := decodeAsPublished(mutated); err == nil || !strings.Contains(err.Error(), "model_overrides") {
			t.Fatalf("the published reader = %v, want the unknown field", err)
		}
		if err := decodeAsPublished(record); err != nil {
			t.Fatalf("the published reader refuses the record this program wrote: %v", err)
		}
	})

	t.Run("a digest of the file's bytes would not survive re-indentation", func(t *testing.T) {
		var recorded struct {
			WrittenSHA256 string `json:"written_sha256"`
		}
		if err := json.Unmarshal(readOwnedKeysFile(t, path)["modelOverrides"], &recorded); err != nil {
			t.Fatal(err)
		}
		value := readSettingsFile(t, path)["modelOverrides"]
		if want := canonicalDigest(t, value); recorded.WrittenSHA256 != want {
			t.Fatalf("the record digests %q, want the value's canonical digest %q", recorded.WrittenSHA256, want)
		}
		reindented := []byte("{\n\t\"claude-fable-5-2\": \"claude-code-fable-5-2\",\n\t\"claude-opus-5-5\": \"claude-code-opus-5-5\"\n}")
		if recorded.WrittenSHA256 == platform.SHA256(reindented) {
			t.Fatal("the digest is taken over the file's bytes, so the predecessor's re-indentation loses the map")
		}
	})

	t.Run("a record that drops an entry it does not know loses it", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		stranger := `{"original":{"present":false},"written_sha256":"feed"}`
		record := `{"version":1,"writer_id":"aigw-cli","keys":{"modelContextLimits":` + stranger + `}}`
		if err := os.WriteFile(ownedKeysPath(path), []byte(record), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"theme":"dark","modelContextLimits":{"some":"value"}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReconcileSettings(path, false, overridesRuntime(), testExecutable(), ""); err != nil {
			t.Fatal(err)
		}
		keys := readOwnedKeysFile(t, path)
		if _, kept := keys["modelContextLimits"]; !kept {
			t.Fatal("the record dropped an entry that this program does not know, so its setting would be lost with the record")
		}
		if _, written := keys["modelOverrides"]; !written {
			t.Fatal("the record dropped the entry for the map that the projection wrote")
		}
	})
}

// decodeAsPublished decodes data as the published 0.8.3 decodes the first
// record: unknown fields refused at every struct level.
func decodeAsPublished(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var published publishedSettingsState
	return decoder.Decode(&published)
}

// publishedSettingsState is AIGW's record of Claude Code's settings as the
// published 0.8.3 reads it, copied from its source at v0.8.3
// (internal/adapter/claude/settings.go:35-52): it refuses an unknown field at
// every level, so this struct is the frozen published contract, and it is
// declared here on purpose. The digest below recomputes what that release
// computes (:28-33, :495-517).
type publishedSettingsState struct {
	Version       int               `json:"version"`
	WriterID      string            `json:"writer_id"`
	Original      publishedOriginal `json:"original"`
	ManagedSHA256 string            `json:"managed_sha256"`
}

type publishedOriginal struct {
	FileExisted  bool                         `json:"file_existed"`
	APIKeyHelper publishedOptional            `json:"api_key_helper"`
	Model        publishedOptional            `json:"model"`
	Environment  map[string]publishedOptional `json:"environment,omitempty"`
}

type publishedOptional struct {
	Present bool            `json:"present"`
	Value   json.RawMessage `json:"value,omitempty"`
}

// publishedManagedSettingsHash is the digest that the published 0.8.3 takes
// of the settings it read: a flat map of the four environment values that
// AIGW manages, apiKeyHelper and model, each present value re-marshalled,
// and nothing else.
func publishedManagedSettingsHash(t *testing.T, document settingsDocument) string {
	t.Helper()
	managed := map[string]json.RawMessage{}
	environment, err := decodeEnvironment(document)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL", "ANTHROPIC_MODEL"} {
		if value, ok := environment[key]; ok {
			managed[key] = value
		}
	}
	for _, key := range []string{"apiKeyHelper", "model"} {
		if value, ok := document[key]; ok {
			managed[key] = value
		}
	}
	for key, raw := range managed {
		var value string
		if bytes.HasPrefix(bytes.TrimSpace(raw), []byte(`"`)) && json.Unmarshal(raw, &value) == nil {
			reencoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			managed[key] = reencoded
		}
	}
	canonical, err := json.Marshal(managed)
	if err != nil {
		t.Fatal(err)
	}
	return platform.SHA256(canonical)
}

// A record that the current code writes with a map in the settings decodes
// under the published 0.8.3's strict reader, and its digest is that release's
// function of the settings: the shape of the first record is a published
// contract.
func TestSettingsRecordReadsAsThePublishedPredecessorReadsIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := overridesRuntime()
	if _, err := ReconcileSettings(path, false, runtime, testExecutable(), ""); err != nil {
		t.Fatal(err)
	}
	if _, present := writtenOverrides(t, path); !present {
		t.Fatal("the fixture wrote no map, so the record under test is not the one this test is about")
	}
	decoder := json.NewDecoder(bytes.NewReader(readFileBytes(t, path+adapter.StateSuffix)))
	decoder.DisallowUnknownFields()
	var published publishedSettingsState
	if err := decoder.Decode(&published); err != nil {
		t.Fatalf("the published predecessor refuses this record: %v", err)
	}
	if published.Version != 1 || published.WriterID != "aigw-cli" || published.ManagedSHA256 == "" {
		t.Fatalf("the published predecessor reads the record as incomplete or not owned: %+v", published)
	}
	if want := publishedManagedSettingsHash(t, readSettingsFile(t, path)); published.ManagedSHA256 != want {
		t.Fatalf("managed_sha256 = %q, want the published digest %q", published.ManagedSHA256, want)
	}
}
