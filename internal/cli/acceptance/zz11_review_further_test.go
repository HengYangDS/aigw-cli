package cli_test

import (
	"testing"
)

// This file holds the eleventh review's runs of the matrix's own commands and
// checker in states of Codex and Hermes that the matrix does not build.

// zz11After runs a last change in a Codex world, after which a rollback
// restores gpt-sol with what the change replaced, and then picks model in the
// first file.
func zz11After(count int, model string, change ...string) func(t *testing.T) selectionWorld {
	return func(t *testing.T) selectionWorld {
		world := codexWorld(t, count)
		world.run(t, change...)
		world.restored = "gpt-sol"
		return world.picking(t, 0, basicString, model)
	}
}

// zz11HermesAfter runs a last change in the plain Hermes world, after which a
// rollback restores the stored selection with what the change replaced, and
// then picks model of account on protocol.
func zz11HermesAfter(account, protocol, model string, change ...string) func(t *testing.T) selectionWorld {
	return func(t *testing.T) selectionWorld {
		world := hermesPlain(t)
		world.run(t, change...)
		return world.choosing(t, account, protocol, model, plainScalars)
	}
}
