package synchronization

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/processtest"
	"aigw-cli/internal/secret"
)

// unusableTokens are the fixture program's accounts whose slot is present and
// holds no Token that AIGW can use: an empty value, and an item that macOS
// refuses AIGW while its keychain is unlocked.
var unusableTokens = []string{"empty", "another-program"}

// waitingCodex selects for Codex a model of account, whose Token the fixture
// program holds in one of the unusable states.
func waitingCodex(t *testing.T, target, account string) domain.Config {
	cfg := testConfig(t, target)
	cfg.Providers[account] = cfg.Providers["gateway"]
	route := cfg.Models[domain.NewModelRef("gateway", "gpt-test")]
	route.Provider = account
	cfg.Models[route.Ref()] = route
	cfg.SetSelectedModel(codex.ID, route.Ref(), "")
	return cfg
}

// withClaudeOnGateway adds Claude on provider gateway, whose Token the fixture
// program returns.
func withClaudeOnGateway(t *testing.T, cfg domain.Config) domain.Config {
	cfg = cfg.Clone()
	gateway := cfg.Providers["gateway"]
	gateway.Endpoints.Anthropic = "https://gateway.test"
	cfg.Providers["gateway"] = gateway
	cfg.Models[domain.NewModelRef("gateway", "claude-test")] = domain.Model{
		Label: "Claude", Provider: "gateway", Family: "claude-test", ID: "claude-test",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("gateway", "claude-test"), "")
	cfg.SetClientActivation(claude.ID, true, processtest.Program(t, "/opt/client"), nil)
	return cfg
}

// nativeOperations returns what the fixture program at executable was asked,
// as "operation account", and nothing when it never ran.
func nativeOperations(t *testing.T, executable string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(filepath.Dir(executable), "operations"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var operations []string
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		operations = append(operations, strings.TrimPrefix(fields[0], "__aigw-native-credential-")+" "+fields[2])
	}
	return operations
}

// forgetNativeOperations empties the fixture program's record at executable.
func forgetNativeOperations(t *testing.T, executable string) {
	t.Helper()
	if err := os.Remove(filepath.Join(filepath.Dir(executable), "operations")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

// allowReader lets the fixture's copied reader read the Tokens that the
// fixture program returns.
func allowReader(t *testing.T, reader string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(reader), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(reader), "allow-reader"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

// A client whose provider's Token slot is present and holds no Token that
// AIGW can use waits, as it does for a missing Token: the configuration is
// committed, its file stays as it was, and no reader is installed or run for
// it. A client on another provider is projected by the same commit.
func TestAClientWaitsForATokenThatAIGWCannotUse(t *testing.T) {
	for _, account := range unusableTokens {
		t.Run(account, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "codex.toml")
			syncer, reader := newKeyringReaderFixture(t, target)
			syncer.Host = hostAt(root)
			original := []byte("model_provider = \"native\"\n")
			if err := os.WriteFile(target, original, 0o600); err != nil {
				t.Fatal(err)
			}
			configStore, ok := syncer.Config.(*configStoreStub)
			if !ok {
				t.Fatal("the fixture has no owned configuration store")
			}
			empty, waiting := domain.NewConfig(clients.Admission()), waitingCodex(t, target, account)

			if err := syncer.CommitProjection(t.Context(), empty, waiting, "sync"); err != nil {
				t.Fatalf("a commit for the client that waits = %v", err)
			}
			if configStore.commits != 1 {
				t.Fatalf("the configuration was committed %d times", configStore.commits)
			}
			for _, path := range []string{reader, reader + ".sha256"} {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("a reader was installed for a client that waits, at %s: %v", path, err)
				}
			}
			if got, want := nativeOperations(t, syncer.AIGWExecutable), []string{"exists " + account, "empty " + account}; !slices.Equal(got, want) {
				t.Fatalf("AIGW's program ran %q; want %q", got, want)
			}
			if got := nativeOperations(t, reader); got != nil {
				t.Fatalf("the reader ran %q", got)
			}

			allowReader(t, reader)
			if err := syncer.CommitProjection(t.Context(), empty, withClaudeOnGateway(t, waiting), "sync"); err != nil {
				t.Fatalf("a commit for a client that waits and one that does not = %v", err)
			}
			if configStore.commits != 2 {
				t.Fatalf("the configuration was committed %d times", configStore.commits)
			}
			settings, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
			if err != nil || !bytes.Contains(settings, []byte("https://gateway.test")) {
				t.Fatalf("the client whose Token is usable was not projected: %q, %v", settings, err)
			}
			if got, want := nativeOperations(t, reader), []string{"read gateway"}; !slices.Equal(got, want) {
				t.Fatalf("the reader ran %q; want %q", got, want)
			}
			if current, err := os.ReadFile(target); err != nil || !bytes.Equal(current, original) {
				t.Fatalf("the file of the client that waits changed: %q, %v", current, err)
			}
		})
	}
}

// A preview leaves out the client that its commit leaves waiting, and reads no
// Token to decide. A provider whose Token the previewed command would store
// counts as usable, and the store is not asked about it.
func TestAPreviewLeavesOutTheClientThatWaitsForItsToken(t *testing.T) {
	for _, account := range unusableTokens {
		t.Run(account, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "codex.toml")
			syncer, reader := newKeyringReaderFixture(t, target)
			syncer.Host = hostAt(root)
			empty, both := domain.NewConfig(clients.Admission()), withClaudeOnGateway(t, waitingCodex(t, target, account))

			projection, err := syncer.PreviewProjection(empty, both)
			if err != nil || !slices.Contains(projection.Clients, claude.ID) || slices.Contains(projection.Clients, codex.ID) {
				t.Fatalf("the preview reconciles %q, %v; want Claude and not Codex", projection.Clients, err)
			}
			asked := nativeOperations(t, syncer.AIGWExecutable)
			slices.Sort(asked)
			if want := []string{"empty " + account, "empty gateway", "exists " + account, "exists gateway"}; !slices.Equal(asked, want) {
				t.Fatalf("the preview ran %q; want %q", asked, want)
			}

			forgetNativeOperations(t, syncer.AIGWExecutable)
			_, projection, err = syncer.PreviewSetup(empty, both, []string{account}, codex.ID)
			if err != nil || !slices.Contains(projection.Clients, claude.ID) || !slices.Contains(projection.Clients, codex.ID) {
				t.Fatalf("the preview of a setup that stores the Token reconciles %q, %v; want Claude and Codex", projection.Clients, err)
			}
			asked = nativeOperations(t, syncer.AIGWExecutable)
			slices.Sort(asked)
			if want := []string{"empty gateway", "exists gateway"}; !slices.Equal(asked, want) {
				t.Fatalf("the preview that presumes the Token ran %q; want %q", asked, want)
			}
			if got := nativeOperations(t, reader); got != nil {
				t.Fatalf("a preview ran the reader: %q", got)
			}
			if _, err := os.Stat(reader); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("a preview installed a reader: %v", err)
			}
		})
	}
}

// The rule holds on every credential backend. An empty Token file is a slot
// that is there and holds nothing, so its client waits. The file backend has
// no reader to check: before, it wrote that client's file to name a reader
// that returns no Token.
func TestAClientWaitsForAnEmptyTokenFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "codex.toml")
	original := []byte("model_provider = \"native\"\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	secrets := filepath.Join(root, "secrets")
	files, err := secret.Select(secret.Selection{Backend: "file", GOOS: runtime.GOOS, Root: secrets})
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{"gateway", "empty"} {
		if err := files.Set(account, "synthetic-token"); err != nil {
			t.Fatal(err)
		}
	}
	// The selected file backend keeps its Token files in the folder tokens.
	if err := os.Truncate(filepath.Join(secrets, "tokens", "empty"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := files.Get("empty"); !errors.Is(err, secret.ErrEmpty) {
		t.Fatalf("the fixture's empty Token file = %v", err)
	}
	// Any bytes serve as the program: this backend runs no reader to check it.
	program := filepath.Join(root, "aigw")
	if err := os.WriteFile(program, []byte("program"), 0o700); err != nil {
		t.Fatal(err)
	}
	reader, err := versionedReaderPath(filepath.Join(root, "data"), program, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	configStore := &configStoreStub{}
	syncer := Synchronizer{Registry: builtinClients, Config: configStore, Secrets: files, Discovery: targetDiscovery(target), Host: hostAt(root),
		AIGWExecutable: program, ResolveCredentialPath: readerAt(reader)}
	empty, both := domain.NewConfig(clients.Admission()), withClaudeOnGateway(t, waitingCodex(t, target, "empty"))

	projection, err := syncer.PreviewProjection(empty, both)
	if err != nil || !slices.Contains(projection.Clients, claude.ID) || slices.Contains(projection.Clients, codex.ID) {
		t.Fatalf("the preview reconciles %q, %v; want Claude and not Codex", projection.Clients, err)
	}
	if err := syncer.CommitProjection(t.Context(), empty, both, "sync"); err != nil || configStore.commits != 1 {
		t.Fatalf("a commit for a client that waits and one that does not = %v, after %d commits", err, configStore.commits)
	}
	settings, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil || !bytes.Contains(settings, []byte("https://gateway.test")) {
		t.Fatalf("the client whose Token is stored was not projected: %q, %v", settings, err)
	}
	if current, err := os.ReadFile(target); err != nil || !bytes.Equal(current, original) {
		t.Fatalf("the file of the client that waits changed: %q, %v", current, err)
	}
}

// The fixture's unusable slots are faithful to the system keyring: each is
// present, and a read returns no Token.
func TestTheUnusableTokenFixturesArePresentAndUnreadable(t *testing.T) {
	syncer, _ := newKeyringReaderFixture(t, filepath.Join(t.TempDir(), "codex.toml"))
	for account, want := range map[string]error{"empty": secret.ErrEmpty, "another-program": nil} {
		if present, err := syncer.Secrets.Exists(account); err != nil || !present {
			t.Fatalf("Exists(%s) = %v, %v", account, present, err)
		}
		_, err := syncer.Secrets.Get(account)
		_, refused := errors.AsType[*secret.KeychainItemRefusedError](err)
		if (want != nil && !errors.Is(err, want)) || (want == nil && !refused) {
			t.Fatalf("Get(%s) = %v", account, err)
		}
	}
}
