// Package catalog implements `aigw catalog`, which reports each Account's
// authenticated model catalogue.
package catalog

import (
	"fmt"
	"strings"

	"aigw-cli/internal/app"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// NewCommand constructs authenticated catalog discovery.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode, all bool
	cmd := &cobra.Command{Use: "catalog", Short: "Discover authenticated model catalogs for each account (compact summary by default)", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		cfg, err := runtime.Config.Load()
		if err != nil {
			return err
		}
		if len(cfg.Routes) == 0 {
			if jsonMode {
				return terminal.WriteJSON(runtime.Out, app.CatalogReport{Observations: []app.CatalogObservation{}})
			}
			return fmt.Errorf("not configured; run `aigw setup`")
		}
		result := app.DiscoverCatalog(cmd.Context(), runtime.Secrets, runtime.HTTP, cfg)
		if jsonMode {
			return terminal.WriteJSON(runtime.Out, result)
		}
		r := invocation.Renderer(runtime)
		r.ProductTitle("Authenticated model catalog")
		r.Text("Each observation proves catalogue membership only for its named protocol source.")
		for _, observation := range result.Observations {
			title := observation.Label + " · " + observation.Account
			if observation.Source.Protocol != "" {
				title += " · " + domain.ProtocolLabel(observation.Source.Protocol)
			}
			r.Section(title)
			if observation.Status != app.CatalogOK {
				r.Rows(terminal.Field{State: terminal.Warn, Label: "Catalog", Value: observation.Status.Label()})
				continue
			}
			rows := []terminal.Field{
				{Label: "Observation", Value: observation.ObservationID},
				{Label: "Source", Value: observation.Source.Endpoint},
			}
			if len(observation.Models) == 0 {
				r.Rows(append(rows, terminal.Field{State: terminal.Info, Label: "model", Value: "Upstream returned an empty catalog"})...)
			} else {
				renderCatalogObservation(r, rows, observation, all)
			}
			renderMissingRoutes(r, observation.MissingRoutes)
		}
		r.Next("aigw route add <route> --account <account> --model <model> --protocol <protocol>")
		return r.Err()
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	cmd.Flags().BoolVar(&all, "all", false, "Show every model in human output (JSON already includes all)")
	return cmd
}

// renderCatalogObservation completes an observation's group with its counts and
// visible models, then notes the candidates that compact output omits.
func renderCatalogObservation(r *terminal.Renderer, rows []terminal.Field, observation app.CatalogObservation, all bool) {
	visible := make([]app.CatalogModel, 0, len(observation.Models))
	admitted, deprecated, candidates := 0, 0, 0
	for _, model := range observation.Models {
		switch model.State {
		case app.CatalogAdmitted:
			admitted++
			visible = append(visible, model)
		case app.CatalogDeprecated:
			deprecated++
			visible = append(visible, model)
		case app.CatalogCandidate:
			candidates++
		}
	}
	rows = append(rows,
		terminal.Field{Label: "Models", Value: fmt.Sprintf("%d observed", len(observation.Models))},
		terminal.Field{Label: "Admitted", Value: fmt.Sprintf("%d admitted", admitted)},
	)
	if deprecated > 0 {
		rows = append(rows, terminal.Field{Label: "Deprecated", Value: fmt.Sprintf("%d deprecated", deprecated)})
	}
	if all {
		visible = observation.Models
	}
	for _, model := range visible {
		state, detail := catalogModelDisplay(model)
		rows = append(rows, terminal.Field{State: state, Label: "model", Value: model.ID, Details: []string{detail}})
	}
	r.Rows(rows...)
	if !all && candidates > 0 {
		r.Detail(fmt.Sprintf("%d candidate models require qualification; full catalog: aigw catalog --all", candidates))
	}
}

func catalogModelDisplay(model app.CatalogModel) (terminal.State, string) {
	if model.State == app.CatalogCandidate {
		return terminal.Info, "Candidate; protocol and client capabilities are unqualified"
	}
	routes := make([]string, 0, len(model.Routes))
	for _, route := range model.Routes {
		routes = append(routes, route.ID)
	}
	if model.State == app.CatalogDeprecated {
		return terminal.Warn, "Deprecated Routes: " + strings.Join(routes, ", ")
	}
	return terminal.OK, "Admitted Routes: " + strings.Join(routes, ", ")
}

func renderMissingRoutes(r *terminal.Renderer, routes []app.CatalogRoute) {
	for _, route := range routes {
		r.StatusLine(terminal.Warn, "Route", route.ID)
		r.Detail(route.UpstreamModel + " · missing from this observation; requalification required")
	}
}
