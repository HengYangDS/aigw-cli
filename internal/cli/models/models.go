// Package models implements `aigw models`, which compares the configured model
// IDs with each provider's catalog.
package models

import (
	"aigw-cli/internal/app"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
)

type modelRow struct {
	Route    string
	Account  string
	Protocol domain.EndpointProtocol
	Model    string
	Catalog  string
	// State is Catalog as a JSON value: listed, not_listed, not_observed or
	// the catalog observation's status.
	State string
}

// schemaVersion is the version of `aigw models`'s JSON document.
const schemaVersion = 1

// modelsDocument is `aigw models`'s JSON document: each Route's catalog
// membership.
type modelsDocument struct {
	Routes     []routeModel `json:"routes"`
	NextDetail string       `json:"next_detail"`
}

// routeModel is one Route and protocol's catalog membership.
type routeModel struct {
	Route         string                  `json:"route"`
	Account       string                  `json:"account"`
	Protocol      domain.EndpointProtocol `json:"protocol,omitempty"`
	Model         string                  `json:"model"`
	UpstreamModel string                  `json:"upstream_model"`
	Catalog       string                  `json:"catalog"`
}

// NewCommand constructs the configured-model availability command.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Compare configured model IDs with provider catalogs; does not test inference",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Routes) == 0 {
				return fmt.Errorf("not configured; run `aigw setup`")
			}
			rows := modelRows(cfg, app.DiscoverCatalog(cmd.Context(), runtime.Secrets, runtime.HTTP, cfg))
			if jsonMode {
				document := modelsDocument{Routes: make([]routeModel, 0, len(rows)), NextDetail: "aigw use --for <client> <route>"}
				for _, row := range rows {
					document.Routes = append(document.Routes, routeModel{
						Route: row.Route, Account: row.Account, Protocol: row.Protocol,
						Model: cfg.Routes[row.Route].Model, UpstreamModel: row.Model, Catalog: row.State,
					})
				}
				return terminal.WriteDocument(runtime.Out, schemaVersion, true, document)
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Configured model catalog")
			r.Text("Catalog membership does not prove protocol capabilities, inference, or client readiness.")
			r.Section("Routes")
			for _, row := range rows {
				state := terminal.Warn
				if row.Catalog == "Listed" || row.Catalog == "Not listed" {
					state = terminal.Info
				}
				r.StatusLine(state, "Route", row.Route)
				r.Detail(fmt.Sprintf("%s · %s · %s · account %s", row.Model, domain.ProtocolLabel(row.Protocol), row.Catalog, row.Account))
			}
			r.Next("aigw use")
			return r.Err()
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write each Route's catalog membership as JSON")
	return cmd
}

func modelRows(cfg domain.Config, catalog app.CatalogReport) []modelRow {
	observations := make(map[string]app.CatalogObservation, len(catalog.Observations))
	for _, observation := range catalog.Observations {
		observations[observationKey(observation.Account, observation.Source.Protocol)] = observation
	}
	rows := []modelRow{}
	for _, routeID := range cfg.CatalogRouteIDs() {
		route := cfg.Routes[routeID]
		protocols := route.AdmittedProtocols()
		if len(protocols) == 0 {
			rows = append(rows, modelRow{Route: routeID, Account: route.Account, Model: route.UpstreamModelID(), Catalog: "Catalog not observed", State: "not_observed"})
			continue
		}
		for _, protocol := range protocols {
			membership, state := "Catalog not observed", "not_observed"
			if observation, ok := observations[observationKey(route.Account, protocol)]; ok {
				membership, state = observation.Status.Label(), string(observation.Status)
				if observation.Status == app.CatalogOK {
					membership, state = "Not listed", "not_listed"
					if slices.ContainsFunc(observation.Models, func(model app.CatalogModel) bool { return model.ID == route.UpstreamModelID() }) {
						membership, state = "Listed", "listed"
					}
				}
			}
			rows = append(rows, modelRow{Route: routeID, Account: route.Account, Protocol: protocol, Model: route.UpstreamModelID(), Catalog: membership, State: state})
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

// observationKey identifies the observation of one Account and protocol.
func observationKey(account string, protocol domain.EndpointProtocol) string {
	return account + "\x00" + string(protocol)
}
