// Package verify implements `aigw verify`, which verifies a Route through a
// real native client and may use provider quota. Only a complete success of
// --for all writes the secret-free verified checkpoint.
package verify

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// schemaVersion is the version of `aigw verify`'s JSON document.
const schemaVersion = 1

// verifyDocument is `aigw verify`'s JSON document: each selected client's
// session and whether the full verification checkpoint was written.
type verifyDocument struct {
	Clients           []verifiedClient `json:"clients"`
	CheckpointWritten bool             `json:"checkpoint_written"`
	NextAction        string           `json:"next_action,omitempty"`
	NextDetail        string           `json:"next_detail,omitempty"`
}

// verifiedClient is one client's verification session: completed, failed or
// not_run after an earlier client failed.
type verifiedClient struct {
	Client        string `json:"client"`
	Route         string `json:"route"`
	State         string `json:"state"`
	Scope         string `json:"scope,omitempty"`
	ClientVersion string `json:"client_version,omitempty"`
	ClientSHA256  string `json:"client_sha256,omitempty"`
	Detail        string `json:"detail,omitempty"`
}

// NewCommand constructs the real-client verification command.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var client, routeName string
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify a Route through a real native client (may use quota)",
		Long: "Run a bounded session through one real native client, or all enabled\n" +
			"clients with --for all. A session may make multiple provider requests\n" +
			"and may use quota. --for all writes a configuration checkpoint only\n" +
			"after every client succeeds. No client binding or native settings\n" +
			"are changed.",
		Args: cobra.MatchAll(cobra.NoArgs, func(_ *cobra.Command, _ []string) error {
			if client == "" {
				return fmt.Errorf("choose a verification client with --for; run `aigw verify --help`")
			}
			if client != "" && client != "all" && !runtime.Clients.Admission().Admits(client) {
				return fmt.Errorf("--for must be %s; run `aigw verify --help`", runtime.Clients.Admission().Usage("all"))
			}
			if client == "all" && routeName != "" {
				return fmt.Errorf("--route requires one explicit client, not --for all; run `aigw verify --help`")
			}
			return nil
		}),
		RunE: func(cmd *cobra.Command, _ []string) error {
			synchronizer := runtime.Synchronizer()
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			clients := []string{client}
			if client == "all" {
				clients = cfg.EnabledClientIDs()
				if len(clients) == 0 {
					return fmt.Errorf("no enabled clients to verify; run `aigw status`")
				}
			}
			clientRuntimes := make(map[string]domain.Runtime, len(clients))
			for _, target := range clients {
				clientRuntime, err := cfg.ResolveRuntime(target, routeName)
				if err != nil {
					return err
				}
				clientRuntimes[target] = clientRuntime
				if client != "all" {
					continue
				}
				status := runtime.InspectClient(cmd.Context(), cfg, target, clientRuntime)
				if !status.Ready {
					return terminal.ProblemError(
						fmt.Sprintf("Full verification requires a ready %s adapter: %s", runtime.Clients.Admission().Label(target), status.Issue), "",
						"No client was verified and the latest full verification checkpoint is unchanged.",
						status.RepairAction, nil,
					)
				}
			}
			if jsonMode {
				return writeVerification(cmd, runtime, cfg, clients, clientRuntimes, routeName, client == "all")
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Live protocol verification")
			r.Section("Native client verification")
			r.Text("Each selected client runs a bounded verification session and may make multiple provider requests. AIGW does not modify client configuration or restart clients.")
			for _, target := range clients {
				clientRuntime := clientRuntimes[target]
				result, err := synchronizer.Verify(cmd.Context(), cfg, target, clientRuntime, routeName)
				if err != nil {
					return err
				}
				// Each client is a group of one, rendered as its result arrives.
				row := terminal.Field{State: terminal.OK, Label: runtime.Clients.Admission().Label(target), Value: clientRuntime.RouteID + " · Completed"}
				if result.Version != "" || result.SHA256 != "" {
					row.Details = append(row.Details, fmt.Sprintf("%s client: %s · SHA-256 %s", runtime.Clients.Admission().Label(target), result.Version, result.SHA256))
				}
				if result.Scope != "" {
					row.Details = append(row.Details, result.Scope)
				}
				r.Rows(row)
			}
			if client == "all" {
				if err := runtime.Config.SaveVerifiedCheckpoint(cmd.Context(), cfg, clients); err != nil {
					return err
				}
				r.Detail("Updated the latest full verification checkpoint.")
			}
			r.Next("aigw doctor")
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "Client whose selected Route to verify: "+runtime.Clients.Admission().Usage("all")+"; all means enabled clients")
	cmd.Flags().StringVar(&routeName, "route", "", "Verify this Route for the explicit client without changing its binding")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write each client's verification as JSON when every session has ended")
	return cmd
}

// writeVerification runs every selected client's session and writes one
// document when the last has ended. A failed session ends the run: its client
// is failed, later clients are not run, no checkpoint is written, and the
// command exits 1.
func writeVerification(cmd *cobra.Command, runtime invocation.Context, cfg domain.Config, clients []string, runtimes map[string]domain.Runtime, routeName string, full bool) error {
	document := verifyDocument{Clients: make([]verifiedClient, 0, len(clients))}
	var failure error
	for _, target := range clients {
		item := verifiedClient{Client: target, Route: runtimes[target].RouteID, State: "not_run"}
		if failure == nil {
			result, err := runtime.Synchronizer().Verify(cmd.Context(), cfg, target, runtimes[target], routeName)
			if err != nil {
				failure, item.State = err, "failed"
				problem := terminal.ProblemFor(err)
				item.Detail = problem.Title
				document.NextAction, document.NextDetail = terminal.NextSteps(problem.Fix)
			} else {
				item.State, item.Scope, item.ClientVersion, item.ClientSHA256 = "completed", result.Scope, result.Version, result.SHA256
			}
		}
		document.Clients = append(document.Clients, item)
	}
	if failure == nil {
		if full {
			if err := runtime.Config.SaveVerifiedCheckpoint(cmd.Context(), cfg, clients); err != nil {
				return err
			}
			document.CheckpointWritten = true
		}
		document.NextAction = "aigw doctor"
	}
	if err := terminal.WriteDocument(runtime.Out, schemaVersion, failure == nil, document); err != nil {
		return err
	}
	if failure != nil {
		return terminal.Presented(failure)
	}
	return nil
}
