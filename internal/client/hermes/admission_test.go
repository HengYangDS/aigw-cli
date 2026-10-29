package hermes

import (
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/claudedesktop"
	"aigw-cli/internal/client/codex"
	configuration "aigw-cli/internal/configuration"
)

// testAdmission admits this adapter's own specification.
func testAdmission() configuration.Admission {
	return configuration.NewAdmission(New().Spec())
}

// builtInAdmission admits every built-in adapter's specification in the
// composition root's order, for the tests that read the shipped team manifest,
// which recommends Routes for all of them.
func builtInAdmission() configuration.Admission {
	return configuration.NewAdmission(claude.New().Spec(), claudedesktop.New().Spec(), codex.New().Spec(), New().Spec())
}
