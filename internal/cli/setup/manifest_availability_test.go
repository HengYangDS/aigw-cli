package setup

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/secret"

	surfaceidentity "aigw-cli/internal/adapter/surface"
	"aigw-cli/internal/store"
)

func TestMain(m *testing.M) {
	if len(os.Args) != 4 || !strings.HasPrefix(os.Args[1], "__aigw-native-credential-") {
		os.Exit(m.Run())
	}
	if os.Args[2] != secret.Service || os.Args[3] != "team" {
		os.Exit(3)
	}
	log, err := os.OpenFile(filepath.Join(filepath.Dir(os.Args[0]), "operations"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		os.Exit(3)
	}
	if _, err := log.WriteString(strings.Join(os.Args[1:], " ") + "\n"); err != nil {
		os.Exit(3)
	}
	if err := log.Close(); err != nil {
		os.Exit(3)
	}
	switch os.Args[1] {
	case "__aigw-native-credential-exists":
		_, _ = io.WriteString(os.Stdout, "1")
	case "__aigw-native-credential-read":
		_, _ = io.WriteString(os.Stdout, "synthetic-token")
	default:
		os.Exit(3)
	}
	os.Exit(0)
}

func TestManifestSetupObservesNativeAvailabilityOncePerInvocation(t *testing.T) {
	root := t.TempDir()
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	source = filepath.Join(root, filepath.Base(source))
	if err := os.WriteFile(source, data, 0o700); err != nil {
		t.Fatal(err)
	}
	secretStore, err := secret.Select(secret.Selection{
		Backend: "keyring", Executable: source,
		KeyringProbe: func(secret.Store) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := secret.VersionedEntrypointPath(filepath.Join(root, "data"), source, filepath.Base(source))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := store.Export(manifestSetupConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "team.toml"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "codex.toml")
	runtime := invocation.Context{Clients: clients.Registry(),
		Executable: source, CredentialPath: reader, Secrets: secretStore,
		Config: store.NewStore(filepath.Join(root, "configuration.toml"), clients.Admission()),
		Discovery: setupDiscovery{result: discovery.Result{
			Executables: map[string]string{codex.ID: source},
			Surfaces: []discovery.Surface{{
				ID: string(codex.HomeDefault), Authority: string(surfaceidentity.AuthorityAIGW),
				ConfigPath: target, Present: true, AutoManaged: true,
			}},
		}},
		Out: io.Discard, RenderOut: io.Discard,
	}
	cmd := NewCommand(runtime)
	cmd.SetContext(t.Context())
	cmd.SetArgs([]string{"--from", filepath.Join(root, "team.toml"), "--json"})
	for invocation := 1; invocation <= 2; invocation++ {
		if invocation > 1 {
			if err := os.Remove(filepath.Join(root, "configuration.toml")); err != nil {
				t.Fatal(err)
			}
		}
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		for _, operation := range []struct{ file, commands string }{
			{filepath.Join(root, "operations"), "__aigw-native-credential-exists " + secret.Service + " team\n__aigw-native-credential-read " + secret.Service + " team\n"},
			{filepath.Join(filepath.Dir(reader), "operations"), "__aigw-native-credential-read " + secret.Service + " team\n"},
		} {
			data, err := os.ReadFile(operation.file)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != strings.Repeat(operation.commands, invocation) {
				t.Fatalf("invocation %d: %s = %q; want one presence observation and live source/reader reads per invocation", invocation, operation.file, data)
			}
		}
	}
}

func TestManifestSetupAvailabilityFollowsTheAdmittedClientRegistry(t *testing.T) {
	executables := map[string]string{}
	for _, clientID := range clients.Admission().IDs() {
		executables[clientID] = "/installed/" + clientID
	}
	available := manifestSetupAvailableClients(clients.Admission(), executables)
	for _, clientID := range clients.Admission().IDs() {
		if !available[clientID] {
			t.Fatalf("admitted installed client %q is unavailable: %#v", clientID, available)
		}
	}
}
