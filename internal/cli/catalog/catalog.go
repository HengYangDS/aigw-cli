// Package catalog owns provider-neutral model discovery and presentation.
package catalog

import (
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/secrets"

	"github.com/spf13/cobra"
)

// HTTPDoer executes catalog requests.
type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

// Dependencies are the capabilities required by model commands.
type Dependencies struct {
	Config    configuration.Store
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
	Protocol configuration.EndpointProtocol
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
			r.Detail("Catalog membership does not prove protocol capabilities, inference, or client readiness.")
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

func modelRows(cfg configuration.Config, catalog catalogOutput) []modelRow {
	observations := make(map[string]catalogObservation, len(catalog.Observations))
	for _, observation := range catalog.Observations {
		observations[observationKey(observation.Account, observation.Source.Protocol)] = observation
	}
	rows := []modelRow{}
	for _, routeID := range cfg.RouteIDs() {
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
	return rows
}

// NewCatalogCommand constructs authenticated catalog discovery.
func NewCatalogCommand(deps Dependencies) *cobra.Command {
	var jsonMode, all bool
	cmd := &cobra.Command{Use: "catalog", Short: "Discover authenticated model catalogs for each account (compact summary by default)", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if jsonMode && all {
			return fmt.Errorf("--all cannot be used with --json; JSON already includes the complete catalog")
		}
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
		r.Detail("Each observation proves catalogue membership only for its named protocol source.")
		for _, observation := range result.Observations {
			title := observation.Label + " · " + observation.Account
			if observation.Source.Protocol != "" {
				title += " · " + protocolTitle(observation.Source.Protocol)
			}
			r.Section(title)
			if observation.Status != catalogOK {
				r.Status(presentation.Warn, "Catalog", catalogStatusText(observation.Status))
				continue
			}
			r.Row("Observation", observation.ObservationID)
			r.Row("Source", observation.Source.Endpoint)
			if len(observation.Models) == 0 {
				r.Status(presentation.Info, "model", "Upstream returned an empty catalog")
			} else {
				renderCatalogObservation(r, observation, all)
			}
			renderMissingRoutes(r, observation.MissingRoutes)
		}
		r.Next("aigw route add <route> --account <account> --for <" + strings.Join(configuration.AdmittedClientIDs(), "|") + "> --model <model>")
		return r.Err()
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	cmd.Flags().BoolVar(&all, "all", false, "Show the complete model catalog")
	return cmd
}

func renderCatalogObservation(r *presentation.Renderer, observation catalogObservation, all bool) {
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
	r.Row("Models", fmt.Sprintf("%d observed", len(observation.Models)))
	r.Row("Admitted", fmt.Sprintf("%d admitted", admitted))
	if deprecated > 0 {
		r.Row("Deprecated", fmt.Sprintf("%d deprecated", deprecated))
	}
	if all {
		visible = observation.Models
	}
	for _, model := range visible {
		state, detail := catalogModelDisplay(model)
		r.Status(state, "model", model.ID)
		r.Detail(detail)
	}
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

func protocolTitle(protocol configuration.EndpointProtocol) string {
	switch protocol {
	case configuration.ProtocolAnthropic:
		return "Anthropic Messages"
	case configuration.ProtocolOpenAIChatCompletions:
		return "OpenAI Chat Completions"
	case configuration.ProtocolOpenAIResponses:
		return "OpenAI Responses"
	default:
		return string(protocol)
	}
}
