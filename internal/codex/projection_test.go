package codex

import (
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestCodexProjectionRejectsUnrepresentableModelsBeforeWriting(t *testing.T) {
	for _, model := range []string{"model\nname", string([]byte{0xff})} {
		path := writeCodexTestConfig(t, "model_provider = 'native'\nmodel = 'user'\n")
		before, err := transaction.CaptureFileSnapshot(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := SyncConfig(path, catalogTestRuntime(model)); err == nil {
			t.Fatalf("projection admitted invalid model %q", model)
		}
		after, err := transaction.CaptureFileSnapshot(path)
		if err != nil || !before.Equal(after) {
			t.Fatalf("rejected model changed configuration: %v", err)
		}
		entries, err := os.ReadDir(filepath.Dir(path))
		if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
			t.Fatalf("rejected model left projection residue: %v, %v", entries, err)
		}
	}
}

func TestCodexProjectionPreservesLiteralSelectionsAndNestedUserSettings(t *testing.T) {
	for _, original := range []string{
		"# User selects a named profile\n",
		"model_provider = 'host-$native'\nmodel = 'user-${model}'\n",
		"'model_provider' = 'host-$native'\n\"model\" = 'user-${model}'\n",
		"model_provider = '''host-$native'''\nmodel = '''user-${model}'''\n",
		"model_provider = '''host-$native''' # keep provider\nmodel = '''user\n${model}''' # keep model\n",
	} {
		t.Run(original, func(t *testing.T) {
			const nested = "\n[profiles.user]\nmodel_provider = 'user-provider'\nmodel = 'user-model'\nmodel_catalog_json = 'user-catalog.json'\n"
			path := writeCodexTestConfig(t, original+nested)
			runtime := catalogTestRuntime(`model-$route-"quoted"-\path-中文`)
			if err := SyncConfig(path, runtime); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var projected struct {
				Provider string `toml:"model_provider"`
				Model    string `toml:"model"`
			}
			if err := toml.Unmarshal(data, &projected); err != nil || projected.Provider != configuration.ModelProviderAIGW || projected.Model != runtime.Model {
				t.Fatalf("projection changed literal selection: %+v, %v\n%s", projected, err, data)
			}
			if !strings.Contains(string(data), nested) {
				t.Fatalf("projection changed nested user settings:\n%s", data)
			}
			if err := ValidateConfig(path, runtime); err != nil {
				t.Fatal(err)
			}
			if err := DisableConfig(path); err != nil {
				t.Fatal(err)
			}
			if restored, err := os.ReadFile(path); err != nil || string(restored) != original+nested {
				t.Fatalf("withdrawal changed original selections: %v\n%s", err, restored)
			}
		})
	}
}

func TestCodexEndpointRequiresConfiguration(t *testing.T) {
	cases := []struct {
		name     string
		runtime  configuration.Runtime
		expected string
		wantErr  bool
	}{
		{"missing", configuration.Runtime{ProfileID: "p"}, "", true},
		{"valid", configuration.Runtime{ProfileID: "p", Endpoint: "https://example.com/", Authentication: configuration.AuthenticationClientNative}, "https://example.com/", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := codexEndpoint(c.runtime)
			if (err != nil) != c.wantErr {
				t.Fatalf("got err = %v, wantErr %v", err, c.wantErr)
			}
			if got != c.expected {
				t.Errorf("got %q, want %q", got, c.expected)
			}
		})
	}
}

func TestRestoreModelSelectionPreservesOriginalState(t *testing.T) {
	cases := []struct {
		name          string
		base          string
		originalModel string
		expected      string
	}{
		{
			name:          "empty original",
			base:          "model = \"a\" # managed by AIGW\nother = 1",
			originalModel: "",
			expected:      "other = 1",
		},
		{
			name:          "with original, matching line",
			base:          "model = \"a\" # managed by AIGW\nother = 1",
			originalModel: "model = \"b\"",
			expected:      "model = \"b\"\nother = 1",
		},
		{
			name:          "with original, no matching line",
			base:          "other = 1",
			originalModel: "model = \"b\"",
			expected:      "model = \"b\"\nother = 1",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := restoreModelSelection(c.base, c.originalModel)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.expected {
				t.Errorf("got %q, want %q", got, c.expected)
			}
		})
	}
}

func TestProjectCodexRejectsMalformedUserConfiguration(t *testing.T) {
	if _, err := projectCodex("[broken", "", "", "", configuration.ModelProviderAIGW); err == nil || !strings.Contains(err.Error(), "parse Codex config") {
		t.Fatalf("projectCodex() error = %v", err)
	}
}

func TestRemoveCodexProjectionRejectsInvalidCapturedSchedulerState(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	current, err := projectCodex("external = true\n", block, runtime.Model, "", configuration.ModelProviderAIGW)
	if err != nil {
		t.Fatal(err)
	}
	state := codexState{
		ManagedBlockHash:       hashText(block),
		OriginalScheduler:      map[string]*int{"invalid": nil},
		ProjectedSchedulerHash: codexSchedulerHash(current),
	}
	if _, err := removeCodexProjection(current, state); err == nil || !strings.Contains(err.Error(), "invalid Codex scheduler state key") {
		t.Fatalf("removeCodexProjection() error = %v", err)
	}
}

func TestManagedBlockAcceptsCRLFMarkerBoundary(t *testing.T) {
	text := codexBegin + "\r\n[model_providers.aigw]\r\n" + codexEnd + "\r\n"
	block, err := codexManagedBlockIn(text)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(block, "\r\n") {
		t.Fatalf("managed block = %q", block)
	}
}

func TestValidateConfigRejectsIncompleteOrMismatchedProjection(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "configuration.toml")

	err := ValidateConfig(path, configuration.Runtime{ProfileID: "p"})
	if err == nil || !strings.Contains(err.Error(), "no Codex endpoint") {
		t.Errorf("expected endpoint error, got %v", err)
	}

	runtime := atomicTestRuntime()
	runtime.ProfileID = "p"
	runtime.Endpoint = "https://e.t"
	runtime.Model = ""
	err = ValidateConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "read Codex config") {
		t.Errorf("expected read error, got %v", err)
	}

	writeCodexFixture(t, path, "")
	err = ValidateConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "AIGW state is missing") {
		t.Errorf("expected missing state error, got %v", err)
	}

	state := codexState{
		ProjectionMode:   ProjectionFullSelection,
		WriterID:         ProjectionWriterID,
		TransactionID:    "t",
		ManagedBlockHash: "mismatch",
	}
	writeCodexStateFixture(t, path, state)

	block := codexManagedBlock(runtime, runtime.Endpoint)
	content := "model_provider = \"aigw\" # managed by AIGW\n" + codexBegin + "\n" + block
	writeCodexFixture(t, path, content)

	err = ValidateConfig(path, runtime)
	if err == nil || !strings.Contains(err.Error(), "state does not match") {
		t.Errorf("expected hash mismatch error, got %v", err)
	}
}

func TestExactTruncationRejectsShortInput(t *testing.T) {
	if isExactTruncatedCodexProjection("too short", nil, configuration.Runtime{}, "") {
		t.Error("expected false for short input")
	}
}

func TestValidateConfigReachableErrors(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	validState := attributedCodexStateFixture(block)

	t.Run("sidecar is directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		projection, err := projectCodex("", block, runtime.Model, "", configuration.ModelProviderAIGW)
		if err != nil {
			t.Fatal(err)
		}
		writeCodexFixture(t, path, projection)
		if err := os.Mkdir(codexStatePath(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "read Codex adapter state") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})

	t.Run("invalid sidecar", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		projection, err := projectCodex("", block, runtime.Model, "", configuration.ModelProviderAIGW)
		if err != nil {
			t.Fatal(err)
		}
		writeCodexFixture(t, path, projection)
		writeCodexFixture(t, codexStatePath(path), "{")
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "parse Codex adapter state") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})

	t.Run("managed block missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeCodexFixture(t, path, fmt.Sprintf("model = %q # managed by AIGW\n%s\n", runtime.Model, codexSelection))
		writeCodexStateFixture(t, path, validState)
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "provider block is missing") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})

	t.Run("provider profile mismatch", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		otherBlock := codexManagedBlock(runtime, "https://other.example/v1")
		projection, err := projectCodex("", otherBlock, runtime.Model, "", configuration.ModelProviderAIGW)
		if err != nil {
			t.Fatal(err)
		}
		writeCodexFixture(t, path, projection)
		writeCodexStateFixture(t, path, attributedCodexStateFixture(otherBlock))
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "provider block does not match") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})

	t.Run("provider selection mismatch", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		writeCodexFixture(t, path, "model_provider = \"native\"\n"+codexBegin+"\n"+block)
		writeCodexStateFixture(t, path, validState)
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "provider selection does not match") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})

	t.Run("scheduler key missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		projection, err := projectCodex("", block, runtime.Model, "", configuration.ModelProviderAIGW)
		if err != nil {
			t.Fatal(err)
		}
		projection = strings.Replace(projection, "max_depth = 1 # managed by AIGW\n", "", 1)
		writeCodexFixture(t, path, projection)
		writeCodexStateFixture(t, path, validState)
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "scheduler key") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})

	t.Run("scheduler state hash mismatch", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		projection, err := projectCodex("", block, runtime.Model, "", configuration.ModelProviderAIGW)
		if err != nil {
			t.Fatal(err)
		}
		state := validState
		state.ProjectedSchedulerHash = "changed"
		writeCodexFixture(t, path, projection)
		writeCodexStateFixture(t, path, state)
		if err := ValidateConfig(path, runtime); err == nil || !strings.Contains(err.Error(), "scheduler keys changed") {
			t.Fatalf("ValidateConfig() error = %v", err)
		}
	})
}

func TestCodexUserConfigRejectsInvalidState(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	config := transaction.FileSnapshot{Exists: true, Data: []byte("external = true\n")}
	state := transaction.FileSnapshot{Exists: true, Data: []byte("{")}
	if _, _, err := codexUserConfig(config, state, runtime, block); err == nil || !strings.Contains(err.Error(), "parse Codex adapter state") {
		t.Fatalf("codexUserConfig() error = %v", err)
	}
}

func TestCodexUserConfigRejectsInvalidCapturedSchedulerState(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	projection, err := projectCodex("external = true\n", block, runtime.Model, "", configuration.ModelProviderAIGW)
	if err != nil {
		t.Fatal(err)
	}
	state := attributedCodexStateFixture(block)
	state.OriginalScheduler = map[string]*int{"invalid": nil}
	stateData, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	config := transaction.FileSnapshot{Exists: true, Data: []byte(projection)}
	sidecar := transaction.FileSnapshot{Exists: true, Data: stateData}
	if _, _, err := codexUserConfig(config, sidecar, runtime, block); err == nil || !strings.Contains(err.Error(), "invalid Codex scheduler state key") {
		t.Fatalf("codexUserConfig() error = %v", err)
	}
}

func TestCompleteExactTruncatedCodexProjectionRejectsAmbiguities(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	state := codexState{
		ManagedBlockHash: hashText(block),
		OriginalScheduler: map[string]*int{
			"agents.max_concurrent_threads_per_session":                  nil,
			"agents.max_depth":                                           nil,
			"features.multi_agent_v2.max_concurrent_threads_per_session": nil,
		},
	}
	truncated := strings.TrimSuffix(block, codexEnd+"\n")

	for _, test := range []struct {
		name    string
		current string
		state   codexState
	}{
		{
			name:    "selection is not managed",
			current: `model_provider = "native"` + "\n" + codexBegin + "\n" + truncated,
			state:   state,
		},
		{
			name:    "managed model differs",
			current: codexSelection + "\n" + `model = "other" # managed by AIGW` + "\n" + codexBegin + "\n" + truncated,
			state:   state,
		},
		{
			name:    "provider table missing",
			current: codexSelection + "\n" + codexBegin + "\n",
			state:   state,
		},
		{
			name:    "completion marker already present",
			current: codexSelection + "\n" + fmt.Sprintf("model = %q # managed by AIGW\n", runtime.Model) + codexBegin + "\n" + block,
			state:   state,
		},
		{
			name:    "truncated bytes mismatch",
			current: codexSelection + "\n" + codexBegin + "\n[model_providers.aigw]\nchanged = true\n",
			state:   state,
		},
		{
			name:    "state block hash missing",
			current: codexSelection + "\n" + fmt.Sprintf("model = %q # managed by AIGW\n", runtime.Model) + codexBegin + "\n" + truncated,
			state:   codexState{},
		},
		{
			name:    "foreign content before next table",
			current: codexSelection + "\n" + codexBegin + "\n" + truncated + "foreign = true\n[other]\n",
			state:   state,
		},
		{
			name:    "foreign trailing content",
			current: codexSelection + "\n" + fmt.Sprintf("model = %q # managed by AIGW\n", runtime.Model) + codexBegin + "\n" + truncated + "foreign = true\n",
			state:   state,
		},
		{
			name:    "blank lines before next table are recoverable",
			current: codexSelection + "\n" + fmt.Sprintf("model = %q # managed by AIGW\n", runtime.Model) + codexBegin + "\n" + truncated + "\n[next]\nvalue = 1\n",
			state:   state,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			completed, ok := completeExactTruncatedCodexProjection(test.current, test.state, runtime, block)
			if test.name == "blank lines before next table are recoverable" {
				if !ok || !strings.Contains(completed, codexEnd+"\n[next]") {
					t.Fatalf("completeExactTruncatedCodexProjection() = %t:\n%s", ok, completed)
				}
				return
			}
			if ok {
				t.Fatalf("completeExactTruncatedCodexProjection() admitted:\n%s", completed)
			}
		})
	}
}

func TestCompleteExactTruncatedProjectionRejectsForeignContentAfterValidPrefix(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	state := codexState{ManagedBlockHash: hashText(block)}
	truncated := strings.TrimSuffix(block, codexEnd+"\n")
	current := codexSelection + "\n" + fmt.Sprintf("model = %q # managed by AIGW\n", runtime.Model) + codexBegin + "\n" + truncated + "foreign = true\n[other]\n"
	if completed, ok := completeExactTruncatedCodexProjection(current, state, runtime, block); ok {
		t.Fatalf("foreign content was admitted:\n%s", completed)
	}
}

func TestRemoveCodexProjectionRestoresAbsentProvider(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	current, err := projectCodex("external = true\n", block, runtime.Model, "", configuration.ModelProviderAIGW)
	if err != nil {
		t.Fatal(err)
	}
	state := attributedCodexStateFixture(block)
	state.OriginalModel = `model = "native-model"`

	restored, err := removeCodexProjection(current, state)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(restored, "model_provider") || !strings.Contains(restored, `model = "native-model"`) || !strings.Contains(restored, "external = true") {
		t.Fatalf("restored config = %q", restored)
	}
	if got := removeCodexBeginMarker("external = true\n"); got != "external = true\n" {
		t.Fatalf("removeCodexBeginMarker() = %q", got)
	}
}

func TestRemoveCodexProjectionRejectsIncompleteSchedulerState(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	current, err := projectCodex("external = true\n", block, runtime.Model, "", configuration.ModelProviderAIGW)
	if err != nil {
		t.Fatal(err)
	}
	state := attributedCodexStateFixture(block)
	state.OriginalScheduler = map[string]*int{
		"agents.max_concurrent_threads_per_session":                  nil,
		"agents.max_depth":                                           nil,
		"features.multi_agent_v2.max_concurrent_threads_per_session": nil,
	}

	if _, err := removeCodexProjection(current, state); err == nil || !strings.Contains(err.Error(), "incomplete Codex scheduler state") {
		t.Fatalf("removeCodexProjection() error = %v", err)
	}
}

func TestRemoveCodexProjectionRejectsChangedManagedBlock(t *testing.T) {
	runtime := atomicTestRuntime()
	block := codexManagedBlock(runtime, runtime.Endpoint)
	current, err := projectCodex("external = true\n", block, runtime.Model, "", configuration.ModelProviderAIGW)
	if err != nil {
		t.Fatal(err)
	}
	state := codexState{ManagedBlockHash: hashText(block)}
	current = strings.Replace(current, runtime.Endpoint, "https://changed.example/v1", 1)

	if _, err := removeCodexProjection(current, state); err == nil || !strings.Contains(err.Error(), "provider block changed") {
		t.Fatalf("removeCodexProjection() error = %v", err)
	}
}
