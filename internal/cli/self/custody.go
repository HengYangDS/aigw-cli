package self

import (
	"maps"
	"slices"

	"aigw-cli/internal/app"
	"aigw-cli/internal/program"
	"aigw-cli/internal/secret"
)

// credentialCustody moves the configured providers' native items between the
// active program and a program that a rollback activates. When this program
// cannot load the configuration that names the credentials, the custody names
// none and the error says why; a missing configuration names none without an
// error.
func credentialCustody(deps app.Dependencies) (program.CredentialCustody, error) {
	custody := providerCustody{store: deps.Secrets, executable: deps.Executable}
	cfg, err := deps.Config.LoadCredentialConfig()
	if err == nil {
		custody.providers = slices.Sorted(maps.Keys(cfg.Providers))
	}
	return custody, err
}

type providerCustody struct {
	store      secret.Store
	executable string
	providers  []string
}

func (custody providerCustody) Release(version string) error {
	return secret.RestorePredecessorCredentials(custody.store, custody.executable, custody.providers, version)
}

func (custody providerCustody) Reclaim() error {
	return secret.ReclaimPredecessorCredentials(custody.store, custody.executable, custody.providers)
}
