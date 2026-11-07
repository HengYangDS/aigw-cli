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
	// The fix adds the closest listed model that is not added yet, whose ID
	// the user most likely meant; only a catalog without one leaves
	// --unlisted, which adds the ID as typed.
	problem := fmt.Sprintf("the %s catalog does not list %s", ref.Provider(), ref.ID())
	closest := closestListed(ref.ID(), slices.DeleteFunc(listed, func(id string) bool {
		return len(cfg.ResolveModelRef(domain.NewModelRef(ref.Provider(), id))) > 0
	}))
	if len(closest) == 0 {
		return terminal.ProblemError(problem, unreadSources(report), "", unlisted, nil)
	}
	problem += "; did you mean " + ref.Provider() + "/" + strings.Join(closest, ", "+ref.Provider()+"/") + "?"
	args := append([]string{ref.Provider() + "/" + closest[0]}, cmd.Flags().Args()[1:]...)
	fix := invocation.Rerun(cmd, args, nil)
	if fix == "" {
		fix = unlisted
	}
	return terminal.ProblemError(problem, unreadSources(report), "", fix, nil)
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
