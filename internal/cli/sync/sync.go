// Package sync implements `aigw sync`, which makes the enabled clients' files
// match the configuration: it discovers the clients, adopts a selection that
// a client made itself, and writes each client's files for its model. It never
// changes client conversation history or model metadata.
package sync

import (
	"fmt"

	clientactivation "aigw-cli/internal/app/activation"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// schemaVersion is the version of `aigw sync`'s JSON document, which also
// replaces `aigw repair`'s: both were at 2.
const schemaVersion = 3

// syncResult is `aigw sync`'s JSON document: each client's model as
// provider/model, any selection adopted from a client, and in a preview the
// client files that it would write and the credential reader that it would
// install. Its next step is a command that a script can run, or guidance.
type syncResult struct {
	DryRun            bool              `json:"dry_run,omitempty"`
	Selections        map[string]string `json:"selections"`
	AdoptedSelections map[string]string `json:"adopted_selections,omitempty"`
	invocation.Preview
	EnabledClients int    `json:"enabled_clients"`
	State          string `json:"state,omitempty"`
	NextAction     string `json:"next_action,omitempty"`
	NextDetail     string `json:"next_detail,omitempty"`
	// next is the follow-up before it is split.
	next string
}

// NewCommand constructs the command that writes each enabled client's files
// for the model that it uses.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var dryRun bool
	var jsonMode bool
	cmd := &cobra.Command{
		Use:     "sync",
		Short:   "Make client files match the configuration",
		Example: "aigw sync\naigw sync --dry-run\naigw sync --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			before, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			// Providers without models synchronize too: no client selects
			// anything, so no client file changes.
			if len(before.Providers) == 0 {
				return terminal.ProblemError("Not configured", "No provider has been added.", "", "aigw setup", fmt.Errorf("not configured"))
			}
			operation := runtime
			operation.Secrets = secret.ObserveAvailability(runtime.Secrets)
			synchronizer := operation.Synchronizer()
			adoptedConfig, adopted, err := synchronizer.AdoptNativeSelections(before)
			if err != nil {
				return fmt.Errorf("sync synchronization preflight failed; configuration and client files were unchanged: %w", err)
			}
			after, _, err := synchronizer.DesiredSyncConfiguration(adoptedConfig)
			// A blocked Token folder states its own fix, which the command's
			// presentation renders.
			if _, blocked := platform.BlockedPath(err); blocked {
				return err
			}
			if err != nil {
				return terminal.ProblemError(
					"Synchronization prerequisites are unavailable",
					"AIGW could not determine which clients' files it can write with the clients and credentials available now.",
					"Configuration and client files are unchanged.",
					"aigw check --depth local",
					err,
				)
			}
			activation := clientactivation.AssessActivation(after, operation.Secrets)
			result := syncResult{DryRun: dryRun, Selections: map[string]string{}, AdoptedSelections: adopted, next: activation.NextActionFor(nil)}
			result.EnabledClients = activation.EnabledClients
			result.State = string(activation.State)
			for _, client := range runtime.Clients.Admission().IDs() {
				if key := after.SelectedModel(client); key != "" {
					result.Selections[client] = after.Models[key].Ref().String()
				}
			}
			for client, key := range result.AdoptedSelections {
				result.AdoptedSelections[client] = after.Models[key].Ref().String()
			}
			if dryRun {
				if err := prepareSyncPreview(operation, before, after, &result); err != nil {
					return err
				}
			} else if err := synchronizer.CommitProjection(cmd.Context(), before, after, "sync"); err != nil {
				return err
			}
			if jsonMode {
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
				if model, ok := result.AdoptedSelections[client]; ok {
					outcome = append(outcome, terminal.Field{
						State: terminal.Info, Label: "Adopted selection",
						Value: runtime.Clients.Admission().Label(client) + " now uses " + model + ", chosen in " + runtime.Clients.Admission().Label(client),
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
				outcome = append(outcome, terminal.Field{State: terminal.Info, Label: "Client activation", Value: "The selected client's files wait"})
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

// prepareSyncPreview previews the synchronization as it would be committed,
// refusing what the commit would refuse.
func prepareSyncPreview(runtime invocation.Context, before, after domain.Config, result *syncResult) error {
	preview, err := invocation.PreviewProjection(runtime, before, after)
	if err != nil {
		return err
	}
	result.Preview = preview
	if preview.CredentialEntrypoint != nil && result.next == "aigw check" {
		result.next = "aigw sync"
	}
	for _, plan := range preview.Plans() {
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
	r.Section("Clients")
	models := make([]terminal.Field, 0, len(after.Admission().IDs()))
	for _, client := range after.Admission().IDs() {
		model, ok := result.Selections[client]
		if !ok {
			model = "No model selected"
		}
		models = append(models, terminal.Field{Label: after.Admission().Label(client), Value: model})
	}
	r.Rows(models...)
	r.Section("Client files")
	plans := result.Plans()
	targets := make([]terminal.Field, 0, len(plans)+1)
	switch {
	case len(plans) == 0 && result.EnabledClients != 0 && result.State == string(domainreadiness.Deferred):
		targets = append(targets, terminal.Field{State: terminal.Info, Label: "Client files", Value: "No selected client's files can be written yet"})
	case len(plans) == 0:
		targets = append(targets, terminal.Field{State: terminal.OK, Label: "Client files", Value: "No client file needs changing"})
	}
	for _, plan := range plans {
		targets = append(targets, terminal.Field{Label: plan.Target, Value: plan.Action})
	}
	r.Rows(targets...)
}
