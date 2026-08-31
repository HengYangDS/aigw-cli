// Package synchronization owns atomic convergence between AIGW configuration,
// Codex projections, and Codex native authentication.
package synchronization

import (
	"context"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
)

// Runner executes the bounded native authentication plan emitted by Codex.
type Runner interface {
	Run(context.Context, process.Plan) error
}

// ConfigStore is the exact persistence capability needed by a synchronization
// transaction. Keeping this boundary narrow makes failure and rollback
// semantics independently testable without substituting host filesystem
// behavior.
type ConfigStore interface {
	CaptureSnapshot() (configuration.Snapshot, error)
	Save(configuration.Config) error
	RestoreSnapshot(configuration.Snapshot, configuration.Snapshot) error
}

// Synchronizer carries the explicit dependencies required for one convergence
// transaction. It does not discover credentials, paths, or executables on its
// own, which keeps host and test policy outside the synchronization domain.
type Synchronizer struct {
	Config             ConfigStore
	Secrets            secrets.Store
	Runner             Runner
	Discovery          discovery.Discoverer
	ClaudeSettingsPath string
	AIGWExecutable     string
}

// DesiredClientConfiguration discovers the requested clients and returns the
// configuration that can be activated with the currently available
// executables, surfaces, routes, and credentials. An empty client list means
// every admitted client; an explicit list leaves every other adapter unchanged.
func (s Synchronizer) DesiredClientConfiguration(before configuration.Config, clients ...string) (configuration.Config, discovery.Result, error) {
	discovered, err := s.discoveredResult()
	if err != nil {
		return configuration.Config{}, discovery.Result{}, err
	}
	if len(clients) == 0 {
		clients = configuration.AdmittedClientIDs()
	}
	after := before.Clone()
	for _, client := range clients {
		switch client {
		case configuration.ClientClaude:
			s.convergeClaude(&after, discovered)
		case configuration.ClientCodex:
			s.convergeCodex(&after, discovered)
		}
	}
	return after, discovered, nil
}

func (s Synchronizer) convergeClaude(cfg *configuration.Config, discovered discovery.Result) {
	clientRuntime, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		return
	}
	adapter := cfg.Adapters[configuration.ClientClaude]
	executable := adapter.Executable
	if executable == "" {
		executable = discovered.Executable(configuration.ClientClaude)
	}
	if executable != "" && (adapter.Enabled || s.secretAvailable(clientRuntime.AccountID)) {
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executable}
	}
}

func (s Synchronizer) convergeCodex(cfg *configuration.Config, discovered discovery.Result) {
	clientRuntime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		return
	}
	adapter := cfg.Adapters[configuration.ClientCodex]
	targets := codexTargets(discovered, adapter.Targets)
	executable := adapter.Executable
	if executable == "" {
		executable = discovered.Executable(configuration.ClientCodex)
	}
	if executable != "" && len(targets) > 0 && (adapter.Enabled || s.secretAvailable(clientRuntime.AccountID)) {
		cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executable, Targets: targets}
	} else if adapter.Enabled && len(targets) == 0 {
		delete(cfg.Adapters, configuration.ClientCodex)
	}
}

func (s Synchronizer) secretAvailable(accountID string) bool {
	return s.Secrets != nil && s.Secrets.Has(accountID)
}

func codexTargets(discovered discovery.Result, current []string) []string {
	seen := map[string]bool{}
	targets := make([]string, 0, len(current)+len(discovered.Surfaces))
	appendTarget := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			targets = append(targets, path)
		}
	}
	for _, path := range discovered.AutoManagedCodexTargets() {
		appendTarget(path)
	}
	for _, path := range current {
		if surface, ok := discovered.SurfaceForConfigPath(path); ok {
			if surface.ID == string(surfaceidentity.CodexHomeDefault) {
				appendTarget(path)
			}
			continue
		}
		appendTarget(path)
	}
	return targets
}
