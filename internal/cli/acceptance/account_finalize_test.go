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

	"aigw-cli/internal/account"
	"aigw-cli/internal/cli"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestAccountFinalizeRequiresCurrentFullVerificationCheckpoint(t *testing.T) {
	t.Run("missing checkpoint", func(t *testing.T) {
		app, _, secretStore, current := prepareAccountFinalizer(t, nil)
		if err := secretStore.Set("zeta-new", "target-token"); err != nil {
			t.Fatal(err)
		}
		assertFinalizeRefused(t, app, "verified checkpoint")
		assertCurrentAccountConfig(t, app, current)
	})

	t.Run("stale checkpoint", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		before := accountRenameConfig()
		if err := app.Config.Save(before); err != nil {
			t.Fatal(err)
		}
		if err := app.Config.SaveVerifiedCheckpoint(before, configuration.AdmittedClientIDs()); err != nil {
			t.Fatal(err)
		}
		staleCheckpoint, err := os.ReadFile(app.Config.Path() + ".verified.json")
		if err != nil {
			t.Fatal(err)
		}
		current := renamedAccountConfig(before)
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
		assertCurrentAccountConfig(t, app, current)
	})

	t.Run("partial client coverage", func(t *testing.T) {
		app, _, secretStore, current := prepareAccountFinalizer(t, []string{configuration.ClientCodex})
		if err := secretStore.Set("zeta-new", "target-token"); err != nil {
			t.Fatal(err)
		}
		assertFinalizeRefused(t, app, "does not cover all admitted clients")
		assertCurrentAccountConfig(t, app, current)
	})

	t.Run("source account remains", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		current := accountRenameConfig()
		current.Accounts["zeta-new"] = current.Accounts["zeta-old"]
		if err := app.Config.Save(current); err != nil {
			t.Fatal(err)
		}
		if err := app.Config.SaveVerifiedCheckpoint(current, configuration.AdmittedClientIDs()); err != nil {
			t.Fatal(err)
		}
		if err := secretStore.Set("zeta-new", "target-token"); err != nil {
			t.Fatal(err)
		}
		assertFinalizeRefused(t, app, "source account")
	})
}

func TestAccountFinalizeDryRunApplyAndAlreadyFinalized(t *testing.T) {
	app, out, secretStore, current := prepareAccountFinalizer(t, configuration.AdmittedClientIDs())
	const token = "finalize-shared-token"
	probe := account.Credential{SystemToken: "finalize-shared-system", UserID: "finalize-shared-user"}
	for _, id := range []string{"zeta-old", "zeta-new"} {
		if err := secretStore.Set(id, token); err != nil {
			t.Fatal(err)
		}
		if err := app.Accounts.Set(id, probe); err != nil {
			t.Fatal(err)
		}
	}
	configBefore := readFile(t, app.Config.Path())
	backupBefore := readFile(t, app.Config.Path()+".bak")
	checkpointBefore := readFile(t, app.Config.Path()+".verified.json")
	out.Reset()

	if err := execute(t, app, "account", "rename", "zeta-old", "zeta-new", "--finalize", "--dry-run", "--json"); err != nil {
		t.Fatal(err)
	}
	result := decodeFinalizeResult(t, out.Bytes())
	if result.Status != "planned" || result.Actions["backup"] != "converge-to-verified-current" || result.Actions["api_token"] != "delete-source" || result.Actions["account_probe"] != "delete-source" {
		t.Fatalf("finalize dry-run = %#v", result)
	}
	if !bytes.Equal(readFile(t, app.Config.Path()), configBefore) || !bytes.Equal(readFile(t, app.Config.Path()+".bak"), backupBefore) || !bytes.Equal(readFile(t, app.Config.Path()+".verified.json"), checkpointBefore) {
		t.Fatal("finalize dry-run changed configuration state")
	}
	if !secretExists(t, secretStore, "zeta-old") || !accountCredentialExists(t, app.Accounts, "zeta-old") {
		t.Fatal("finalize dry-run deleted source credentials")
	}

	out.Reset()
	if err := execute(t, app, "account", "rename", "zeta-old", "zeta-new", "--finalize"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readFile(t, app.Config.Path()), configBefore) || !bytes.Equal(readFile(t, app.Config.Path()+".bak"), configBefore) || !bytes.Equal(readFile(t, app.Config.Path()+".verified.json"), checkpointBefore) {
		t.Fatal("finalize did not converge only the backup to current bytes")
	}
	if secretExists(t, secretStore, "zeta-old") || accountCredentialExists(t, app.Accounts, "zeta-old") {
		t.Fatal("finalize retained source credentials")
	}
	if got, err := secretStore.Get("zeta-new"); err != nil || got != token {
		t.Fatalf("target token = %q, %v", got, err)
	}
	if got, err := app.Accounts.Get("zeta-new"); err != nil || !reflect.DeepEqual(got, probe) {
		t.Fatalf("target probe = %#v, %v", got, err)
	}
	assertCurrentAccountConfig(t, app, current)

	out.Reset()
	if err := execute(t, app, "account", "rename", "zeta-old", "zeta-new", "--finalize", "--dry-run", "--json"); err != nil {
		t.Fatal(err)
	}
	result = decodeFinalizeResult(t, out.Bytes())
	if result.Status != "already-finalized" || result.Actions["backup"] != "already-converged" {
		t.Fatalf("idempotent finalize preview = %#v", result)
	}
	for _, forbidden := range []string{token, probe.SystemToken, probe.UserID, app.Config.Path()} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("finalize JSON leaked %q: %s", forbidden, out.String())
		}
	}
}

func TestAccountFinalizeRequiresTargetAPIToken(t *testing.T) {
	app, _, secretStore, _ := prepareAccountFinalizer(t, configuration.AdmittedClientIDs())
	if err := secretStore.Set("zeta-old", "source-token"); err != nil {
		t.Fatal(err)
	}
	backupBefore := readFile(t, app.Config.Path()+".bak")

	assertFinalizeRefused(t, app, "target API token")

	if !bytes.Equal(readFile(t, app.Config.Path()+".bak"), backupBefore) || !secretExists(t, secretStore, "zeta-old") {
		t.Fatal("missing target token changed backup or source token")
	}
}

func TestAccountFinalizeCredentialRotationRequiresConfirmationAndLiveProbe(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	before := accountRenameConfig()
	providerAccount := before.Accounts["zeta-old"]
	providerAccount.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://dmx.test"}
	before.Accounts["zeta-old"] = providerAccount
	if err := app.Config.Save(before); err != nil {
		t.Fatal(err)
	}
	current := renamedAccountConfig(before)
	if err := app.Config.Save(current); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(current, configuration.AdmittedClientIDs()); err != nil {
		t.Fatal(err)
	}
	const oldToken = "old-api-token"
	const newToken = "sk-abcd-middle-wxyz"
	oldProbe := account.Credential{SystemToken: "old-system", UserID: "old-user"}
	newProbe := account.Credential{SystemToken: "new-system", UserID: "10000"}
	if err := secretStore.Set("zeta-old", oldToken); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("zeta-new", newToken); err != nil {
		t.Fatal(err)
	}
	if err := app.Accounts.Set("zeta-old", oldProbe); err != nil {
		t.Fatal(err)
	}
	if err := app.Accounts.Set("zeta-new", newProbe); err != nil {
		t.Fatal(err)
	}
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

	if err := execute(t, app, "account", "rename", "zeta-old", "zeta-new", "--finalize", "--dry-run", "--json"); err != nil {
		t.Fatal(err)
	}
	result := decodeFinalizeResult(t, out.Bytes())
	if result.Status != "blocked" || !containsString(result.ExternalTODOs, "--confirm-api-token-rotation") || !containsString(result.ExternalTODOs, "--confirm-account-probe-rotation") {
		t.Fatalf("rotation preview = %#v", result)
	}
	if requests != 0 || !bytes.Equal(readFile(t, app.Config.Path()+".bak"), backupBefore) {
		t.Fatal("rotation dry-run performed a probe or changed backup")
	}

	out.Reset()
	err := execute(t, app, "account", "rename", "zeta-old", "zeta-new", "--finalize")
	if err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("unconfirmed rotation error = %v", err)
	}
	if requests != 0 || !secretExists(t, secretStore, "zeta-old") || !accountCredentialExists(t, app.Accounts, "zeta-old") {
		t.Fatal("unconfirmed rotation changed credentials or performed a probe")
	}

	out.Reset()
	if err := execute(t, app, "account", "rename", "zeta-old", "zeta-new", "--finalize", "--confirm-api-token-rotation", "--confirm-account-probe-rotation"); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || secretExists(t, secretStore, "zeta-old") || accountCredentialExists(t, app.Accounts, "zeta-old") {
		t.Fatalf("confirmed rotation requests=%d source_token=%v source_probe=%v", requests, secretExists(t, secretStore, "zeta-old"), accountCredentialExists(t, app.Accounts, "zeta-old"))
	}
	for _, forbidden := range []string{oldToken, newToken, oldProbe.SystemToken, oldProbe.UserID, newProbe.SystemToken, newProbe.UserID} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("rotation finalizer leaked %q: %s", forbidden, out.String())
		}
	}
}

func TestAccountFinalizePartialDeleteCanRetryAfterBackupConvergence(t *testing.T) {
	app, _, secretStore, _ := prepareAccountFinalizer(t, configuration.AdmittedClientIDs())
	const token = "partial-delete-token"
	probe := account.Credential{SystemToken: "partial-delete-system", UserID: "partial-delete-user"}
	for _, id := range []string{"zeta-old", "zeta-new"} {
		if err := secretStore.Set(id, token); err != nil {
			t.Fatal(err)
		}
		if err := app.Accounts.Set(id, probe); err != nil {
			t.Fatal(err)
		}
	}
	baseAccounts := app.Accounts
	app.Accounts = failingDeleteAccountStore{Store: baseAccounts, accountID: "zeta-old", err: errors.New("probe delete refused")}

	err := execute(t, app, "account", "rename", "zeta-old", "zeta-new", "--finalize")
	if err == nil || !strings.Contains(err.Error(), "finalization incomplete") {
		t.Fatalf("partial delete error = %v", err)
	}
	if secretExists(t, secretStore, "zeta-old") || !accountCredentialExists(t, baseAccounts, "zeta-old") {
		t.Fatalf("partial delete state: source_token=%v source_probe=%v", secretExists(t, secretStore, "zeta-old"), accountCredentialExists(t, baseAccounts, "zeta-old"))
	}
	if !bytes.Equal(readFile(t, app.Config.Path()), readFile(t, app.Config.Path()+".bak")) {
		t.Fatal("partial deletion occurred before backup convergence")
	}

	app.Accounts = baseAccounts
	if err := execute(t, app, "account", "rename", "zeta-old", "zeta-new", "--finalize"); err != nil {
		t.Fatal(err)
	}
	if accountCredentialExists(t, baseAccounts, "zeta-old") || !accountCredentialExists(t, baseAccounts, "zeta-new") {
		t.Fatal("retry did not finish probe credential cleanup")
	}
}

func TestAccountFinalizeEnvironmentCleanupIsExternalAndRetryable(t *testing.T) {
	app, _, _, _ := prepareAccountFinalizer(t, configuration.AdmittedClientIDs())
	const token = "finalize-environment-token"
	values := map[string]string{
		secrets.EnvironmentKey("zeta-old"): token,
		secrets.EnvironmentKey("zeta-new"): token,
	}
	app.Secrets = secrets.NewEnvironmentStore(func(key string) string { return values[key] })
	probe := account.Credential{SystemToken: "environment-probe-system", UserID: "environment-probe-user"}
	for _, id := range []string{"zeta-old", "zeta-new"} {
		if err := app.Accounts.Set(id, probe); err != nil {
			t.Fatal(err)
		}
	}

	err := execute(t, app, "account", "rename", "zeta-old", "zeta-new", "--finalize")
	if err == nil || !strings.Contains(err.Error(), secrets.EnvironmentKey("zeta-old")) || strings.Contains(err.Error(), token) {
		t.Fatalf("environment cleanup error = %v", err)
	}
	if accountCredentialExists(t, app.Accounts, "zeta-old") {
		t.Fatal("environment cleanup did not finish writable probe cleanup")
	}
	if !bytes.Equal(readFile(t, app.Config.Path()), readFile(t, app.Config.Path()+".bak")) {
		t.Fatal("environment cleanup was reported before backup convergence")
	}

	delete(values, secrets.EnvironmentKey("zeta-old"))
	if err := execute(t, app, "account", "rename", "zeta-old", "zeta-new", "--finalize"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Secrets.Get("zeta-old"); !errors.Is(err, secrets.ErrNotFound) {
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

func decodeFinalizeResult(t *testing.T, data []byte) finalizeResult {
	t.Helper()
	var result finalizeResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode finalize JSON: %v\n%s", err, data)
	}
	return result
}

func prepareAccountFinalizer(t *testing.T, clients []string) (*cli.App, *bytes.Buffer, *secrets.MemoryStore, configuration.Config) {
	t.Helper()
	app, out, secretStore, _ := testApp(t, "")
	before := accountRenameConfig()
	if err := app.Config.Save(before); err != nil {
		t.Fatal(err)
	}
	current := renamedAccountConfig(before)
	if err := app.Config.Save(current); err != nil {
		t.Fatal(err)
	}
	if clients != nil {
		if err := app.Config.SaveVerifiedCheckpoint(current, clients); err != nil {
			t.Fatal(err)
		}
	}
	return app, out, secretStore, current
}

func renamedAccountConfig(cfg configuration.Config) configuration.Config {
	providerAccount := cfg.Accounts["zeta-old"]
	delete(cfg.Accounts, "zeta-old")
	cfg.Accounts["zeta-new"] = providerAccount
	for profileID, profile := range cfg.Profiles {
		if profile.Account == "zeta-old" {
			profile.Account = "zeta-new"
			cfg.Profiles[profileID] = profile
		}
	}
	return cfg
}

func assertFinalizeRefused(t *testing.T, app *cli.App, message string) {
	t.Helper()
	err := execute(t, app, "account", "rename", "zeta-old", "zeta-new", "--finalize", "--dry-run")
	if err == nil || !strings.Contains(err.Error(), message) {
		t.Fatalf("finalize error = %v, want %q", err, message)
	}
}

func assertCurrentAccountConfig(t *testing.T, app *cli.App, want configuration.Config) {
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

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type failingDeleteAccountStore struct {
	account.Store
	accountID string
	err       error
}

func (s failingDeleteAccountStore) Delete(accountID string) error {
	if accountID == s.accountID {
		return s.err
	}
	return s.Store.Delete(accountID)
}
