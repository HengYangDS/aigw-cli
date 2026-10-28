// Package clients is the composition root's list of built-in client adapters.
package clients

import (
	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
)

// Registry registers every built-in adapter against the admitted client
// records. The binary injects it into the CLI, and tests that exercise real
// adapters use it too. An adapter set that does not match the admission
// records is a build defect, so it panics rather than starting without one.
func Registry() client.Registry {
	registry, err := client.NewRegistry(
		configuration.AdmittedClientSpecs(),
		client.NewClaudeAdapter(),
		client.NewClaudeDesktopAdapter(),
		client.NewCodexAdapter(),
		client.NewHermesAdapter(),
	)
	if err != nil {
		panic(err)
	}
	return registry
}
