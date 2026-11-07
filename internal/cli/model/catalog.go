package model

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/app"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// checkCatalog applies the catalog check of `aigw model add`: it reads the
// provider's authenticated catalog through the observation that `aigw model
// search` shares, on each of the provider's protocols, and admits the model
// only when a catalog lists its ID. A catalog that cannot be read on any
// protocol, or that lists no model, is refused with the same command and
// --unlisted, and a catalog that does not list the model with its closest
// listed IDs.
func checkCatalog(ctx context.Context, runtime invocation.Context, cmd *cobra.Command, cfg domain.Config, ref domain.ModelRef) error {
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
	unlisted := unlistedCommand(cmd)
	if !read || len(listed) == 0 {
		reason := "cannot be read"
		if read {
			reason = "lists no model"
		}
		problem := fmt.Sprintf("the %s catalog %s, so AIGW cannot check that it serves %s", ref.Provider(), reason, ref)
		return terminal.ProblemError(problem, unreadSources(report), "", unlisted, nil)
	}
	return unlistedModel(cmd, cfg, ref, listed, unreadSources(report))
}

// unlistedModel refuses a model ID that the catalog does not list. The
// closest listed ID is most likely what the user meant: an added one is named
// as added, with `aigw model show`, and another one is added instead. Only a
// catalog without a close ID leaves --unlisted, which adds the ID as typed; it
// is never the exact fix for what is likely a typo of an added model, and it
// asks the user to confirm the ID first.
func unlistedModel(cmd *cobra.Command, cfg domain.Config, ref domain.ModelRef, listed []string, evidence string) error {
	problem := fmt.Sprintf("the %s catalog does not list %s", ref.Provider(), ref.ID())
	closest := closestListed(ref.ID(), listed)
	if len(closest) == 0 {
		return terminal.ProblemError(problem, evidence, "", confirmUnlisted(cmd), nil)
	}
	nearest := domain.NewModelRef(ref.Provider(), closest[0])
	if len(cfg.ResolveModelRef(nearest)) > 0 {
		fix := invocation.Command("model", "show", nearest.String())
		if fix == "" {
			fix = "Run `aigw model show` for " + nearest.String() + "."
		}
		return terminal.ProblemError(problem+"; "+nearest.String()+" is already added", evidence, "", fix, nil)
	}
	problem += "; did you mean " + ref.Provider() + "/" + strings.Join(closest, ", "+ref.Provider()+"/") + "?"
	args := append([]string{nearest.String()}, cmd.Flags().Args()[1:]...)
	fix := invocation.Rerun(cmd, args, nil)
	if fix == "" {
		fix = confirmUnlisted(cmd)
	}
	return terminal.ProblemError(problem, evidence, "", fix, nil)
}

// confirmUnlisted is the fix that adds a model ID that the catalog does not
// list, after the user confirms it: prose, so that JSON carries it as
// next_detail and no script adds a typo.
func confirmUnlisted(cmd *cobra.Command) string {
	if line := invocation.Rerun(cmd, cmd.Flags().Args(), nil, "--unlisted"); line != "" {
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
	return invocation.ClosestMatches(id, slices.Compact(slices.Sorted(slices.Values(listed))))
}

// unlistedCommand is the command that adds the model without the catalog
// check: this command line with --unlisted.
func unlistedCommand(cmd *cobra.Command) string {
	if line := invocation.Rerun(cmd, cmd.Flags().Args(), nil, "--unlisted"); line != "" {
		return line
	}
	return "Run the same command with --unlisted."
}
