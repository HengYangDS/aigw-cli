package cli

import (
	"errors"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
)

// hostRefusingAdapter is an admitted client that derives no native path on any
// host. AIGW calls nothing else on it before it refuses to start.
type hostRefusingAdapter struct{ adapter.Adapter }

func (hostRefusingAdapter) Spec() domain.ClientSpec {
	return domain.ClientSpec{ID: "host-refusing", Label: "Host refusing", EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolResponses}}
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
