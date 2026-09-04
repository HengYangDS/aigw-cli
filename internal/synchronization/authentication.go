package synchronization

import (
	"context"

	configuration "aigw-cli/internal/configuration"
)

// BindCredential updates one admitted client's native credential projection.
func (s Synchronizer) BindCredential(ctx context.Context, cfg configuration.Config, clientID string, targets []string) error {
	return s.registry().BindCredential(ctx, s.clientDependencies(), cfg, clientID, targets)
}

// BindChangedCredentials updates every native credential binding affected by
// a configuration transition.
func (s Synchronizer) BindChangedCredentials(ctx context.Context, before, after configuration.Config) error {
	return s.registry().BindChangedCredentials(ctx, s.clientDependencies(), before, after)
}

// BindCredentialsForAccount refreshes every native credential projection that
// currently depends on accountID.
func (s Synchronizer) BindCredentialsForAccount(ctx context.Context, cfg configuration.Config, accountID string) error {
	return s.registry().BindCredentialsForAccount(ctx, s.clientDependencies(), cfg, accountID)
}

// CredentialBindingChanged reports whether any admitted adapter must refresh
// native authentication for a configuration transition.
func (s Synchronizer) CredentialBindingChanged(before, after configuration.Config) bool {
	return s.registry().CredentialBindingChanged(before, after)
}

// UsesCredentialAccount reports whether accountID backs a native client
// credential projection.
func (s Synchronizer) UsesCredentialAccount(cfg configuration.Config, accountID string) bool {
	return s.registry().UsesCredentialAccount(cfg, accountID)
}
