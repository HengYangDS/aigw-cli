package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

func TestProviderFinalizeRequiresCurrentFullVerificationCheckpoint(t *testing.T) {
	t.Run("missing checkpoint", func(t *testing.T) {
		app, _, secretStore, current := prepareProviderFinalizer(t, nil)
		if err := secretStore.Set("zeta-new", "target-token"); err != nil {
			t.Fatal(err)
		}
		assertFinalizeRefused(t, app, "verified checkpoint")
		assertCurrentProviderConfig(t, app, current)
	})

	t.Run("stale checkpoint", func(t *testing.T) {
		app, _, secretStore, _, _ := testApp(t, "")
		before := providerRenameConfig()
		before.SetClientActivation(claude.ID, true, "", nil)
		if err := app.Config.Save(before); err != nil {
			t.Fatal(err)
		}
		if err := app.Config.SaveVerifiedCheckpoint(t.Context(), before, before.EnabledClientIDs()); err != nil {
			t.Fatal(err)
		}
		staleCheckpoint, err := os.ReadFile(app.Config.Path() + ".verified.json")
		if err != nil {
			t.Fatal(err)
		}
		current := renamedProviderConfig(before)
		if err := app.Config.Save(current); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(app.Config.Path()+".verified.json", staleCheckpoint, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := secretStore.Set("zeta-new", "target-token"); err != nil {
			t.Fatal(err)
		}
		assertFinalizeRefused(t, app, "does not match current configuration")
		assertCurrentProviderConfig(t, app, current)
	})

	t.Run("partial client coverage", func(t *testing.T) {
		app, _, secretStore, current := prepareProviderFinalizer(t, nil)
		if err := app.Config.SaveVerifiedCheckpoint(t.Context(), current, []string{codex.ID}); err != nil {
			t.Fatal(err)
		}
		if err := secretStore.Set("zeta-new", "target-token"); err != nil {
			t.Fatal(err)
		}
		assertFinalizeRefused(t, app, "enabled client")
		assertCurrentProviderConfig(t, app, current)
	})

	t.Run("source account remains", func(t *testing.T) {
		app, _, secretStore, _, _ := testApp(t, "")
		current := providerRenameConfig()
		current.Providers["zeta-new"] = current.Providers["zeta-old"]
		current.SetClientActivation(claude.ID, true, "", nil)
		if err := app.Config.Save(current); err != nil {
			t.Fatal(err)
		}
		if err := app.Config.SaveVerifiedCheckpoint(t.Context(), current, current.EnabledClientIDs()); err != nil {
			t.Fatal(err)
		}
		if err := secretStore.Set("zeta-new", "target-token"); err != nil {
			t.Fatal(err)
		}
		assertFinalizeRefused(t, app, "source account")
	})
}

func TestProviderFinalizeUsesEnabledClientScope(t *testing.T) {
	for _, enabled := range []string{"", claude.ID, codex.ID} {
		t.Run("enabled="+enabled, func(t *testing.T) {
			app, _, store, _, _ := testApp(t, "")
			cfg := renamedProviderConfig(providerRenameConfig())
			if enabled != "" {
				cfg.SetClientActivation(enabled, true, "", nil)
			}
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if enabled != "" {
				if err := app.Config.SaveVerifiedCheckpoint(t.Context(), cfg, []string{enabled}); err != nil {
					t.Fatal(err)
				}
			}
			for _, id := range []string{"zeta-old", "zeta-new"} {
				if err := store.Set(id, "same-token"); err != nil {
					t.Fatal(err)
				}
			}
			if err := cli.Execute(app, []string{"account", "rename", "zeta-old", "zeta-new", "--finalize"}); err != nil {
				t.Fatal(err)
			}
			if secretExists(t, store, "zeta-old") || !secretExists(t, store, "zeta-new") {
				t.Fatal("retirement did not preserve only the target Token")
			}
			if !bytes.Equal(readFile(t, app.Config.Path()), readFile(t, app.Config.Path()+".bak")) {
				t.Fatal("retirement did not converge backup")
			}
		})
	}
}

func TestProviderFinalizeWithoutCredentialsNeedsNoToken(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	if err := app.Config.Save(renamedProviderConfig(providerRenameConfig())); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"account", "rename", "zeta-old", "zeta-new", "--finalize"}); err != nil {
		t.Fatal(err)
	}
}

func TestProviderFinalizeDryRunApplyAndAlreadyFinalized(t *testing.T) {
	app, out, secretStore, current := prepareProviderFinalizer(t, []string{claude.ID, codex.ID})
	const token = "finalize-shared-token"
	probe := secret.DiagnosticCredential{SystemToken: "finalize-shared-system", UserID: "finalize-shared-user"}
	for _, id := range []string{"zeta-old", "zeta-new"} {
		setFinalizerCredentials(t, app, id, token, probe)
	}
	configBefore := readFile(t, app.Config.Path())
	backupBefore := readFile(t, app.Config.Path()+".bak")
	checkpointBefore := readFile(t, app.Config.Path()+".verified.json")
	out.Reset()

	if err := cli.Execute(app, []string{"account", "rename", "zeta-old", "zeta-new", "--finalize", "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	result := decodeFinalizeResult(t, out.Bytes())
	want := finalizeResult{Status: "planned", Actions: map[string]string{
		"configuration": "already_renamed", "backup": "converge_to_verified_current",
		"api_token": "delete_source", "account_probe": "delete_source",
	}, ExternalTODOs: []string{}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("finalize dry-run = %#v", result)
	}
	if !bytes.Equal(readFile(t, app.Config.Path()), configBefore) || !bytes.Equal(readFile(t, app.Config.Path()+".bak"), backupBefore) || !bytes.Equal(readFile(t, app.Config.Path()+".verified.json"), checkpointBefore) {
		t.Fatal("finalize dry-run changed configuration state")
	}
	if !secretExists(t, secretStore, "zeta-old") || !providerCredentialExists(t, app.Providers, "zeta-old") {
		t.Fatal("finalize dry-run deleted source credentials")
	}

	out.Reset()
	if err := cli.Execute(app, []string{"account", "rename", "zeta-old", "zeta-new", "--finalize"}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readFile(t, app.Config.Path()), configBefore) || !bytes.Equal(readFile(t, app.Config.Path()+".bak"), configBefore) || !bytes.Equal(readFile(t, app.Config.Path()+".verified.json"), checkpointBefore) {
		t.Fatal("finalize did not converge only the backup to current bytes")
	}
	if secretExists(t, secretStore, "zeta-old") || providerCredentialExists(t, app.Providers, "zeta-old") {
		t.Fatal("finalize retained source credentials")
	}
	if got, err := secretStore.Get("zeta-new"); err != nil || got != token {
		t.Fatalf("target token = %q, %v", got, err)
	}
	if got, err := app.Providers.Get("zeta-new"); err != nil || !reflect.DeepEqual(got, probe) {
		t.Fatalf("target probe = %#v, %v", got, err)
	}
	assertCurrentProviderConfig(t, app, current)

	out.Reset()
	if err := cli.Execute(app, []string{"account", "rename", "zeta-old", "zeta-new", "--finalize", "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	result = decodeFinalizeResult(t, out.Bytes())
	if result.Status != "already_finalized" || result.Actions["backup"] != "already_converged" {
		t.Fatalf("idempotent finalize preview = %#v", result)
	}
	for _, forbidden := range []string{token, probe.SystemToken, probe.UserID, app.Config.Path()} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("finalize JSON leaked %q: %s", forbidden, out.String())
		}
	}
}

func TestProviderFinalizeRequiresTargetAPIToken(t *testing.T) {
	app, _, secretStore, _ := prepareProviderFinalizer(t, []string{claude.ID, codex.ID})
	if err := secretStore.Set("zeta-old", "source-token"); err != nil {
		t.Fatal(err)
	}
	backupBefore := readFile(t, app.Config.Path()+".bak")

	assertFinalizeRefused(t, app, "target API token")

	if !bytes.Equal(readFile(t, app.Config.Path()+".bak"), backupBefore) || !secretExists(t, secretStore, "zeta-old") {
		t.Fatal("missing target token changed backup or source token")
	}
}

func TestProviderFinalizeCredentialRotationRequiresConfirmationAndLiveProbe(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	before := providerRenameConfig()
	providerAccount := before.Providers["zeta-old"]
	providerAccount.BalanceProbe = &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://dmx.test"}
	before.Providers["zeta-old"] = providerAccount
	if err := app.Config.Save(before); err != nil {
		t.Fatal(err)
	}
	current := renamedProviderConfig(before)
	for _, client := range []string{claude.ID, codex.ID} {
		current.SetClientActivation(client, true, "", nil)
	}
	if err := app.Config.Save(current); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(t.Context(), current, current.EnabledClientIDs()); err != nil {
		t.Fatal(err)
	}
	const oldToken = "old-api-token"
	const newToken = "sk-abcd-middle-wxyz"
	oldProbe := secret.DiagnosticCredential{SystemToken: "old-system", UserID: "old-user"}
	newProbe := secret.DiagnosticCredential{SystemToken: "new-system", UserID: "10000"}
	setFinalizerCredentials(t, app, "zeta-old", oldToken, oldProbe)
	setFinalizerCredentials(t, app, "zeta-new", newToken, newProbe)
	requests := 0
	app.HTTP = &fakeHTTP{handler: func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Header.Get("Authorization") != "Bearer "+newProbe.SystemToken || req.Header.Get("Rix-Api-User") != newProbe.UserID {
			t.Fatalf("probe used wrong target credentials: %#v", req.Header)
		}
		body := `{"success":true,"data":{"quota":6250000}}`
		if strings.Contains(req.URL.Path, "/api/token/search") {
			body = `{"success":true,"data":{"items":[{"name":"Codex","key":"abcd**********wxyz","status":1,"used_quota":1000000,"remain_quota":2500000,"unlimited_quota":false,"unlimited_count":true,"expired_time":-1}]}}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	}}
	backupBefore := readFile(t, app.Config.Path()+".bak")
	out.Reset()

	if err := cli.Execute(app, []string{"account", "rename", "zeta-old", "zeta-new", "--finalize", "--dry-run", "--json"}); err == nil {
		t.Fatal("a blocked finalization plan succeeded")
	}
	result := decodeFinalizeResult(t, out.Bytes())
	if result.Status != "blocked" || !containsString(result.ExternalTODOs, "--confirm-api-token-rotation") || !containsString(result.ExternalTODOs, "--confirm-account-probe-rotation") {
		t.Fatalf("rotation preview = %#v", result)
	}
	if requests != 0 || !bytes.Equal(readFile(t, app.Config.Path()+".bak"), backupBefore) {
		t.Fatal("rotation dry-run performed a probe or changed backup")
	}

	out.Reset()
	err := cli.Execute(app, []string{"account", "rename", "zeta-old", "zeta-new", "--finalize"})
	if err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("unconfirmed rotation error = %v", err)
	}
	if requests != 0 || !secretExists(t, secretStore, "zeta-old") || !providerCredentialExists(t, app.Providers, "zeta-old") {
		t.Fatal("unconfirmed rotation changed credentials or performed a probe")
	}

	out.Reset()
	if err := cli.Execute(app, []string{"account", "rename", "zeta-old", "zeta-new", "--finalize", "--confirm-api-token-rotation", "--confirm-account-probe-rotation"}); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || secretExists(t, secretStore, "zeta-old") || providerCredentialExists(t, app.Providers, "zeta-old") {
		t.Fatalf("confirmed rotation requests=%d source_token=%v source_probe=%v", requests, secretExists(t, secretStore, "zeta-old"), providerCredentialExists(t, app.Providers, "zeta-old"))
	}
	for _, forbidden := range []string{oldToken, newToken, oldProbe.SystemToken, oldProbe.UserID, newProbe.SystemToken, newProbe.UserID} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("rotation finalizer leaked %q: %s", forbidden, out.String())
		}
	}
}

func TestProviderFinalizePartialDeleteCanRetryAfterBackupConvergence(t *testing.T) {
	app, _, secretStore, _ := prepareProviderFinalizer(t, []string{claude.ID, codex.ID})
	const token = "partial-delete-token"
	probe := secret.DiagnosticCredential{SystemToken: "partial-delete-system", UserID: "partial-delete-user"}
	for _, id := range []string{"zeta-old", "zeta-new"} {
		setFinalizerCredentials(t, app, id, token, probe)
	}
	baseAccounts := app.Providers
	app.Providers = failingDeleteProviderStore{DiagnosticCredentialStore: baseAccounts, providerID: "zeta-old", err: errors.New("probe delete refused")}

	err := cli.Execute(app, []string{"account", "rename", "zeta-old", "zeta-new", "--finalize"})
	if err == nil || !strings.Contains(err.Error(), "finalization incomplete") {
		t.Fatalf("partial delete error = %v", err)
	}
	if secretExists(t, secretStore, "zeta-old") || !providerCredentialExists(t, baseAccounts, "zeta-old") {
		t.Fatalf("partial delete state: source_token=%v source_probe=%v", secretExists(t, secretStore, "zeta-old"), providerCredentialExists(t, baseAccounts, "zeta-old"))
	}
	if !bytes.Equal(readFile(t, app.Config.Path()), readFile(t, app.Config.Path()+".bak")) {
		t.Fatal("partial deletion occurred before backup convergence")
	}

	app.Providers = baseAccounts
	if err := cli.Execute(app, []string{"account", "rename", "zeta-old", "zeta-new", "--finalize"}); err != nil {
		t.Fatal(err)
	}
	if providerCredentialExists(t, baseAccounts, "zeta-old") || !providerCredentialExists(t, baseAccounts, "zeta-new") {
		t.Fatal("retry did not finish probe credential cleanup")
	}
}

func TestProviderFinalizeEnvironmentCleanupIsExternalAndRetryable(t *testing.T) {
	app, _, _, _ := prepareProviderFinalizer(t, []string{claude.ID, codex.ID})
	const token = "finalize-environment-token"
	values := map[string]string{
		secret.EnvironmentKey("zeta-old"): token,
		secret.EnvironmentKey("zeta-new"): token,
	}
	app.Secrets = secret.NewEnvironmentStore(func(key string) string { return values[key] })
	probe := secret.DiagnosticCredential{SystemToken: "environment-probe-system", UserID: "environment-probe-user"}
	for _, id := range []string{"zeta-old", "zeta-new"} {
		if err := app.Providers.Set(id, probe); err != nil {
			t.Fatal(err)
		}
	}

	err := cli.Execute(app, []string{"account", "rename", "zeta-old", "zeta-new", "--finalize"})
	if err == nil || !strings.Contains(err.Error(), secret.EnvironmentKey("zeta-old")) || strings.Contains(err.Error(), token) {
		t.Fatalf("environment cleanup error = %v", err)
	}
	if providerCredentialExists(t, app.Providers, "zeta-old") {
		t.Fatal("environment cleanup did not finish writable probe cleanup")
	}
	if !bytes.Equal(readFile(t, app.Config.Path()), readFile(t, app.Config.Path()+".bak")) {
		t.Fatal("environment cleanup was reported before backup convergence")
	}

	delete(values, secret.EnvironmentKey("zeta-old"))
	if err := cli.Execute(app, []string{"account", "rename", "zeta-old", "zeta-new", "--finalize"}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Secrets.Get("zeta-old"); !errors.Is(err, secret.ErrNotFound) {
		t.Fatalf("source environment slot error = %v", err)
	}
	if got, err := app.Secrets.Get("zeta-new"); err != nil || got != token {
		t.Fatalf("target environment token = %q, %v", got, err)
	}
}

type finalizeResult struct {
	Status        string            `json:"status"`
	Actions       map[string]string `json:"actions"`
	ExternalTODOs []string          `json:"external_todos"`
}

func setFinalizerCredentials(t *testing.T, app *cli.App, account, token string, probe secret.DiagnosticCredential) {
	t.Helper()
	if err := app.Secrets.Set(account, token); err != nil {
		t.Fatal(err)
	}
	if err := app.Providers.Set(account, probe); err != nil {
		t.Fatal(err)
	}
}

func decodeFinalizeResult(t *testing.T, data []byte) finalizeResult {
	t.Helper()
	var result finalizeResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode finalize JSON: %v\n%s", err, data)
	}
	return result
}

func prepareProviderFinalizer(t *testing.T, clients []string) (*cli.App, *bytes.Buffer, secret.Store, domain.Config) {
	t.Helper()
	app, out, secretStore, _, _ := testApp(t, "")
	before := providerRenameConfig()
	if err := app.Config.Save(before); err != nil {
		t.Fatal(err)
	}
	current := renamedProviderConfig(before)
	enabledClients := clients
	if enabledClients == nil {
		enabledClients = []string{claude.ID, codex.ID}
	}
	for _, client := range enabledClients {
		current.SetClientActivation(client, true, "", nil)
	}
	if err := app.Config.Save(current); err != nil {
		t.Fatal(err)
	}
	if clients != nil {
		if err := app.Config.SaveVerifiedCheckpoint(t.Context(), current, clients); err != nil {
			t.Fatal(err)
		}
	}
	return app, out, secretStore, current
}

func renamedProviderConfig(cfg domain.Config) domain.Config {
	providerAccount := cfg.Providers["zeta-old"]
	delete(cfg.Providers, "zeta-old")
	cfg.Providers["zeta-new"] = providerAccount
	for routeID, route := range cfg.Models {
		if route.Provider == "zeta-old" {
			route.Provider = "zeta-new"
			cfg.Models[routeID] = route
		}
	}
	return cfg
}

func assertFinalizeRefused(t *testing.T, app *cli.App, message string) {
	t.Helper()
	err := cli.Execute(app, []string{"account", "rename", "zeta-old", "zeta-new", "--finalize", "--dry-run"})
	if err == nil || !strings.Contains(err.Error(), message) {
		t.Fatalf("finalize error = %v, want %q", err, message)
	}
}

func assertCurrentProviderConfig(t *testing.T, app *cli.App, want domain.Config) {
	t.Helper()
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("current config = %#v, want %#v", got, want)
	}
}

func containsString(values []string, substring string) bool {
	for _, value := range values {
		if strings.Contains(value, substring) {
			return true
		}
	}
	return false
}

type failingDeleteProviderStore struct {
	secret.DiagnosticCredentialStore
	providerID string
	err        error
}

func (s failingDeleteProviderStore) Delete(accountID string) error {
	if accountID == s.providerID {
		return s.err
	}
	return s.DiagnosticCredentialStore.Delete(accountID)
}
