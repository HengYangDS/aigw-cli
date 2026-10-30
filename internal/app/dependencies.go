// Package app holds AIGW's use cases: the operations behind each command. A
// use case receives the invocation's Dependencies and returns values for the
// terminal to render; it imports neither the command tree nor the terminal.
package app

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"time"

	"aigw-cli/internal/client"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/synchronization"
	"aigw-cli/internal/upgrade"
)

// HTTPDoer executes one HTTP request.
type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

// Updater performs one verified program lifecycle transition.
type Updater interface {
	Update(ctx context.Context, currentVersion string) (string, error)
	UpdateCandidate(ctx context.Context, currentVersion string, candidate upgrade.CandidateArchive) (string, error)
	Rollback(ctx context.Context, config []byte, custody upgrade.CredentialCustody) (string, error)
}

// Dependencies are the capabilities that one invocation's use cases share,
// without product-global state.
type Dependencies struct {
	Version               string
	Executable            string
	DataDir               string
	CredentialPath        string
	ResolveCredentialPath func() (string, error)
	ResolveProgram        func() (secret.Program, error)
	Files                 secret.Files
	InstallTarget         string
	// Host is the observed platform from which client adapters derive their
	// native paths.
	Host      discovery.Host
	Config    store.Store
	Secrets   secret.Store
	Accounts  secret.DiagnosticCredentialStore
	Runner    process.VerificationRunner
	HTTP      HTTPDoer
	Discovery discovery.Discoverer
	// Clients is the composition root's adapter registry; every client
	// operation of this invocation goes through it.
	Clients client.Registry
	Updater Updater
	Now     func() time.Time
}

// Discover reads client discovery through the invocation's discoverer.
func (d Dependencies) Discover() (discovery.Result, error) {
	if d.Discovery == nil {
		return discovery.Result{}, fmt.Errorf("client discovery is unavailable")
	}
	return d.Discovery.Discover(), nil
}

// Synchronizer assembles the synchronization domain from these capabilities.
func (d Dependencies) Synchronizer() synchronization.Synchronizer {
	return synchronization.Synchronizer{
		Config:                d.Config,
		Secrets:               d.Secrets,
		Runner:                d.Runner,
		Discovery:             d.Discovery,
		Registry:              d.Clients,
		Host:                  d.Host,
		AIGWExecutable:        d.Executable,
		CredentialPath:        d.CredentialPath,
		ResolveCredentialPath: d.ResolveCredentialPath,
		ResolveProgram:        d.ResolveProgram,
		Files:                 d.Files,
	}
}

// CredentialCustody moves the configured Accounts' native items between the
// active program and a program that a rollback activates. When this program
// cannot load the routing and credential authority, the custody names no
// credential and the error says why; a missing configuration names none
// without an error.
func (d Dependencies) CredentialCustody() (upgrade.CredentialCustody, error) {
	custody := credentialCustody{store: d.Secrets, executable: d.Executable}
	cfg, err := d.Config.LoadCredentialConfig()
	if err == nil {
		custody.accounts = slices.Sorted(maps.Keys(cfg.Accounts))
	}
	return custody, err
}

type credentialCustody struct {
	store      secret.Store
	executable string
	accounts   []string
}

func (custody credentialCustody) Release(version string) error {
	return secret.RestorePredecessorCredentials(custody.store, custody.executable, custody.accounts, version)
}

func (custody credentialCustody) Reclaim() error {
	return secret.ReclaimPredecessorCredentials(custody.store, custody.executable, custody.accounts)
}
