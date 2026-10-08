package synchronization

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/activation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/secrets"
)

func newKeyringReaderFixture(t *testing.T, target string) Synchronizer {
	t.Helper()
	root := filepath.Dir(target)
	sourceName := "aigw"
	if runtime.GOOS == "windows" {
		sourceName += ".exe"
	}
	source := filepath.Join(root, sourceName)
	fixture := filepath.Join(root, "reader.go")
	program := `package main
import ("os"; "path/filepath"; "strings")
func main() {
    if len(os.Args) != 4 { os.Exit(3) }
    log, err := os.OpenFile(filepath.Join(filepath.Dir(os.Args[0]), "operations"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
    if err != nil { os.Exit(3) }
    if _, err := log.WriteString(strings.Join(os.Args[1:], " ")+"\n"); err != nil { os.Exit(3) }
    if err := log.Close(); err != nil { os.Exit(3) }
    if os.Args[1] == "__aigw-native-credential-exists" {
        if os.Args[3] == "missing" { _, _ = os.Stdout.WriteString("0") } else { _, _ = os.Stdout.WriteString("1") }
        return
    }
    if os.Args[1] != "__aigw-native-credential-read" { os.Exit(3) }
    if os.Args[3] == "missing" { os.Exit(2) }
    if strings.Contains(filepath.ToSlash(os.Args[0]), "/credential/") {
        if os.Args[3] == "copied-missing" { os.Exit(2) }
        if _, err := os.Stat(filepath.Join(filepath.Dir(os.Args[0]), "allow-reader")); err != nil { os.Exit(3) }
        if os.Args[3] == "blocked" { os.Exit(3) }
    }
    _, _ = os.Stdout.WriteString("synthetic-token")
}`
	if err := os.WriteFile(fixture, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	build := exec.CommandContext(t.Context(), "go", "build", "-o", source, fixture)
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build disposable credential reader: %v\n%s", err, output)
	}
	store, err := secrets.Select(secrets.Selection{
		Backend: "keyring", Executable: source,
		KeyringProbe: func(secrets.Store) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := credential.VersionedEntrypointPath(filepath.Join(root, "data"), source, sourceName)
	if err != nil {
		t.Fatal(err)
	}
	return Synchronizer{Config: &configStoreStub{}, Secrets: store, Discovery: targetDiscovery(target), AIGWExecutable: source, CredentialPath: reader}
}

func TestKeyringReaderCopyDenialPreventsProjection(t *testing.T) {
	for _, account := range []string{"gateway", "copied-missing"} {
		t.Run(account, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "codex.toml")
			syncer := newKeyringReaderFixture(t, target)
			reader := syncer.CredentialPath
			original := []byte("model_provider = \"native\"\n")
			if err := os.WriteFile(target, original, 0o600); err != nil {
				t.Fatal(err)
			}
			selected := testConfig(target)
			selected.Accounts[account] = selected.Accounts["gateway"]
			route := selected.Routes["gpt"]
			route.Account = account
			selected.Routes["gpt"] = route
			err := syncer.CommitProjection(t.Context(), configuration.NewConfig(), selected, "test")
			if err == nil {
				t.Fatal("copied reader denied by the native store was accepted")
			}
			if !strings.Contains(err.Error(), "copied credential reader") || !errors.Is(err, secrets.ErrNativeReaderUnverified) {
				t.Fatalf("failure lost its reader boundary or public diagnostic category: %v", err)
			}
			for _, path := range []string{reader, reader + ".sha256"} {
				if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("denied copied reader resource remains at %s: %v", path, statErr)
				}
			}
			if current, readErr := os.ReadFile(target); readErr != nil || !bytes.Equal(current, original) {
				t.Fatalf("preflight changed client projection: %q, %v", current, readErr)
			}
			configStore, ok := syncer.Config.(*configStoreStub)
			if !ok || configStore.commits != 0 {
				t.Fatal("denied credential reader reached configuration commit")
			}
			deferred := testConfig(target)
			deferred.Accounts["missing"] = deferred.Accounts["gateway"]
			route = deferred.Routes["gpt"]
			route.Account = "missing"
			deferred.Routes["gpt"] = route
			if err := syncer.CommitProjection(t.Context(), configuration.NewConfig(), deferred, "deferred", configuration.ClientCodex); err != nil {
				t.Fatalf("absent Token blocked deferred selection: %v", err)
			}
			if current, err := os.ReadFile(target); err != nil || !bytes.Equal(current, original) {
				t.Fatal("deferred selection changed the client projection")
			}
			if err := os.MkdirAll(filepath.Dir(reader), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(filepath.Dir(reader), "allow-reader"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			selected = testConfig(target)
			selected.Accounts["blocked"] = configuration.Account{
				Label: "Blocked", Endpoints: configuration.Endpoints{Anthropic: "https://blocked.test"},
			}
			selected.Routes["blocked-claude"] = configuration.Route{
				Account: "blocked", Model: "claude-test",
				Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}},
			}
			selected.SetSelectedRoute(configuration.ClientClaude, "blocked-claude", "")
			selected.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
			if err := syncer.CommitProjection(t.Context(), configuration.NewConfig(), selected, "codex", configuration.ClientCodex); err != nil {
				t.Fatalf("unrelated Claude Token blocked a Codex-only projection: %v", err)
			}
		})
	}
}

func TestKeyringReaderCopyVerifiesEachSelectedAccountOnce(t *testing.T) {
	target := filepath.Join(t.TempDir(), "codex.toml")
	syncer := newKeyringReaderFixture(t, target)
	reader := syncer.CredentialPath
	if err := os.MkdirAll(filepath.Dir(reader), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(reader), "allow-reader"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	requireSingleCopiedCredentialRead(t, syncer, testConfig(target), "gateway")
}

func TestSyncObservesNativeAvailabilityOncePerInvocation(t *testing.T) {
	target := filepath.Join(t.TempDir(), "codex.toml")
	syncer := newKeyringReaderFixture(t, target)
	if err := os.MkdirAll(filepath.Dir(syncer.CredentialPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(syncer.CredentialPath), "allow-reader"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	before := testConfig(target)
	configStore, ok := syncer.Config.(*configStoreStub)
	if !ok {
		t.Fatal("native reader fixture has no owned configuration store")
	}
	configStore.bindConfiguration(t, before)
	for invocation := 1; invocation <= 2; invocation++ {
		syncer.Secrets = secrets.ObserveAvailability(syncer.Secrets)
		after, _, err := syncer.DesiredSyncConfiguration(before)
		if err != nil {
			t.Fatal(err)
		}
		activation.AssessActivation(after, syncer.Secrets)
		if err := syncer.CommitProjection(t.Context(), before, after, "sync"); err != nil {
			t.Fatal(err)
		}
		for _, operation := range []struct{ executable, name string }{
			{syncer.AIGWExecutable, "__aigw-native-credential-exists"},
			{syncer.CredentialPath, "__aigw-native-credential-read"},
		} {
			data, err := os.ReadFile(filepath.Join(filepath.Dir(operation.executable), "operations"))
			if err != nil {
				t.Fatal(err)
			}
			want := operation.name + " " + secrets.Service + " gateway\n"
			if string(data) != strings.Repeat(want, invocation) {
				t.Fatalf("invocation %d: native operations = %q; want one exact operation per invocation: %q", invocation, data, want)
			}
		}
		before = after
	}
}

func requireSingleCopiedCredentialRead(t *testing.T, syncer Synchronizer, cfg configuration.Config, account string) {
	t.Helper()
	logs := []string{filepath.Join(filepath.Dir(syncer.AIGWExecutable), "operations"), filepath.Join(filepath.Dir(syncer.CredentialPath), "operations")}
	for _, path := range logs {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := syncer.prepareCredentialEntrypoint(cfg, configuration.ClientCodex); err != nil {
		t.Fatal(err)
	}
	var operations []string
	for _, path := range logs {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		operations = append(operations, strings.FieldsFunc(string(data), func(r rune) bool { return r == '\n' })...)
	}
	t.Logf("native credential operations: %v", operations)
	want := "__aigw-native-credential-read " + secrets.Service + " " + account
	if len(operations) != 1 || operations[0] != want {
		t.Fatalf("copied-reader authorization operations = %v, want one exact copied read: %s", operations, want)
	}
}
