package update

import (
	"maps"
	"slices"

	"aigw-cli/internal/app"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/upgrade"
)

// credentialCustody moves the configured Accounts' native items between the
// active program and a program that a rollback activates. When this program
// cannot load the routing and credential authority, the custody names no
// credential and the error says why; a missing configuration names none
// without an error.
func credentialCustody(deps app.Dependencies) (upgrade.CredentialCustody, error) {
	custody := accountCustody{store: deps.Secrets, executable: deps.Executable}
	cfg, err := deps.Config.LoadCredentialConfig()
	if err == nil {
		custody.accounts = slices.Sorted(maps.Keys(cfg.Accounts))
	}
	return custody, err
}

type accountCustody struct {
	store      secret.Store
	executable string
	accounts   []string
}

func (custody accountCustody) Release(version string) error {
	return secret.RestorePredecessorCredentials(custody.store, custody.executable, custody.accounts, version)
}

func (custody accountCustody) Reclaim() error {
	return secret.ReclaimPredecessorCredentials(custody.store, custody.executable, custody.accounts)
}
