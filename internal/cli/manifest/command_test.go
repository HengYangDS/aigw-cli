package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"

	"github.com/spf13/cobra"
)

const importManifest = `version = 7

[recommendations.codex.primary]
route = "remote"

[accounts.gateway]
label = "Gateway"

[accounts.gateway.endpoints]
openai_responses = "https://gateway.example/v1"

[models.gpt-remote]
label = "GPT Remote"

[routes.remote]
label = "Remote"
account = "gateway"
model = "gpt-remote"
upstream_model = "gpt-remote"
interfaces = { openai_responses = ["text"] }
`

func executeManifestCommand(command *cobra.Command) error {
	command.SilenceErrors = true
	command.SilenceUsage = true
	return command.Execute()
}

func TestCommandRequiresAConfigurationOperation(t *testing.T) {
	command := NewCommand(invocation.Context{Out: io.Discard})
	if err := command.RunE(command, nil); err == nil || !strings.Contains(err.Error(), "Choose a config subcommand") {
		t.Fatalf("missing-operation error = %v", err)
	}
	if err := command.RunE(command, []string{"unknown"}); err == nil || !strings.Contains(err.Error(), `Unknown config subcommand "unknown"`) {
		t.Fatalf("unknown-operation error = %v", err)
	}
}

func TestConfigurationCommandTreeMatchesItsManifestResponsibility(t *testing.T) {
	command := NewCommand(invocation.Context{Out: io.Discard})
	var names []string
	for _, child := range command.Commands() {
		names = append(names, child.Name())
		if child.RunE == nil || child.Args == nil {
			t.Errorf("configuration operation %q lacks execution or argument admission", child.Name())
		}
	}
	if want := []string{"export", "import", "migrate", "path"}; !slices.Equal(names, want) {
		t.Fatalf("configuration operations = %q, want %q", names, want)
	}
}

func TestPathPrintsTheConfiguredStorePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	out := &bytes.Buffer{}
	command := NewCommand(invocation.Context{Config: configuration.NewStore(path), Out: out})
	command.SetArgs([]string{"path"})
	if err := executeManifestCommand(command); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != path {
		t.Fatalf("path output = %q, want %q", got, path)
	}
}

func TestPathReturnsOutputFailure(t *testing.T) {
	command := newPathCommand(invocation.Context{Config: configuration.NewStore("configuration.toml"), Out: failingWriter{}})
	if err := executeManifestCommand(command); err == nil || !strings.Contains(err.Error(), "write refused") {
		t.Fatalf("error = %v", err)
	}
}

func TestExportWritesASecretFreeRoundTripManifest(t *testing.T) {
	runtime, _, out, _ := savedRuntime(t, localConfig())
	secretStore := runtime.Secrets
	if err := secretStore.Set("local", "must-not-appear"); err != nil {
		t.Fatal(err)
	}
	command := NewCommand(runtime)
	command.SetArgs([]string{"export"})
	if err := executeManifestCommand(command); err != nil {
		t.Fatal(err)
	}
	data := out.Bytes()
	if bytes.Contains(data, []byte("must-not-appear")) {
		t.Fatal("export leaked a system secret")
	}
	parsed, err := configuration.Parse(data)
	if err != nil {
		t.Fatalf("parse exported manifest: %v\n%s", err, data)
	}
	if parsed.Recommendations[configuration.ClientCodex].Primary.Route != "local" || parsed.Routes["local"].Account != "local" {
		t.Fatalf("exported manifest = %#v", parsed)
	}
}

func TestExportSurfacesLoadAndOutputFailures(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "configuration.toml")
		if err := os.WriteFile(path, []byte("not = [valid"), 0o600); err != nil {
			t.Fatal(err)
		}
		command := newExportCommand(invocation.Context{Config: configuration.NewStore(path), Out: io.Discard})
		if err := executeManifestCommand(command); err == nil {
			t.Fatal("expected malformed configuration to fail")
		}
	})

	t.Run("output", func(t *testing.T) {
		runtime, _, _, _ := savedRuntime(t, localConfig())
		runtime.Out = failingWriter{}
		if err := executeManifestCommand(newExportCommand(runtime)); err == nil || !strings.Contains(err.Error(), "write refused") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestExportSurfacesManifestValidationFailure(t *testing.T) {
	store := configuration.NewStore(filepath.Join(t.TempDir(), "missing.toml"))
	command := newExportCommand(invocation.Context{Config: store, Out: io.Discard})
	if err := executeManifestCommand(command); err == nil || !strings.Contains(err.Error(), "at least one route") {
		t.Fatalf("error = %v", err)
	}
}

func TestImportMergesPublicConfigurationAndDefersReadiness(t *testing.T) {
	runtime, path, _, renderOut := savedRuntime(t, localConfig())
	manifestPath := writeManifest(t, importManifest)
	command := NewCommand(runtime)
	command.SetArgs([]string{"import", manifestPath})
	if err := executeManifestCommand(command); err != nil {
		t.Fatal(err)
	}

	loaded, err := configuration.NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Routes["remote"].Account != "gateway" {
		t.Fatalf("imported config = %#v", loaded)
	}
	output := renderOut.String()
	for _, want := range []string{"Configuration manifest imported", "Routes", "Accounts", "aigw status"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output %q does not contain %q", output, want)
		}
	}
	if strings.Contains(output, "Token") || strings.Contains(output, "aigw sync") {
		t.Fatalf("configuration import claimed credential or projection readiness: %q", output)
	}
}

func TestImportPreservesExplicitSelectionWithoutSuggestingNoOpSync(t *testing.T) {
	runtime, _, _, renderOut := savedRuntime(t, localConfig())
	if err := runtime.Secrets.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	command := newImportCommand(runtime)
	command.SetArgs([]string{writeManifest(t, importManifest)})
	if err := executeManifestCommand(command); err != nil {
		t.Fatal(err)
	}
	cfg, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.SelectedRoute(configuration.ClientCodex); got != "local" {
		t.Fatalf("explicit Codex Route changed to %q", got)
	}
	if output := strings.TrimSpace(renderOut.String()); !strings.HasSuffix(output, "aigw status") || strings.Contains(output, "Next\n  aigw sync") {
		t.Fatalf("import suggested activation without inspecting the selected Route: %q", output)
	}
}

func TestImportPreviewReportsAffectedClientsWithoutReadingTokensOrWriting(t *testing.T) {
	cfg := localConfig()
	account := cfg.Accounts["local"]
	account.Endpoints.OpenAIResponses = "http://127.0.0.1:8792/local/v1"
	cfg.Accounts["local"] = account
	cfg.Routes["old"] = configuration.Route{
		Label: "Old", Account: "local", Model: "gpt-old",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
	}
	cfg.Models["gpt-old"] = configuration.Model{Label: "Old"}
	cfg.SetSelectedRoute(configuration.ClientHermes, "local")
	clientTarget := filepath.Join(t.TempDir(), "hermes.yaml")
	cfg.SetClientActivation(configuration.ClientHermes, true, "/opt/hermes", []string{clientTarget})
	runtime, path, out, renderOut := savedRuntime(t, cfg)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	reads := 0
	runtime.Secrets = secrets.NewEnvironmentStore(func(string) string { reads++; return "" })
	incoming := strings.ReplaceAll(importManifest, "gateway", "local")
	incoming = strings.ReplaceAll(incoming, "Gateway", "Local")
	manifestPath := writeManifest(t, incoming)
	command := newImportCommand(runtime)
	command.SetArgs([]string{manifestPath, "--keep-account", "local", "--retire-route", "old", "--dry-run", "--json"})
	if err := executeManifestCommand(command); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		DryRun               bool     `json:"dry_run"`
		ImportedAccounts     int      `json:"imported_accounts"`
		ImportedRoutes       int      `json:"imported_routes"`
		KeptAccounts         []string `json:"kept_accounts"`
		RetiredRoutes        []string `json:"retired_routes"`
		ProjectionCandidates []string `json:"projection_candidates"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || preview.ImportedAccounts != 1 || preview.ImportedRoutes != 1 ||
		!slices.Equal(preview.KeptAccounts, []string{"local"}) ||
		!slices.Equal(preview.RetiredRoutes, []string{"old"}) ||
		!slices.Contains(preview.ProjectionCandidates, configuration.ClientHermes) {
		t.Fatalf("import preview = %+v", preview)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) || reads != 0 || renderOut.Len() != 0 {
		t.Fatalf("preview changed state or read Tokens: config=%t reads=%d render=%q error=%v", bytes.Equal(before, after), reads, renderOut.String(), err)
	}
	if _, err := os.Stat(clientTarget); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview wrote client projection: %v", err)
	}
	human := newImportCommand(runtime)
	human.SetArgs([]string{manifestPath, "--keep-account", "local", "--retire-route", "old", "--dry-run"})
	if err := executeManifestCommand(human); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(renderOut.String(), "Configuration import preview") || !strings.Contains(renderOut.String(), "Potential client projections") || reads != 0 {
		t.Fatalf("human preview omitted the impact or read Tokens: %q, reads=%d", renderOut.String(), reads)
	}
}

func TestImportDoesNotInspectUnselectedAccountCredentials(t *testing.T) {
	runtime, _, _, _ := savedRuntime(t, localConfig())
	reads := 0
	runtime.Secrets = secrets.NewEnvironmentStore(func(string) string {
		reads++
		return ""
	})
	command := newImportCommand(runtime)
	command.SetArgs([]string{writeManifest(t, importManifest)})
	if err := executeManifestCommand(command); err != nil {
		t.Fatal(err)
	}
	if reads != 0 {
		t.Fatalf("config import inspected %d Account environment values", reads)
	}
}

func TestImportReplacementFlagsMakeIdentityChangesExplicit(t *testing.T) {
	conflicting := strings.ReplaceAll(importManifest, "gateway", "local")
	conflicting = strings.ReplaceAll(conflicting, "remote", "local")

	runtime, path, out, _ := savedRuntime(t, localConfig())
	manifestPath := writeManifest(t, conflicting)
	withoutConsent := newImportCommand(runtime)
	withoutConsent.SetArgs([]string{manifestPath, "--dry-run", "--json"})
	if err := executeManifestCommand(withoutConsent); err == nil || !strings.Contains(err.Error(), "conflicts with local configuration") {
		t.Fatalf("error = %v", err)
	}

	withConsent := newImportCommand(runtime)
	withConsent.SetArgs([]string{manifestPath, "--replace-account", "local", "--replace-model", "gpt-local", "--replace-route", "local", "--json"})
	if err := executeManifestCommand(withConsent); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"dry_run": false`) || !strings.Contains(out.String(), `"next_action": "aigw status"`) {
		t.Fatalf("applied import omitted JSON result: %q", out.String())
	}
	loaded, err := configuration.NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Accounts["local"].Label != "Gateway" || loaded.Routes["local"].Label != "Remote" {
		t.Fatalf("explicit replacement did not converge: %#v", loaded)
	}
}

func TestImportRetiresExplicitUnselectedRoutes(t *testing.T) {
	cfg := localConfig()
	addRoute := func(id, label, model string) {
		cfg.Routes[id] = configuration.Route{
			Label: label, Account: "local", Model: model,
			Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
		}
	}
	addRoute("old", "Old", "gpt-old")
	addRoute("shared-old", "Shared Old", "gpt-shared")
	addRoute("shared-current", "Shared Current", "gpt-shared")
	addRoute("declared-old", "Declared Old", "gpt-declared")
	cfg.Models["gpt-old"] = configuration.Model{Label: "Old"}
	cfg.Models["gpt-shared"] = configuration.Model{Label: "Shared"}
	cfg.Models["gpt-declared"] = configuration.Model{Label: "Declared"}
	cfg.Recommendations[configuration.ClientCodex] = configuration.ClientRecommendation{
		Primary:      configuration.ClientSelection{Route: "old"},
		Alternatives: []configuration.ClientSelection{{Route: "shared-old"}},
	}
	runtime, _, _, renderOut := savedRuntime(t, cfg)
	command := newImportCommand(runtime)
	incoming := strings.Replace(importManifest, "[routes.remote]", "[models.gpt-declared]\nlabel = \"Declared\"\n\n[routes.remote]", 1)
	command.SetArgs([]string{writeManifest(t, incoming), "--retire-route", "old", "--retire-route", "shared-old", "--retire-route", "declared-old"})
	if err := executeManifestCommand(command); err != nil {
		t.Fatal(err)
	}
	got, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := got.Routes["old"]; exists {
		t.Fatal("explicitly retired Route remains")
	}
	if _, exists := got.Routes["shared-old"]; exists {
		t.Fatal("second explicitly retired Route remains")
	}
	if _, exists := got.Routes["declared-old"]; exists {
		t.Fatal("third explicitly retired Route remains")
	}
	if _, exists := got.Models["gpt-old"]; exists {
		t.Fatal("Model orphaned by retired Route remains")
	}
	if got.Models["gpt-shared"].Label != "Shared" || got.Models["gpt-declared"].Label != "Declared" || got.SelectedRoute(configuration.ClientCodex) != "local" || got.Routes["remote"].Account != "gateway" {
		t.Fatalf("unrelated state or imported Route changed: %#v", got)
	}
	if recommendation := got.Recommendations[configuration.ClientCodex]; recommendation.Primary.Route != "remote" || len(recommendation.Alternatives) != 0 {
		t.Fatalf("recommendation was not replaced: %#v", got.Recommendations)
	}
	if !strings.Contains(renderOut.String(), "Retired Routes") {
		t.Fatalf("import did not report Route retirement: %q", renderOut.String())
	}
}

func TestImportRejectsRouteRetirementBeforeWriting(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest string
		retire   string
		want     string
	}{
		{"missing", importManifest, "absent", "does not name an existing Route"},
		{"selected", importManifest, "local", "selected by client"},
		{"incoming", strings.ReplaceAll(importManifest, "remote", "local"), "local", "also declared in the imported configuration manifest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime, path, _, _ := savedRuntime(t, localConfig())
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			command := newImportCommand(runtime)
			command.SetArgs([]string{writeManifest(t, tc.manifest), "--retire-route", tc.retire})
			if err := executeManifestCommand(command); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("retirement error = %v, want %q", err, tc.want)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("rejected retirement changed configuration")
			}
		})
	}
}

func TestImportFallsBackToThePrimaryOutput(t *testing.T) {
	runtime, _, out, renderOut := savedRuntime(t, localConfig())
	runtime.RenderOut = nil
	command := newImportCommand(runtime)
	command.SetArgs([]string{writeManifest(t, importManifest)})
	if err := executeManifestCommand(command); err != nil {
		t.Fatal(err)
	}
	if output := out.String(); !strings.Contains(output, "Configuration manifest imported") || renderOut.Len() != 0 {
		t.Fatalf("output = %q", output)
	}
}

func TestImportSurfacesReadParseLoadAndMergeFailures(t *testing.T) {
	validPath := writeManifest(t, importManifest)
	tests := []struct {
		name    string
		runtime func(*testing.T) invocation.Context
		path    func(*testing.T) string
		want    string
	}{
		{
			name: "read",
			runtime: func(t *testing.T) invocation.Context {
				runtime, _, _, _ := savedRuntime(t, localConfig())
				return runtime
			},
			path: func(t *testing.T) string { return filepath.Join(t.TempDir(), "absent.toml") },
			want: "Failed to read configuration manifest",
		},
		{
			name: "parse",
			runtime: func(t *testing.T) invocation.Context {
				runtime, _, _, _ := savedRuntime(t, localConfig())
				return runtime
			},
			path: func(t *testing.T) string { return writeManifest(t, "not = [valid") },
			want: "parse configuration manifest",
		},
		{
			name: "load",
			runtime: func(t *testing.T) invocation.Context {
				path := filepath.Join(t.TempDir(), "configuration.toml")
				if err := os.WriteFile(path, []byte("not = [valid"), 0o600); err != nil {
					t.Fatal(err)
				}
				return invocation.Context{Config: configuration.NewStore(path), Secrets: secrets.NewMemoryStore(), Out: io.Discard}
			},
			path: func(*testing.T) string { return validPath },
			want: "parse config",
		},
		{
			name: "merge",
			runtime: func(t *testing.T) invocation.Context {
				runtime, _, _, _ := savedRuntime(t, localConfig())
				return runtime
			},
			path: func(t *testing.T) string {
				conflicting := strings.ReplaceAll(importManifest, "gateway", "local")
				return writeManifest(t, conflicting)
			},
			want: "conflicts with local configuration",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := newImportCommand(test.runtime(t))
			command.SetArgs([]string{test.path(t)})
			if err := executeManifestCommand(command); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestImportReturnsConfigurationTransactionFailure(t *testing.T) {
	runtime, path, _, _ := savedRuntime(t, localConfig())
	backup := path + ".bak"
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "blocker"), []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := newImportCommand(runtime)
	command.SetArgs([]string{writeManifest(t, importManifest)})
	if err := executeManifestCommand(command); err == nil {
		t.Fatal("configuration transaction failure was accepted")
	}
}

func savedRuntime(t *testing.T, cfg configuration.Config) (invocation.Context, string, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "configuration.toml")
	store := configuration.NewStore(path)
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	renderOut := &bytes.Buffer{}
	return invocation.Context{
		Config:    store,
		Secrets:   secrets.NewMemoryStore(),
		Out:       out,
		RenderOut: renderOut,
		Width:     120,
	}, path, out, renderOut
}

func localConfig() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["local"] = configuration.Account{Label: "Local", Endpoints: configuration.Endpoints{OpenAIResponses: "https://local.example/v1"}}
	cfg.Routes["local"] = configuration.Route{
		Label: "Local", Account: "local", Model: "gpt-local",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedRoute(configuration.ClientCodex, "local")
	return cfg
}

func writeManifest(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write refused") }
