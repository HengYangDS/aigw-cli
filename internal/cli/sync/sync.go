// Package sync implements `aigw sync`, which resynchronizes the enabled
// clients' configuration with the selected Routes. It never changes client
// conversation history or model metadata.
package sync

import (
	"fmt"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/app"
	clientactivation "aigw-cli/internal/app/activation"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/app/synchronization"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// schemaVersion is the version of `aigw sync`'s JSON document.
const schemaVersion = 2

// syncResult is `aigw sync`'s JSON document: the Client Bindings, any
// selection adopted from a client, and in a preview the client files that it
// would write and the credential reader that it would install. Its next step
// is a command that a script can run, or guidance.
type syncResult struct {
	DryRun               bool                          `json:"dry_run,omitempty"`
	Selections           map[string]string             `json:"selections"`
	AdoptedSelections    map[string]string             `json:"adopted_selections,omitempty"`
	Projections          []invocation.Projection       `json:"projections,omitempty"`
	CredentialEntrypoint *app.CredentialEntrypointPlan `json:"credential_entrypoint,omitempty"`
	EnabledClients       int                           `json:"enabled_clients"`
	State                string                        `json:"state,omitempty"`
	NextAction           string                        `json:"next_action,omitempty"`
	NextDetail           string                        `json:"next_detail,omitempty"`
	// next is the follow-up before it is split, and plans the preview's
	// targets as human output names them.
	next  string
	plans []adapter.ProjectionPlan
}

// NewCommand constructs the command that projects current bindings to discovered clients.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var dryRun bool
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Resynchronize client configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			before, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			operation := runtime
			operation.Secrets = secret.ObserveAvailability(runtime.Secrets)
			synchronizer := operation.Synchronizer()
			adoptedConfig, adopted, err := synchronizer.AdoptNativeSelections(before)
			if err != nil {
				return fmt.Errorf("sync synchronization preflight failed; configuration and client files were unchanged: %w", err)
			}
			after, _, err := synchronizer.DesiredSyncConfiguration(adoptedConfig)
			if err != nil {
				return invocation.Problem(
					runtime,
					"Synchronization prerequisites are unavailable",
					"AIGW could not determine which selected Client Bindings can be projected with the currently available clients and credentials.",
					"Configuration and client projections remain unchanged.",
					"aigw doctor",
					err,
				)
			}
			activation := clientactivation.AssessActivation(after, operation.Secrets)
			result := syncResult{DryRun: dryRun, Selections: map[string]string{}, AdoptedSelections: adopted, next: activation.NextActionFor(nil)}
			result.EnabledClients = activation.EnabledClients
			result.State = string(activation.State)
			for _, client := range runtime.Clients.Admission().IDs() {
				if route := after.SelectedRoute(client); route != "" {
					result.Selections[client] = route
				}
			}
			if dryRun {
				if err := prepareSyncPreview(synchronizer, before, after, &result); err != nil {
					return err
				}
			} else if err := synchronizer.CommitProjection(cmd.Context(), before, after, "sync"); err != nil {
				return err
			}
			if jsonMode {
				result.Projections = invocation.Projections(runtime, result.plans)
				result.NextAction, result.NextDetail = terminal.NextSteps(result.next)
				return terminal.WriteDocument(runtime.Out, schemaVersion, true, result)
			}
			r := invocation.Renderer(runtime)
			if dryRun {
				renderSyncPreview(r, after, result)
			} else {
				r.ProductTitle("Synchronization completed")
			}
			outcome := []terminal.Field{}
			for _, client := range runtime.Clients.Admission().IDs() {
				if route, ok := result.AdoptedSelections[client]; ok {
					outcome = append(outcome, terminal.Field{
						State: terminal.Info, Label: "Adopted selection",
						Value: runtime.Clients.Admission().Label(client) + " now uses " + after.RouteLabel(route) + ", chosen in " + runtime.Clients.Admission().Label(client),
					})
				}
			}
			switch {
			case result.EnabledClients == 0:
				// The activation decision names the continuation for the current backend.
				outcome = append(outcome, terminal.Field{
					State: terminal.Info, Label: "Client activation", Value: "No client is enabled",
					Details: []string{"Authentication was unchanged"},
				})
			case activation.State == domainreadiness.Deferred:
				outcome = append(outcome, terminal.Field{State: terminal.Info, Label: "Client activation", Value: "Selected client projection is deferred"})
			}
			if len(outcome) > 0 && dryRun {
				r.Section("Activation")
			}
			r.Rows(outcome...)
			switch {
			case dryRun:
				r.Success("Preview did not write configuration, state files, authentication, or conversations")
			case result.EnabledClients != 0 && activation.State != domainreadiness.Deferred:
				r.Success("Client configuration is aligned; authentication was unchanged")
			}
			r.Next(result.next)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the synchronization plan without writing configuration")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the synchronization preview or result as JSON")
	return cmd
}

func prepareSyncPreview(synchronizer synchronization.Synchronizer, before, after domain.Config, result *syncResult) error {
	plans, err := synchronizer.Plan(before, after)
	if err != nil {
		return err
	}
	entrypoint, err := app.PlanCredentialEntrypoint(synchronizer, after)
	if err != nil {
		return err
	}
	result.plans = plans
	result.CredentialEntrypoint = entrypoint
	if entrypoint != nil && result.next == "aigw check" {
		result.next = "aigw sync"
	}
	for _, plan := range plans {
		if plan.ChangesState && result.next == "aigw check" {
			result.next = "aigw sync"
			break
		}
	}
	return nil
}

func renderSyncPreview(r *terminal.Renderer, after domain.Config, result syncResult) {
	r.ProductTitle("Synchronization preview")
	if result.CredentialEntrypoint != nil {
		// The entrypoint path is too long to share the target column.
		r.Section("Credential entrypoint")
		r.Text("Install " + result.CredentialEntrypoint.Path)
	}
	r.Section("Client Bindings")
	bindings := make([]terminal.Field, 0, len(after.Admission().IDs()))
	for _, client := range after.Admission().IDs() {
		route := after.SelectedRoute(client)
		if route == "" {
			route = "Not selected"
		}
		bindings = append(bindings, terminal.Field{Label: after.Admission().Label(client), Value: route})
	}
	r.Rows(bindings...)
	r.Section("Projection targets")
	targets := make([]terminal.Field, 0, len(result.plans)+1)
	switch {
	case len(result.plans) == 0 && result.EnabledClients != 0 && result.State == string(domainreadiness.Deferred):
		targets = append(targets, terminal.Field{State: terminal.Info, Label: "Projection", Value: "No selected client can be projected yet"})
	case len(result.plans) == 0:
		targets = append(targets, terminal.Field{State: terminal.OK, Label: "Projection", Value: "No client configuration needs changing"})
	}
	for _, plan := range result.plans {
		targets = append(targets, terminal.Field{Label: plan.Target, Value: plan.Action})
	}
	r.Rows(targets...)
}
