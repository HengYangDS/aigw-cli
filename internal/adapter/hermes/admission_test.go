package hermes

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/domain"
)

// testAdmission admits this adapter's own specification.
func testAdmission() domain.Admission {
	return domain.NewAdmission(New().Spec())
}

// builtInAdmission admits every built-in adapter's specification in the
// composition root's order, for the tests that read the shipped team manifest,
// which recommends Routes for all of them.
func builtInAdmission() domain.Admission {
	return domain.NewAdmission(claude.New().Spec(), claudedesktop.New().Spec(), codex.New().Spec(), New().Spec())
}
