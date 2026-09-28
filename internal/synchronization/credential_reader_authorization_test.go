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

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/secrets"
)

func TestKeyringReaderCopyDenialPreventsProjection(t *testing.T) {
	root := t.TempDir()
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
    if os.Args[1] == "__aigw-native-credential-exists" {
        if os.Args[3] == "missing" { _, _ = os.Stdout.WriteString("0") } else { _, _ = os.Stdout.WriteString("1") }
        return
    }
    if os.Args[1] != "__aigw-native-credential-read" { os.Exit(3) }
    if strings.Contains(filepath.ToSlash(os.Args[0]), "/credential/") {
        if _, err := os.Stat(filepath.Join(filepath.Dir(os.Args[0]), "allow-reader")); err != nil { os.Exit(3) }
        if os.Args[3] == "blocked" { os.Exit(3) }
    }
    _, _ = os.Stdout.WriteString("synthetic-token")
}`
	if err := os.WriteFile(fixture, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", source, fixture)
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
	target := filepath.Join(root, "codex.toml")
	original := []byte("model_provider = \"native\"\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	configStore := &configStoreStub{}
	syncer := Synchronizer{Config: configStore, Secrets: store, Discovery: targetDiscovery(target), AIGWExecutable: source, CredentialPath: reader}
	err = syncer.CommitProjection(t.Context(), configuration.NewConfig(), testConfig(target), "test")
	if err == nil {
		t.Fatal("copied reader denied by the native store was accepted")
	}
	if !strings.Contains(err.Error(), "copied credential reader") {
		t.Fatalf("failure did not identify the copied reader boundary: %v", err)
	}
	for _, path := range []string{reader, reader + ".sha256"} {
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("denied copied reader resource remains at %s: %v", path, statErr)
		}
	}
	if current, readErr := os.ReadFile(target); readErr != nil || !bytes.Equal(current, original) {
		t.Fatalf("preflight changed client projection: %q, %v", current, readErr)
	}
	if configStore.commits != 0 {
		t.Fatal("denied credential reader reached configuration commit")
	}
	deferred := testConfig(target)
	deferred.Accounts["missing"] = deferred.Accounts["gateway"]
	route := deferred.Routes["gpt"]
	route.Account = "missing"
	deferred.Routes["gpt"] = route
	undo, err := syncer.prepareCredentialEntrypoint(deferred)
	if err != nil {
		t.Fatalf("absent Token blocked deferred selection: %v", err)
	}
	if undo != nil {
		if err := undo(); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(reader), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(reader), "allow-reader"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	selected := testConfig(target)
	selected.Accounts["blocked"] = configuration.Account{
		Label: "Blocked", Endpoints: configuration.Endpoints{Anthropic: "https://blocked.test"},
	}
	selected.Routes["blocked-claude"] = configuration.Route{
		Account: "blocked", Model: "claude-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}},
	}
	selected.SetSelectedRoute(configuration.ClientClaude, "blocked-claude")
	selected.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
	if err := syncer.CommitProjection(t.Context(), configuration.NewConfig(), selected, "codex", configuration.ClientCodex); err != nil {
		t.Fatalf("unrelated Claude Token blocked a Codex-only projection: %v", err)
	}
}
