package desktop

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestProjectionPreservesForeignConfigurationAndRestoresOwnedState(t *testing.T) {
	root := t.TempDir()
	paths := PathsForLibrary(filepath.Join(root, "Claude-3p", "configLibrary"))
	writeJSON(t, paths.StandardConfig, map[string]any{"deploymentMode": "1p", "mcpServers": map[string]any{"local": map[string]any{"command": "tool"}}})
	writeJSON(t, paths.ThirdPartyConfig, map[string]any{"theme": "dark"})
	writeJSON(t, paths.Metadata, map[string]any{
		"appliedId": "personal",
		"entries":   []any{map[string]any{"id": "personal", "name": "Personal"}},
		"foreign":   true,
	})

	desired := Desired{
		BaseURL:              "https://gateway.example.test/v1",
		CredentialExecutable: filepath.Join(root, "aigw"),
		CredentialArguments:  []string{"credential", "claude-desktop", "fingerprint"},
		Models: []Model{
			{Name: "claude-fable-5-1", Label: "Fable 5.1"},
			{Name: "claude-opus-5", Label: "Opus 5"},
		},
	}
	plan, err := Prepare(paths, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != ActionProject {
		t.Fatalf("plan action = %q", plan.Action)
	}
	receipt, err := plan.Apply()
	if err != nil {
		t.Fatal(err)
	}

	requireJSONValues(t, paths.StandardConfig, map[string]any{"deploymentMode": "3p"})
	requireJSONKeys(t, paths.StandardConfig, "mcpServers")
	requireJSONValues(t, paths.ThirdPartyConfig, map[string]any{"deploymentMode": "3p", "theme": "dark"})
	requireJSONValues(t, paths.Profile, map[string]any{
		"chatTabEnabled":                true,
		"coworkTabEnabled":              true,
		"inferenceProvider":             "gateway",
		"inferenceGatewayBaseUrl":       desired.BaseURL,
		"inferenceCredentialKind":       "helper-script",
		"inferenceCredentialHelper":     desired.CredentialExecutable,
		"isClaudeCodeForDesktopEnabled": true,
		"modelDiscoveryEnabled":         false,
	})
	requireJSONList(t, paths.Profile, "inferenceCredentialHelperArgs", []any{"credential", "claude-desktop", "fingerprint"})
	requireModelNames(t, paths.Profile, []string{"claude-fable-5-1", "claude-opus-5"})
	requireJSONValues(t, paths.Metadata, map[string]any{"appliedId": profileID, "foreign": true})

	if err := receipt.Rollback(); err != nil {
		t.Fatal(err)
	}
	if got := readJSON(t, paths.StandardConfig); got["deploymentMode"] != "1p" || got["mcpServers"] == nil {
		t.Fatalf("rolled-back standard configuration = %#v", got)
	}
	if got := readJSON(t, paths.ThirdPartyConfig); got["deploymentMode"] != nil || got["theme"] != "dark" {
		t.Fatalf("rolled-back third-party configuration = %#v", got)
	}
	if got := readJSON(t, paths.Metadata); got["appliedId"] != "personal" || got["foreign"] != true {
		t.Fatalf("rolled-back metadata = %#v", got)
	}
	for _, path := range []string{paths.Profile, paths.State} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("rollback left %s: %v", path, err)
		}
	}
}

func TestWithdrawalPreservesLaterForeignConfiguration(t *testing.T) {
	root := t.TempDir()
	paths := PathsForLibrary(filepath.Join(root, "Claude-3p", "configLibrary"))
	desired := Desired{
		BaseURL:              "https://gateway.example.test/v1",
		CredentialExecutable: filepath.Join(root, "aigw"),
		CredentialArguments:  []string{"credential", "claude-desktop", "fingerprint"},
		Models:               []Model{{Name: "claude-fable-5-1"}},
	}
	plan, err := Prepare(paths, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	standard := readJSON(t, paths.StandardConfig)
	standard["mcpServers"] = map[string]any{"local": map[string]any{"command": "tool"}}
	writeJSON(t, paths.StandardConfig, standard)
	metadata := readJSON(t, paths.Metadata)
	metadata["foreign"] = true
	writeJSON(t, paths.Metadata, metadata)

	withdrawal, err := Prepare(paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	if withdrawal.Action != ActionRestore {
		t.Fatalf("withdrawal action = %q", withdrawal.Action)
	}
	if _, err := withdrawal.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := readJSON(t, paths.StandardConfig); got["deploymentMode"] != nil || got["mcpServers"] == nil {
		t.Fatalf("withdrawn standard configuration = %#v", got)
	}
	if got := readJSON(t, paths.Metadata); got["appliedId"] != nil || got["foreign"] != true || got["entries"] != nil {
		t.Fatalf("withdrawn metadata = %#v", got)
	}
	for _, path := range []string{paths.Profile, paths.State, paths.ThirdPartyConfig} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("withdrawal left %s: %v", path, err)
		}
	}
}

func TestUpdatedProjectionRestoresTheOriginalFiles(t *testing.T) {
	root := t.TempDir()
	paths := PathsForLibrary(filepath.Join(root, "Claude-3p", "configLibrary"))
	writeJSON(t, paths.StandardConfig, map[string]any{"deploymentMode": "1p"})
	writeJSON(t, paths.ThirdPartyConfig, map[string]any{})
	writeJSON(t, paths.Metadata, map[string]any{})

	desired := Desired{
		BaseURL:              "https://gateway.example.test/v1",
		CredentialExecutable: filepath.Join(root, "aigw"),
		CredentialArguments:  []string{"credential", "claude-desktop", "fingerprint"},
		Models:               []Model{{Name: "claude-fable-5-1"}},
	}
	plan, err := Prepare(paths, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	desired.BaseURL = "https://replacement.example.test/v1"
	desired.Models = []Model{{Name: "claude-opus-5"}}
	plan, err = Prepare(paths, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}

	withdrawal, err := Prepare(paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := withdrawal.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := readJSON(t, paths.StandardConfig); got["deploymentMode"] != "1p" || len(got) != 1 {
		t.Fatalf("standard configuration = %#v", got)
	}
	for _, path := range []string{paths.ThirdPartyConfig, paths.Metadata} {
		if got := readJSON(t, path); len(got) != 0 {
			t.Fatalf("restored %s = %#v", path, got)
		}
	}
	for _, path := range []string{paths.Profile, paths.State} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("withdrawal left %s: %v", path, err)
		}
	}
}

func TestRollbackRejectsAChangedPostimage(t *testing.T) {
	root := t.TempDir()
	paths := PathsForLibrary(filepath.Join(root, "Claude-3p", "configLibrary"))
	desired := Desired{
		BaseURL:              "https://gateway.example.test/v1",
		CredentialExecutable: filepath.Join(root, "aigw"),
		CredentialArguments:  []string{"credential", "claude-desktop", "fingerprint"},
		Models:               []Model{{Name: "claude-fable-5-1"}},
	}
	plan, err := Prepare(paths, &desired)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := plan.Apply()
	if err != nil {
		t.Fatal(err)
	}
	foreign := []byte("{\n  \"deploymentMode\": \"3p\",\n  \"foreign\": true\n}\n")
	if err := os.WriteFile(paths.StandardConfig, foreign, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := receipt.Rollback(); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("Rollback() error = %v", err)
	}
	if got, err := os.ReadFile(paths.StandardConfig); err != nil || !bytes.Equal(got, foreign) {
		t.Fatalf("foreign postimage = %q, %v", got, err)
	}
}

func TestProjectionRejectsInvalidConfigurationDocuments(t *testing.T) {
	for _, target := range []struct {
		name string
		path func(Paths) string
	}{
		{name: "standard", path: func(paths Paths) string { return paths.StandardConfig }},
		{name: "third-party", path: func(paths Paths) string { return paths.ThirdPartyConfig }},
		{name: "metadata", path: func(paths Paths) string { return paths.Metadata }},
	} {
		t.Run(target.name, func(t *testing.T) {
			root := t.TempDir()
			paths := PathsForLibrary(filepath.Join(root, "Claude-3p", "configLibrary"))
			path := target.path(paths)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("[]\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			desired := Desired{
				BaseURL:              "https://gateway.example.test/v1",
				CredentialExecutable: filepath.Join(root, "aigw"),
				Models:               []Model{{Name: "claude-fable-5-1"}},
			}
			if _, err := Prepare(paths, &desired); err == nil || !strings.Contains(err.Error(), path) {
				t.Fatalf("Prepare() error = %v", err)
			}
		})
	}
}

func TestProjectionRejectsManagedDrift(t *testing.T) {
	root := t.TempDir()
	paths := PathsForLibrary(filepath.Join(root, "Claude-3p", "configLibrary"))
	desired := Desired{
		BaseURL:              "https://gateway.example.test/v1",
		CredentialExecutable: filepath.Join(root, "aigw"),
		CredentialArguments:  []string{"credential", "claude-desktop", "fingerprint"},
		Models:               []Model{{Name: "claude-fable-5-1"}},
	}
	plan, err := Prepare(paths, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	profile := readJSON(t, paths.Profile)
	profile["inferenceGatewayBaseUrl"] = "https://external-edit.example.test/v1"
	writeJSON(t, paths.Profile, profile)
	if _, err := Prepare(paths, &desired); err == nil {
		t.Fatal("managed drift was accepted")
	}
}

func TestWithdrawalWithoutOwnershipIsUnchanged(t *testing.T) {
	paths := PathsForLibrary(filepath.Join(t.TempDir(), "Claude-3p", "configLibrary"))
	plan, err := Prepare(paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != ActionUnchanged {
		t.Fatalf("withdrawal action = %q", plan.Action)
	}
}

func TestProjectionRejectsInvalidDesiredConfiguration(t *testing.T) {
	root := t.TempDir()
	valid := Desired{BaseURL: "https://gateway.example.test/v1", CredentialExecutable: filepath.Join(root, "aigw"), Models: []Model{{Name: "claude-fable-5-1"}}}
	for _, test := range []struct {
		name string
		edit func(*Desired)
		want string
	}{
		{name: "gateway", edit: func(value *Desired) { value.BaseURL = "relative" }, want: "absolute HTTP(S) URL"},
		{name: "helper", edit: func(value *Desired) { value.CredentialExecutable = "aigw" }, want: "absolute path"},
		{name: "models", edit: func(value *Desired) { value.Models = nil }, want: "at least one named model"},
		{name: "blank model", edit: func(value *Desired) { value.Models = []Model{{}} }, want: "at least one named model"},
		{name: "duplicate model", edit: func(value *Desired) { value.Models = []Model{{Name: "same"}, {Name: "same"}} }, want: "non-empty and unique"},
	} {
		t.Run(test.name, func(t *testing.T) {
			desired := valid
			test.edit(&desired)
			if _, err := Prepare(PathsForLibrary(filepath.Join(root, test.name)), &desired); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Prepare() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestProjectionRejectsInvalidOwnershipMetadata(t *testing.T) {
	root := t.TempDir()
	paths := PathsForLibrary(filepath.Join(root, "invalid-state"))
	writeJSON(t, paths.State, map[string]any{"version": 2, "writer_id": "aigw-cli"})
	if _, err := Prepare(paths, &Desired{BaseURL: "https://gateway.example.test", CredentialExecutable: filepath.Join(root, "aigw"), Models: []Model{{Name: "model"}}}); err == nil || !strings.Contains(err.Error(), "ownership state") {
		t.Fatalf("Prepare() error = %v", err)
	}

	paths = PathsForLibrary(filepath.Join(root, "foreign-profile"))
	writeJSON(t, paths.Profile, map[string]any{"inferenceProvider": "foreign"})
	if _, err := Prepare(paths, &Desired{BaseURL: "https://gateway.example.test", CredentialExecutable: filepath.Join(root, "aigw"), Models: []Model{{Name: "model"}}}); err == nil || !strings.Contains(err.Error(), "without AIGW ownership") {
		t.Fatalf("Prepare() error = %v", err)
	}

	paths = PathsForLibrary(filepath.Join(root, "invalid-metadata"))
	writeJSON(t, paths.Metadata, map[string]any{"entries": map[string]any{}})
	if _, err := Prepare(paths, &Desired{BaseURL: "https://gateway.example.test", CredentialExecutable: filepath.Join(root, "aigw"), Models: []Model{{Name: "model"}}}); err == nil || !strings.Contains(err.Error(), "metadata entries") {
		t.Fatalf("Prepare() error = %v", err)
	}
}

func TestProjectionIsIdempotentAndRollsBackPartialApply(t *testing.T) {
	root := t.TempDir()
	paths := PathsForLibrary(filepath.Join(root, "stable", "Claude-3p", "configLibrary"))
	desired := Desired{BaseURL: "https://gateway.example.test", CredentialExecutable: filepath.Join(root, "aigw"), Models: []Model{{Name: "model"}}}
	plan, err := Prepare(paths, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	plan, err = Prepare(paths, &desired)
	if err != nil || plan.Action != ActionUnchanged {
		t.Fatalf("second Prepare() = %q, %v", plan.Action, err)
	}

	paths = PathsForLibrary(filepath.Join(root, "partial", "Claude-3p", "configLibrary"))
	plan, err = Prepare(paths, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.ThirdPartyConfig, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err == nil {
		t.Fatal("partial projection apply succeeded")
	}
	if _, err := os.Stat(paths.StandardConfig); !os.IsNotExist(err) {
		t.Fatalf("partial apply left standard configuration: %v", err)
	}
}

func TestProjectionRejectsUnreadableAndCorruptOwnedState(t *testing.T) {
	root := t.TempDir()
	paths := PathsForLibrary(filepath.Join(root, "capture", "Claude-3p", "configLibrary"))
	if err := os.MkdirAll(paths.StandardConfig, 0o700); err != nil {
		t.Fatal(err)
	}
	desired := Desired{BaseURL: "https://gateway.example.test", CredentialExecutable: filepath.Join(root, "aigw"), Models: []Model{{Name: "model"}}}
	if _, err := Prepare(paths, &desired); err == nil {
		t.Fatal("directory-backed configuration was accepted")
	}

	paths = PathsForLibrary(filepath.Join(root, "owned", "Claude-3p", "configLibrary"))
	plan, err := Prepare(paths, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, paths.Metadata, map[string]any{"entries": map[string]any{}})
	if _, err := Prepare(paths, &desired); err == nil || !strings.Contains(err.Error(), "metadata entries") {
		t.Fatalf("Prepare() error = %v", err)
	}
}

func TestInternalDocumentBoundariesFailClosed(t *testing.T) {
	foreign, _ := json.Marshal(map[string]string{"id": "foreign"})
	owned, _ := json.Marshal(map[string]string{"id": profileID})
	metadata := document{"entries": raw([]json.RawMessage{foreign, owned})}
	if err := removeProfileEntries(metadata); err != nil {
		t.Fatal(err)
	}
	if entries, err := metadataEntries(metadata); err != nil || len(entries) != 1 {
		t.Fatalf("metadata entries = %d, %v", len(entries), err)
	}
	if err := removeProfileEntries(document{"entries": raw(map[string]any{})}); err == nil {
		t.Fatal("invalid metadata entries were accepted")
	}
	if _, err := encode(map[string]any{"unsupported": func() {}}); err == nil {
		t.Fatal("unsupported JSON value was accepted")
	}

	invalid := json.RawMessage("{")
	for _, projected := range []projectedFiles{
		{standard: document{"invalid": invalid}, thirdParty: document{}, metadata: document{}, original: originalState{StandardExists: true, ThirdPartyExists: true, MetadataExists: true}},
		{standard: document{}, thirdParty: document{"invalid": invalid}, metadata: document{}, original: originalState{StandardExists: true, ThirdPartyExists: true, MetadataExists: true}},
		{standard: document{}, thirdParty: document{}, metadata: document{"invalid": invalid}, original: originalState{StandardExists: true, ThirdPartyExists: true, MetadataExists: true}},
	} {
		if _, err := buildPlan(ActionProject, Paths{}, snapshots{}, projected); err == nil {
			t.Fatal("invalid projected document was accepted")
		}
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func requireJSONValues(t *testing.T, path string, want map[string]any) {
	t.Helper()
	got := readJSON(t, path)
	for key, value := range want {
		if !reflect.DeepEqual(got[key], value) {
			t.Fatalf("%s[%q] = %#v, want %#v", path, key, got[key], value)
		}
	}
}

func requireJSONKeys(t *testing.T, path string, keys ...string) {
	t.Helper()
	got := readJSON(t, path)
	for _, key := range keys {
		if _, exists := got[key]; !exists {
			t.Fatalf("%s lacks %q: %#v", path, key, got)
		}
	}
}

func requireJSONList(t *testing.T, path, key string, want []any) {
	t.Helper()
	got := readJSON(t, path)
	values, ok := got[key].([]any)
	if !ok || !slices.Equal(values, want) {
		t.Fatalf("%s[%q] = %#v, want %#v", path, key, got[key], want)
	}
}

func requireModelNames(t *testing.T, path string, want []string) {
	t.Helper()
	profile := readJSON(t, path)
	values, ok := profile["inferenceModels"].([]any)
	if !ok {
		t.Fatalf("%s inferenceModels = %#v", path, profile["inferenceModels"])
	}
	got := make([]string, 0, len(values))
	for _, value := range values {
		model, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("%s model = %#v", path, value)
		}
		name, ok := model["name"].(string)
		if !ok {
			t.Fatalf("%s model name = %#v", path, model["name"])
		}
		got = append(got, name)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("%s model names = %#v, want %#v", path, got, want)
	}
}
