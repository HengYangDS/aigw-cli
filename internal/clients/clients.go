// Package clients is the composition root's list of built-in client adapters.
package clients

import (
	"aigw-cli/internal/client"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/claudedesktop"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/client/hermes"
	"aigw-cli/internal/domain"
)

// Registry registers every built-in adapter in admission order; each adapter's
// Spec is its client's admission record. The binary injects it into the CLI,
// and tests that exercise real adapters use it too. An adapter set that the
// registry refuses is a build defect, so it panics rather than starting
// without one.
func Registry() client.Registry {
	registry, err := client.NewRegistry(adapters()...)
	if err != nil {
		panic(err)
	}
	return registry
}

// Admission returns the built-in clients that every configuration of this
// program admits, in admission order. Tests that build configurations for the
// built-in clients take it too.
func Admission() domain.Admission { return Registry().Admission() }

// adapters lists every built-in client adapter in admission order.
func adapters() []client.Adapter {
	return []client.Adapter{
		claude.New(),
		claudedesktop.New(),
		codex.New(),
		hermes.New(),
	}
}
