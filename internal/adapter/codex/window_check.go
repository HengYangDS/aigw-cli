package codex

import (
	"fmt"
	"os"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/codex/modelcatalog"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// windowCheck is the passing check, numbered as the target's own check and
// naming the same target, of the context window that Codex is given for the
// selected model at target. It reads the model table that AIGW wrote for the
// target and runs no client. Its detail names the window that the model's
// entry carries, or, for a model that the table has no entry for, says that
// Codex uses its own fallback window, of which AIGW keeps no figure. It
// reports false, and the target's own check owns the failure, where AIGW
// recorded no table for the target, where the file is not the one that it
// recorded or cannot be parsed, and where the model's entry carries no window.
func windowCheck(index int, target string, runtime domain.Runtime) (adapter.Check, bool) {
	table, owned := ownedCatalog(target)
	if !owned {
		return adapter.Check{}, false
	}
	check := adapter.Check{ID: fmt.Sprintf("codex:window-%d", index+1), Ready: true, Target: target}
	model := runtime.Model.String()
	const fallback = ", so Codex uses its own fallback window whatever the model declares"
	switch window, _ := table.ContextWindow(runtime.UpstreamModel); {
	case window > 0:
		check.Detail = fmt.Sprintf("Codex's model table gives %s a context window of %d tokens", model, window)
	case table.Model(runtime.UpstreamModel) != nil:
		return adapter.Check{}, false
	case table.Model(runtime.FamilyID) == nil:
		check.Detail = "Codex's model table has no entry for the family " + runtime.FamilyID + " of " + model + fallback
	default:
		check.Detail = "Codex's model table has no entry for " + model + fallback
	}
	return check, true
}

// ownedCatalog reads the model table that AIGW recorded writing for the Codex
// file at target, and reports false where there is none that it can read.
func ownedCatalog(target string) (modelcatalog.Document, bool) {
	canonical, err := canonicalCodexTargetPath(target)
	if err != nil {
		return modelcatalog.Document{}, false
	}
	record, err := os.ReadFile(codexRecordPath(target))
	if err != nil {
		return modelcatalog.Document{}, false
	}
	state, err := codexStateForTarget(platform.FileSnapshot{Exists: true, Data: record})
	if err != nil || state.CatalogHash == "" {
		return modelcatalog.Document{}, false
	}
	// The plan writes the table beside the resolved file.
	data, err := os.ReadFile(codexCatalogPath(canonical))
	if err != nil || platform.SHA256(data) != state.CatalogHash {
		return modelcatalog.Document{}, false
	}
	table, err := modelcatalog.Parse(data)
	return table, err == nil
}
