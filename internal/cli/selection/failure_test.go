package selection

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/client"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
)

func TestUseSurfacesTokenStoreAndOutputFailures(t *testing.T) {
	runtime, _, _ := configuredRuntime(t)
	runtime.Secrets = failingSecretStore{Store: secrets.NewMemoryStore(), setErr: errors.New("store failed")}
	runtime.Interactive = true
	runtime.Prompt = &promptStub{secret: "token"}
	runtime.HTTP = doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})
	command := NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "store failed") {
		t.Fatalf("store error = %v", err)
	}

	runtime, _, _ = configuredRuntime(t)
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = store
	runtime.RenderOut = failingWriter{err: errors.New("write failed")}
	command = NewUseCommand(runtime)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("output error = %v", err)
	}
}

type failingSecretStore struct {
	secrets.Store
	getErr    error
	setErr    error
	deleteErr error
}

func (store failingSecretStore) Get(account string) (string, error) {
	if store.getErr != nil {
		return "", store.getErr
	}
	return store.Store.Get(account)
}

func (store failingSecretStore) Set(account, token string) error {
	if store.setErr != nil {
		return store.setErr
	}
	return store.Store.Set(account, token)
}

func (store failingSecretStore) Delete(account string) error {
	if store.deleteErr != nil {
		return store.deleteErr
	}
	return store.Store.Delete(account)
}

type failingWriter struct{ err error }

func (writer failingWriter) Write([]byte) (int, error) { return 0, writer.err }

func TestCodexForwardingPreviewAndDirectPreserveUpstream(t *testing.T) {
	app, cfg, out := configuredRuntime(t)
	binding := cfg.Clients[configuration.ClientCodex]
	binding.Enabled = true
	cfg.Clients[configuration.ClientCodex] = binding
	cfg.Clients[configuration.ClientHermes] = configuration.ClientBinding{
		Route: "codex", Protocol: configuration.ProtocolOpenAIResponses,
		CredentialCommand: filepath.Join(t.TempDir(), "hermes-reader"),
	}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	direct, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	credentials := secrets.NewMemoryStore()
	if err := credentials.Set("gateway", "synthetic"); err != nil {
		t.Fatal(err)
	}
	app.Secrets = failingSecretStore{Store: credentials, getErr: errors.New("preview read a credential")}
	const endpoint = "http://127.0.0.1:8792/selected/v1"
	command := NewUseCommand(app)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex", "--forwarding-endpoint", endpoint, "--dry-run", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatalf("forwarding preview: %v", err)
	}
	var preview struct {
		DryRun           bool   `json:"dry_run"`
		Endpoint         string `json:"endpoint"`
		UpstreamEndpoint string `json:"upstream_endpoint"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || preview.Endpoint != endpoint || preview.UpstreamEndpoint != direct.Endpoint {
		t.Fatalf("forwarding preview = %#v", preview)
	}
	unchanged, err := app.Config.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, unchanged) {
		t.Fatalf("preview changed configuration or recovery inputs: %v", err)
	}
	app.Secrets = credentials
	for _, arguments := range [][]string{
		{"--forwarding-endpoint", endpoint},
		{"--direct"},
	} {
		out.Reset()
		command := NewUseCommand(app)
		command.SilenceErrors, command.SilenceUsage = true, true
		command.SetArgs(append([]string{"--for", configuration.ClientCodex, "codex"}, arguments...))
		if err := command.Execute(); err != nil {
			t.Fatalf("selection %q: %v", arguments, err)
		}
		want := endpoint
		if arguments[0] == "--direct" {
			want = direct.Endpoint
		}
		requireCodexSelectionPreserved(t, app.Config, cfg, before, direct, want)
	}
	component := strings.TrimSuffix(app.Config.Path(), filepath.Ext(app.Config.Path())) + ".forwarding.toml"
	if _, err := os.Stat(component); !os.IsNotExist(err) {
		t.Fatalf("direct selection left an active forwarding component: %v", err)
	}
}

func requireCodexSelectionPreserved(t *testing.T, store configuration.Store, before configuration.Config, snapshot configuration.Snapshot, direct configuration.Runtime, endpoint string) {
	t.Helper()
	current, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	selected, err := current.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if selected.Endpoint != endpoint || selected.CredentialProjectionFingerprint(configuration.ClientCodex) != direct.CredentialProjectionFingerprint(configuration.ClientCodex) {
		t.Fatalf("selection changed its destination or original reader identity: %#v", selected)
	}
	if !reflect.DeepEqual(current.Accounts, before.Accounts) || !reflect.DeepEqual(current.Clients[configuration.ClientHermes], before.Clients[configuration.ClientHermes]) || current.Clients[configuration.ClientCodex].CredentialCommand != before.Clients[configuration.ClientCodex].CredentialCommand {
		t.Fatal("forwarding changed the Account, Hermes or original reader")
	}
	main, err := os.ReadFile(store.Path())
	if err != nil || !bytes.Equal(main, snapshot.Config.Data) || bytes.Contains(main, []byte("forwarding_endpoint")) {
		t.Fatalf("forwarding changed the predecessor-readable main configuration: %v", err)
	}
}

func TestCodexForwardingPreviewPlansOwnedTargetBeforeCredentialReads(t *testing.T) {
	app, cfg, out := configuredRuntime(t)
	target := filepath.Join(t.TempDir(), "config.toml")
	const original = "# User settings\nmodel_reasoning_effort = \"medium\"\n"
	if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	binding := cfg.Clients[configuration.ClientCodex]
	binding.Enabled, binding.Targets = true, []string{target}
	binding.CredentialCommand = filepath.Join(t.TempDir(), "codex-reader")
	cfg.Clients[configuration.ClientCodex] = binding
	claudeTarget := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(claudeTarget, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Routes["claude"] = configuration.Route{
		Account: "gateway", Model: "claude-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{
			configuration.ProtocolAnthropic: {},
		},
	}
	cfg.Clients[configuration.ClientClaude] = configuration.ClientBinding{
		Route: "claude", Enabled: true, Protocol: configuration.ProtocolAnthropic,
	}
	app.ClaudeSettingsPath = claudeTarget
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	app.Secrets = failingSecretStore{Store: secrets.NewMemoryStore(), getErr: errors.New("preview read a credential")}
	app.HTTP = doerFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("preview sent a request")
		return nil, errors.New("preview sent a request")
	})
	command := NewUseCommand(app)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1", "--dry-run", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatalf("preview inspected another client or read credentials: %v", err)
	}
	var preview struct {
		Projections []struct {
			Client string `json:"client"`
			Target string `json:"target"`
			Action string `json:"action"`
		} `json:"projections"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Projections) != 1 || preview.Projections[0].Client != configuration.ClientCodex ||
		preview.Projections[0].Target != target || preview.Projections[0].Action != "initial-project" {
		t.Fatalf("preview did not expose its exact target transition: %#v", preview)
	}
	out.Reset()
	command = NewUseCommand(app)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1", "--dry-run"})
	if err := command.Execute(); err != nil {
		t.Fatalf("human preview inspected another client or read credentials: %v", err)
	}
	for _, want := range []string{"Selection preview", filepath.Base(target), "initial-project", "http://127.0.0.1:8792/v1", "https://gateway.example.test/v1"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("human preview omitted %q: %s", want, out.String())
		}
	}
	after, err := app.Config.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("preview changed configuration or recovery inputs: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != original {
		t.Fatalf("preview changed native bytes: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil || len(entries) != 1 {
		t.Fatalf("preview created projection artifacts: %v, %v", entries, err)
	}
}

func TestCodexForwardingPreviewRejectsOwnedTargetConflict(t *testing.T) {
	app, cfg, out := configuredRuntime(t)
	target := filepath.Join(t.TempDir(), "config.toml")
	binding := cfg.Clients[configuration.ClientCodex]
	binding.Enabled, binding.Targets = true, []string{target}
	binding.CredentialCommand = filepath.Join(t.TempDir(), "codex-reader")
	cfg.Clients[configuration.ClientCodex] = binding
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	selected, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(target, selected); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(original, []byte("https://gateway.example.test/v1"), []byte("https://user.example.test/v1"), 1)
	if bytes.Equal(original, changed) {
		t.Fatal("fixture did not change the owned endpoint")
	}
	if err := os.WriteFile(target, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	owned := make(map[string][]byte)
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		path := filepath.Join(filepath.Dir(target), entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		owned[path] = data
	}
	app.Secrets = failingSecretStore{Store: secrets.NewMemoryStore(), getErr: errors.New("preview read a credential")}
	command := NewUseCommand(app)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1", "--dry-run", "--json"})
	if err := command.Execute(); err == nil {
		t.Fatalf("preview admitted a conflicting native target: %s", out.String())
	}
	after, err := app.Config.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("refusal changed configuration or recovery inputs: %v", err)
	}
	for path, data := range owned {
		actual, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(data, actual) {
			t.Fatalf("refusal changed projection %s: %v", path, err)
		}
	}
}

func TestInvalidSelectionFlagsPreserveRecoveryInputs(t *testing.T) {
	app, _, _ := configuredRuntime(t)
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"--for", "codex", "codex", "--direct", "--forwarding-endpoint="}, "mutually exclusive"},
		{[]string{"--for", "codex", "codex", "--json"}, "--json requires --dry-run"},
		{nil, "non-interactive use requires a Route"},
		{[]string{"codex"}, "non-interactive use requires --for"},
		{[]string{"--for", "unknown", "codex"}, "--for must be"},
		{[]string{"--for", "codex", "codex", "--forwarding-endpoint="}, "use --direct to remove forwarding"},
	} {
		command := NewUseCommand(app)
		command.SilenceErrors, command.SilenceUsage = true, true
		command.SetArgs(test.args)
		if err := command.Execute(); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("invalid selection %v: %v", test.args, err)
		}
	}
	after, err := app.Config.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("invalid flags changed recovery inputs: %v", err)
	}
}

func TestCodexForwardingPreviewIncludesNewlyDiscoveredTarget(t *testing.T) {
	app, cfg, out := configuredRuntime(t)
	target := filepath.Join(t.TempDir(), "config.toml")
	executable := filepath.Join(t.TempDir(), "codex")
	app.Discovery = staticDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: executable},
		Surfaces: []discovery.Surface{{
			ID: string(surfaceidentity.CodexHomeDefault), Authority: string(surfaceidentity.AuthorityAIGW),
			ConfigPath: target, AutoManaged: true,
		}},
	}}
	binding := cfg.Clients[configuration.ClientCodex]
	binding.Enabled, binding.CredentialCommand = true, filepath.Join(t.TempDir(), "reader")
	cfg.Clients[configuration.ClientCodex] = binding
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	app.Secrets = failingSecretStore{Store: secrets.NewMemoryStore(), getErr: errors.New("preview read a credential")}
	command := NewUseCommand(app)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1", "--dry-run", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Projections []client.ProjectionPlan `json:"projections"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Projections) != 1 || preview.Projections[0].Target != target ||
		preview.Projections[0].Client != configuration.ClientCodex || preview.Projections[0].Action != "initial-project" {
		t.Fatalf("preview missed a newly discovered native target: %#v", preview)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview created the discovered target: %v", err)
	}
	credentials := secrets.NewMemoryStore()
	if err := credentials.Set("gateway", "synthetic"); err != nil {
		t.Fatal(err)
	}
	app.Secrets = credentials
	command = NewUseCommand(app)
	command.SilenceErrors, command.SilenceUsage = true, true
	command.SetArgs([]string{"--for", configuration.ClientCodex, "codex", "--forwarding-endpoint", "http://127.0.0.1:8792/v1"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	current, err := app.Config.Load()
	if err != nil || len(current.Clients[configuration.ClientCodex].Targets) != 1 ||
		current.Clients[configuration.ClientCodex].Targets[0] != preview.Projections[0].Target {
		t.Fatalf("selection did not apply its exact discovered preview: %v", err)
	}
}
