package claudedesktop

import (
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/client/hermes"
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
	return configuration.NewAdmission(claude.New().Spec(), New().Spec(), codex.New().Spec(), hermes.New().Spec())
}
