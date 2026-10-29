// Package catalog owns provider-neutral model discovery and presentation.
package catalog

import (
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/store"

	"github.com/spf13/cobra"
)

// HTTPDoer executes catalog requests.
type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

// Dependencies are the capabilities required by model commands.
type Dependencies struct {
	Config    store.Store
	Secrets   secrets.Store
	HTTP      HTTPDoer
	Out       io.Writer
	Color     bool
	Width     int
	RenderOut io.Writer
}

type modelRow struct {
	Route    string
	Account  string
	Protocol domain.EndpointProtocol
	Model    string
	Catalog  string
}

// NewModelsCommand constructs the configured-model availability command.
func NewModelsCommand(deps Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "Compare configured model IDs with provider catalogs; does not test inference",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := deps.Config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Routes) == 0 {
				return fmt.Errorf("not configured; run `aigw setup`")
			}
			rows := modelRows(cfg, discoverCatalog(cmd.Context(), deps, cfg))
			r := renderer(deps)
			r.ProductTitle("Configured model catalog")
			r.Text("Catalog membership does not prove protocol capabilities, inference, or client readiness.")
			r.Section("Routes")
			for _, row := range rows {
				state := presentation.Warn
				if row.Catalog == "Listed" || row.Catalog == "Not listed" {
					state = presentation.Info
				}
				r.StatusLine(state, "Route", row.Route)
				r.Detail(fmt.Sprintf("%s · %s · %s · account %s", row.Model, protocolTitle(row.Protocol), row.Catalog, row.Account))
			}
			r.Next("aigw use")
			return r.Err()
		},
	}
}

func modelRows(cfg domain.Config, catalog catalogOutput) []modelRow {
	observations := make(map[string]catalogObservation, len(catalog.Observations))
	for _, observation := range catalog.Observations {
		observations[observationKey(observation.Account, observation.Source.Protocol)] = observation
	}
	rows := []modelRow{}
	for _, routeID := range cfg.CatalogRouteIDs() {
		route := cfg.Routes[routeID]
		protocols := route.AdmittedProtocols()
		if len(protocols) == 0 {
			rows = append(rows, modelRow{Route: routeID, Account: route.Account, Model: route.UpstreamModelID(), Catalog: "Catalog not observed"})
			continue
		}
		for _, protocol := range protocols {
			membership := "Catalog not observed"
			if observation, ok := observations[observationKey(route.Account, protocol)]; ok {
				membership = catalogStatusText(observation.Status)
				if observation.Status == catalogOK {
					membership = "Not listed"
					if slices.ContainsFunc(observation.Models, func(model catalogModel) bool { return model.ID == route.UpstreamModelID() }) {
						membership = "Listed"
					}
				}
			}
			rows = append(rows, modelRow{Route: routeID, Account: route.Account, Protocol: protocol, Model: route.UpstreamModelID(), Catalog: membership})
		}
	}
	slices.SortStableFunc(rows, func(left, right modelRow) int {
		if result := cfg.CompareAccounts(left.Account, right.Account); result != 0 {
			return result
		}
		if result := domain.CompareProtocols(left.Protocol, right.Protocol); result != 0 {
			return result
		}
		return cfg.CompareModels(cfg.Routes[left.Route].Model, cfg.Routes[right.Route].Model)
	})
	return rows
}

// NewCatalogCommand constructs authenticated catalog discovery.
func NewCatalogCommand(deps Dependencies) *cobra.Command {
	var jsonMode, all bool
	cmd := &cobra.Command{Use: "catalog", Short: "Discover authenticated model catalogs for each account (compact summary by default)", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		cfg, err := deps.Config.Load()
		if err != nil {
			return err
		}
		if len(cfg.Routes) == 0 {
			if jsonMode {
				return presentation.WriteJSON(deps.Out, catalogOutput{Observations: []catalogObservation{}})
			}
			return fmt.Errorf("not configured; run `aigw setup`")
		}
		result := discoverCatalog(cmd.Context(), deps, cfg)
		if jsonMode {
			return presentation.WriteJSON(deps.Out, result)
		}
		r := renderer(deps)
		r.ProductTitle("Authenticated model catalog")
		r.Text("Each observation proves catalogue membership only for its named protocol source.")
		for _, observation := range result.Observations {
			title := observation.Label + " · " + observation.Account
			if observation.Source.Protocol != "" {
				title += " · " + protocolTitle(observation.Source.Protocol)
			}
			r.Section(title)
			if observation.Status != catalogOK {
				r.Rows(presentation.Field{State: presentation.Warn, Label: "Catalog", Value: catalogStatusText(observation.Status)})
				continue
			}
			rows := []presentation.Field{
				{Label: "Observation", Value: observation.ObservationID},
				{Label: "Source", Value: observation.Source.Endpoint},
			}
			if len(observation.Models) == 0 {
				r.Rows(append(rows, presentation.Field{State: presentation.Info, Label: "model", Value: "Upstream returned an empty catalog"})...)
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
func renderCatalogObservation(r *presentation.Renderer, rows []presentation.Field, observation catalogObservation, all bool) {
	visible := make([]catalogModel, 0, len(observation.Models))
	admitted, deprecated, candidates := 0, 0, 0
	for _, model := range observation.Models {
		switch model.State {
		case catalogAdmitted:
			admitted++
			visible = append(visible, model)
		case catalogDeprecated:
			deprecated++
			visible = append(visible, model)
		case catalogCandidate:
			candidates++
		}
	}
	rows = append(rows,
		presentation.Field{Label: "Models", Value: fmt.Sprintf("%d observed", len(observation.Models))},
		presentation.Field{Label: "Admitted", Value: fmt.Sprintf("%d admitted", admitted)},
	)
	if deprecated > 0 {
		rows = append(rows, presentation.Field{Label: "Deprecated", Value: fmt.Sprintf("%d deprecated", deprecated)})
	}
	if all {
		visible = observation.Models
	}
	for _, model := range visible {
		state, detail := catalogModelDisplay(model)
		rows = append(rows, presentation.Field{State: state, Label: "model", Value: model.ID, Details: []string{detail}})
	}
	r.Rows(rows...)
	if !all && candidates > 0 {
		r.Detail(fmt.Sprintf("%d candidate models require qualification; full catalog: aigw catalog --all", candidates))
	}
}

func catalogModelDisplay(model catalogModel) (presentation.State, string) {
	if model.State == catalogCandidate {
		return presentation.Info, "Candidate; protocol and client capabilities are unqualified"
	}
	routes := make([]string, 0, len(model.Routes))
	for _, route := range model.Routes {
		routes = append(routes, route.ID)
	}
	if model.State == catalogDeprecated {
		return presentation.Warn, "Deprecated Routes: " + strings.Join(routes, ", ")
	}
	return presentation.OK, "Admitted Routes: " + strings.Join(routes, ", ")
}

func renderMissingRoutes(r *presentation.Renderer, routes []catalogRoute) {
	for _, route := range routes {
		r.StatusLine(presentation.Warn, "Route", route.ID)
		r.Detail(route.UpstreamModel + " · missing from this observation; requalification required")
	}
}

func catalogStatusText(status catalogStatus) string {
	switch status {
	case catalogOK:
		return string(status)
	case catalogEndpointUnavailable:
		return "No supported model catalogue endpoint is configured"
	case catalogTokenUnavailable:
		return "Token unavailable"
	case catalogCredentialUnavailable:
		return "Credential backend failed; catalog was not queried"
	case catalogRequestFailed:
		return "Catalog request failed; configuration was not changed"
	default:
		return string(status)
	}
}

func renderer(deps Dependencies) *presentation.Renderer {
	out := deps.Out
	if deps.RenderOut != nil {
		out = deps.RenderOut
	}
	return presentation.NewWithWidth(out, deps.Color, deps.Width)
}

func modelTitle(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func protocolTitle(protocol domain.EndpointProtocol) string {
	return domain.ProtocolLabel(protocol)
}
