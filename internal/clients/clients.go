// Package clients is the composition root's list of built-in client adapters.
package clients

import (
	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/domain"
)

// Registry registers every built-in adapter in admission order; each adapter's
// Spec is its client's admission record. The binary injects it into the CLI,
// and tests that exercise real adapters use it too. An adapter set that the
// registry refuses is a build defect, so it panics rather than starting
// without one.
func Registry() adapter.Registry {
	registry, err := adapter.NewRegistry(adapters()...)
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
func adapters() []adapter.Adapter {
	return []adapter.Adapter{
		claude.New(),
		claudedesktop.New(),
		codex.New(),
		hermes.New(),
	}
}
