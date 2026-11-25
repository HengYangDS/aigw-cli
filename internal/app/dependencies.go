// Package app holds the use cases that several AIGW commands, or the CLI root
// on a command's behalf, share. A use case receives the capabilities that it
// needs, usually the invocation's Dependencies, and returns values for the
// terminal to render; it imports neither the command tree nor the terminal.
package app

import (
	"context"
	"fmt"
	"net/http"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/app/synchronization"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/program"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

// HTTPDoer executes one HTTP request.
type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

// Updater performs one verified program lifecycle transition.
type Updater interface {
	Update(ctx context.Context, currentVersion string) (program.Transition, error)
	UpdateCandidate(ctx context.Context, currentVersion string, candidate program.CandidateArchive) (program.Transition, error)
	// Preview and PreviewCandidate report what Update and UpdateCandidate
	// would do, from public release metadata or the archive's name alone.
	Preview(ctx context.Context, currentVersion string) (program.Transition, error)
	PreviewCandidate(currentVersion string, candidate program.CandidateArchive) (program.Transition, error)
	Rollback(ctx context.Context, config []byte) (program.Transition, error)
}

// Dependencies are the capabilities that one invocation's use cases share,
// without product-global state.
type Dependencies struct {
	Version               string
	Executable            string
	ResolveCredentialPath func() (string, error)
	ResolveProgram        func() (secret.Program, error)
	Files                 secret.Files
	InstallTarget         string
	// Host is the observed platform from which client adapters derive their
	// native paths.
	Host discovery.Host
	// Env is the invocation's process environment, as os.Environ lists it.
	Env       []string
	Config    store.Store
	Secrets   secret.Store
	Providers secret.DiagnosticCredentialStore
	Runner    process.VerificationRunner
	HTTP      HTTPDoer
	Discovery discovery.Discoverer
	// Clients is the composition root's adapter registry; every client
	// operation of this invocation goes through it.
	Clients adapter.Registry
	Updater Updater
	// Follows collects what this invocation made of the clients' own
	// choices, for the invocation's report; nil records nothing.
	Follows *synchronization.Follows
	// Stops collects the clients that this invocation's transitions stopped,
	// for the invocation's report; nil records nothing.
	Stops *synchronization.Stops
}

// LoadConfiguration reads the configuration that every command works from:
// the stored one with the model that each enabled client chose in its own
// file recorded as that client's selection. The invocation's record keeps
// those choices, which the stored configuration does not hold yet.
func (d Dependencies) LoadConfiguration() (domain.Config, error) {
	stored, err := d.Config.Load()
	if err != nil {
		return domain.Config{}, err
	}
	working, followed := d.Synchronizer().Working(stored)
	d.Follows.Observe(followed)
	return working, nil
}

// SaveVerifiedSnapshot records a completed check of cfg, the configuration
// that the check read, while the stored configuration and the clients' own
// files still give it.
func (d Dependencies) SaveVerifiedSnapshot(ctx context.Context, cfg domain.Config, clients []store.VerifiedClient) error {
	return d.Config.SaveVerifiedCheckpoint(ctx, cfg, clients, func(stored domain.Config) domain.Config {
		working, _ := d.Synchronizer().Working(stored)
		return working
	})
}

// InspectClient observes one client's projection without changing it.
func (d Dependencies) InspectClient(ctx context.Context, cfg domain.Config, clientID string, runtime domain.Runtime) adapter.Status {
	return d.Synchronizer().Inspect(ctx, cfg, clientID, runtime)
}

// Discover reads client discovery through the invocation's discoverer: of
// the named clients, or of every admitted client when none is named.
func (d Dependencies) Discover(clients ...string) (discovery.Result, error) {
	if d.Discovery == nil {
		return discovery.Result{}, fmt.Errorf("client discovery is unavailable")
	}
	return d.Discovery.Discover(clients...), nil
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
		ResolveCredentialPath: d.ResolveCredentialPath,
		ResolveProgram:        d.ResolveProgram,
		Files:                 d.Files,
		Follows:               d.Follows,
		Stops:                 d.Stops,
	}
}
