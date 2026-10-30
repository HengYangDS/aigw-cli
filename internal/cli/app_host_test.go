package cli

import (
	"errors"
	"runtime"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// hostRefusingAdapter is an admitted client that derives no native path on any
// host. AIGW calls nothing else on it before it refuses to start.
type hostRefusingAdapter struct{ adapter.Adapter }

func (hostRefusingAdapter) Spec() domain.ClientSpec {
	return domain.ClientSpec{ID: "host-refusing", Label: "Host refusing", EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolOpenAIResponses}}
}

func (hostRefusingAdapter) ValidateHost(discovery.Host) error {
	return errors.New("CLIENT_HOME is not set")
}

// AIGW does not start on a host where an admitted client cannot derive its
// native paths, and it reports that client's refusal unchanged.
func TestNewDefaultRefusesAHostThatAClientCannotServe(t *testing.T) {
	refusing := hostRefusingAdapter{}
	registry, err := adapter.NewRegistry(refusing)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewDefault(registry); err == nil || err.Error() != "CLIENT_HOME is not set" {
		t.Fatalf("NewDefault() error = %v, want the client's refusal", err)
	}
}

// On Windows, the built-in clients need variables that AIGW's own paths do not:
// Claude Code needs USERPROFILE and Claude Desktop needs LOCALAPPDATA.
func TestNewDefaultRefusesAWindowsHostWithoutTheBuiltInClientPaths(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("on macOS and Linux, AIGW's own paths need every variable that the built-in clients need")
	}
	for name, want := range map[string]string{"USERPROFILE": "USERPROFILE is not set", "LOCALAPPDATA": "LOCALAPPDATA is not set"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "")
			if _, err := NewDefault(clients.Registry()); err == nil || err.Error() != want {
				t.Fatalf("NewDefault() without %s = %v, want %q", name, err, want)
			}
		})
	}
}
