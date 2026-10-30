// Package app holds the use cases that several AIGW commands, or the CLI root
// on a command's behalf, share. A use case receives the capabilities that it
// needs, usually the invocation's Dependencies, and returns values for the
// terminal to render; it imports neither the command tree nor the terminal.
package app

import (
	"context"
	"fmt"
	"net/http"

	"aigw-cli/internal/app/renaming"
	"aigw-cli/internal/app/synchronization"
	"aigw-cli/internal/client"
	"aigw-cli/internal/client/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
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
	CredentialPath        string
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
	Accounts  secret.DiagnosticCredentialStore
	Runner    process.VerificationRunner
	HTTP      HTTPDoer
	Discovery discovery.Discoverer
	// Clients is the composition root's adapter registry; every client
	// operation of this invocation goes through it.
	Clients client.Registry
	Updater Updater
	// ClientInspector observes one client's projection in place of the
	// synchronizer in every inspection of this invocation; nil means the
	// synchronizer inspects it.
	ClientInspector func(ctx context.Context, cfg domain.Config, clientID string, runtime domain.Runtime) client.Status
}

// InspectClient observes one client's projection without changing it.
func (d Dependencies) InspectClient(ctx context.Context, cfg domain.Config, clientID string, runtime domain.Runtime) client.Status {
	if d.ClientInspector != nil {
		return d.ClientInspector(ctx, cfg, clientID, runtime)
	}
	return d.Synchronizer().Inspect(ctx, cfg, clientID, runtime)
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

// Renamer migrates Account and Route identities with these capabilities.
func (d Dependencies) Renamer() renaming.Renamer {
	return renaming.Renamer{
		Config: d.Config, Secrets: d.Secrets, Accounts: d.Accounts,
		HTTP: d.HTTP, Synchronizer: d.Synchronizer(),
	}
}
