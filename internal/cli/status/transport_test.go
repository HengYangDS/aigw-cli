package status

import (
	"aigw-cli/internal/cli/invocation"

	"aigw-cli/internal/clients"

	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"bytes"

	"io"

	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/app"
)

func TestReadinessTransportHelpers(t *testing.T) {
	if got := app.EndpointTransport("%"); got != "" {
		t.Fatalf("invalid transport = %#v", got)
	}
	if got := app.EndpointTransport("http://LOCALHOST:48721/v1"); got != "external_loopback" {
		t.Fatalf("loopback transport = %#v", got)
	}
	if got := app.EndpointTransport("https://api.example.test/v1"); got != "" {
		t.Fatalf("remote transport = %#v", got)
	}
	buffer := &bytes.Buffer{}
	runtime := invocation.Context{Clients: clients.Registry(), Config: store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission()), Out: buffer, Secrets: secret.NewMemoryStore()}
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), "Not configured") {
		t.Fatalf("status output = %q", buffer.String())
	}
	command := NewCommand(invocation.Context{Clients: clients.Registry(), Config: store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission()), Out: io.Discard, Secrets: secret.NewMemoryStore()})
	command.SetArgs([]string{"--json"})
	if err := executeCommand(command); err != nil {
		t.Fatal(err)
	}
}
