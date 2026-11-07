package invocation

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/app"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
)

// CatalogCommands are the commands that a catalog refusal names as its fix,
// each "" when a word would need quoting: Rerun, the command again for the
// model named model with the added words after it, and Added, the command
// for a model that is already added.
type CatalogCommands struct {
	Rerun func(model string, added ...string) string
	Added func(model string) string
}

// CheckCatalog applies the catalog check of adding a model, which `aigw model
// add` and `aigw use` share: it reads the provider's authenticated catalog
// through the observation that `aigw model search` shares, on each of the
// provider's protocols, with runtime's Tokens, and admits the model only
// when a catalog lists its ID. A catalog that cannot be read on any protocol,
// or that lists no model, is refused with the command rerun with --unlisted,
// and a catalog that does not list the model with its closest listed IDs.
func CheckCatalog(ctx context.Context, runtime Context, cfg domain.Config, ref domain.ModelRef, commands CatalogCommands) error {
	scoped := cfg.Clone()
	for id := range scoped.Providers {
		if id != ref.Provider() {
			delete(scoped.Providers, id)
		}
	}
	report := app.DiscoverCatalog(ctx, runtime.Secrets, runtime.HTTP, scoped)
	var listed []string
	read := false
	for _, observation := range report.Observations {
		if observation.Status != app.CatalogOK {
			continue
		}
		read = true
		for _, model := range observation.Models {
			listed = append(listed, model.ID)
		}
	}
	if slices.Contains(listed, ref.ID()) {
		return nil
	}
	unlisted := unlistedCommand(ref, commands)
	if !read || len(listed) == 0 {
		reason := "cannot be read"
		if read {
			reason = "lists no model"
		}
		problem := fmt.Sprintf("the %s catalog %s, so AIGW cannot check that it serves %s", ref.Provider(), reason, ref)
		return terminal.ProblemError(problem, unreadSources(report), "", unlisted, nil)
	}
	return unlistedModel(cfg, ref, listed, unreadSources(report), commands)
}

// unlistedModel refuses a model ID that the catalog does not list. The
// closest listed ID is most likely what the user meant: an added one is named
// as added, with `aigw model show`, and another one is added instead. Only a
// catalog without a close ID leaves --unlisted, which adds the ID as typed; it
// is never the exact fix for what is likely a typo of an added model, and it
// asks the user to confirm the ID first.
func unlistedModel(cfg domain.Config, ref domain.ModelRef, listed []string, evidence string, commands CatalogCommands) error {
	problem := fmt.Sprintf("the %s catalog does not list %s", ref.Provider(), ref.ID())
	closest := closestListed(ref.ID(), listed)
	if len(closest) == 0 {
		return terminal.ProblemError(problem, evidence, "", confirmUnlisted(ref, commands), nil)
	}
	nearest := domain.NewModelRef(ref.Provider(), closest[0])
	if len(cfg.ResolveModelRef(nearest)) > 0 {
		fix := commands.Added(nearest.String())
		if fix == "" {
			fix = "Run `aigw model show` for " + nearest.String() + "."
		}
		return terminal.ProblemError(problem+"; "+nearest.String()+" is already added", evidence, "", fix, nil)
	}
	problem += "; did you mean " + ref.Provider() + "/" + strings.Join(closest, ", "+ref.Provider()+"/") + "?"
	fix := commands.Rerun(nearest.String())
	if fix == "" {
		fix = confirmUnlisted(ref, commands)
	}
	return terminal.ProblemError(problem, evidence, "", fix, nil)
}

// confirmUnlisted is the fix that adds a model ID that the catalog does not
// list, after the user confirms it: prose, so that JSON carries it as
// next_detail and no script adds a typo.
func confirmUnlisted(ref domain.ModelRef, commands CatalogCommands) string {
	if line := commands.Rerun(ref.String(), "--unlisted"); line != "" {
		return "If the provider serves this exact model ID, run `" + line + "`."
	}
	return "If the provider serves this exact model ID, run the same command with --unlisted."
}

// unreadSources names each of the provider's catalog sources that could not
// be read, and why, or nothing when every source was read.
func unreadSources(report app.CatalogReport) string {
	var reasons []string
	for _, observation := range report.Observations {
		if observation.Status == app.CatalogOK {
			continue
		}
		source := "No endpoint"
		if observation.Source.Protocol != "" {
			source = domain.ProtocolLabel(observation.Source.Protocol)
		}
		reasons = append(reasons, source+": "+observation.Status.Label())
	}
	return strings.Join(reasons, "; ")
}

// closestListed returns the listed IDs closest to id.
func closestListed(id string, listed []string) []string {
	return closestMatches(id, slices.Compact(slices.Sorted(slices.Values(listed))))
}

// unlistedCommand is the command that adds the model without the catalog
// check: this command line with --unlisted.
func unlistedCommand(ref domain.ModelRef, commands CatalogCommands) string {
	if line := commands.Rerun(ref.String(), "--unlisted"); line != "" {
		return line
	}
	return "Run the same command with --unlisted."
}
