package codex

import (
	"fmt"
	"os"
	"slices"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/codex/modelcatalog"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// windowCheck is the check, numbered as the target's own check and naming the
// same target, of the context window that Codex is given for the selected
// model at target. It reads the model table that AIGW wrote for the target and
// runs no client. Where the table holds an entry for the model's ID, the check
// passes and names the window while both window fields of that entry carry the
// window that the model declares; otherwise it is not ready, names what the
// entry carries and what the model declares, and is repaired by a sync, which
// writes the table again. For a model that the table has no entry for, it
// passes and says that Codex chooses the window itself: Codex takes the entry
// whose slug is the longest prefix of the model's ID before it falls back to a
// window of its own, and AIGW keeps a figure of neither. Where AIGW writes no
// table for the target, for a reason that tableLeftOut names, it passes and
// says that Codex is not given the declared window, and why. It reports false,
// and the target's own check owns the failure, where AIGW's table for the
// target cannot be read or is missing for any other reason.
func windowCheck(index int, target string, runtime domain.Runtime) (adapter.Check, bool) {
	check := adapter.Check{ID: fmt.Sprintf("codex:window-%d", index+1), Ready: true, Target: target}
	model, declared := runtime.Model.String(), runtime.ContextWindow
	table, owned := ownedCatalog(target)
	if !owned {
		reason := tableLeftOut(target)
		if reason == "" {
			return adapter.Check{}, false
		}
		check.Detail = fmt.Sprintf("Codex is not given the window of %d tokens that %s declares, because %s", declared, model, reason)
		return check, true
	}
	const chooses = ", so Codex chooses the window itself, whatever the model declares"
	switch table.DescribedBy(runtime.UpstreamModel, runtime.FamilyID) {
	case "":
		check.Detail = "Codex's model table has no entry for the family " + runtime.FamilyID + " of " + model + chooses
	case runtime.UpstreamModel:
		window, ceiling := table.ContextWindow(runtime.UpstreamModel)
		check.Detail = "Codex's model table gives " + model + " " + carried("context window", window)
		if window != declared || ceiling != declared {
			check.Ready, check.RepairAction = false, "aigw sync"
			check.Detail += fmt.Sprintf(" and %s, but the model declares %d tokens", carried("ceiling", ceiling), declared)
		}
	default:
		check.Detail = "Codex's model table has no entry for " + model + chooses
	}
	return check, true
}

// tableLeftOut names why AIGW writes no model table for the Codex file at
// target, for the reason of noTableReason, on which the projection
// (codexCatalogProjection) writes none. The projection reads the text it
// starts from, which holds no line of AIGW's; this reads the file itself, where
// a line that AIGW wrote is not the user's own table. A line that it cannot
// read counts as none.
func tableLeftOut(target string) string {
	own := ""
	if text, readErr := os.ReadFile(target); readErr == nil {
		if line, lineErr := codexSelectionLine(string(text), "model_catalog_json"); lineErr == nil && !managedLine(line) {
			own = line
		}
	}
	return noTableReason(own)
}

// carried words what an entry carries in the window field that a sentence
// calls name: tokens as [modelcatalog.Document.ContextWindow] reads the field,
// of which zero stands for a field that the entry does not carry as a whole
// number above zero.
func carried(name string, tokens int) string {
	if tokens == 0 {
		return "no " + name
	}
	return fmt.Sprintf("a %s of %d tokens", name, tokens)
}

// listingReaches reports whether a change from the listed models before to
// those after can change a file of Codex's at one of targets: a model added,
// removed or moved, or one whose name or declared window changed. It can where
// the table that AIGW wrote for a target describes such a model, by its ID or
// by its family, and it is taken to where that table cannot be read, unless
// AIGW writes no table for the target (see tableLeftOut); a table that
// describes the model by neither gives it no entry.
func listingReaches(targets []string, before, after []domain.ListedModel) bool {
	var changed []domain.ListedModel
	for index := range max(len(before), len(after)) {
		if index < len(before) && index < len(after) && before[index] == after[index] {
			continue
		}
		if index < len(before) {
			changed = append(changed, before[index])
		}
		if index < len(after) {
			changed = append(changed, after[index])
		}
	}
	return len(changed) > 0 && slices.ContainsFunc(targets, func(target string) bool {
		table, owned := ownedCatalog(target)
		if !owned {
			// Where AIGW writes no table for the target, no listing reaches it.
			return tableLeftOut(target) == ""
		}
		return slices.ContainsFunc(changed, func(model domain.ListedModel) bool {
			return table.DescribedBy(model.ID, model.Family) != ""
		})
	})
}

// ownedCatalog parses the model table that AIGW recorded writing for the Codex
// file at target, and reports false where there is none that it can read.
func ownedCatalog(target string) (modelcatalog.Document, bool) {
	record, err := os.ReadFile(codexRecordPath(target))
	if err != nil {
		return modelcatalog.Document{}, false
	}
	state, err := codexStateForTarget(platform.FileSnapshot{Exists: true, Data: record})
	if err != nil {
		return modelcatalog.Document{}, false
	}
	data, err := recordedCatalog(target, state)
	if err != nil || data == nil {
		return modelcatalog.Document{}, false
	}
	table, err := modelcatalog.Parse(data)
	return table, err == nil
}
