package readiness

import (
	"fmt"
	"net/http"
	"strings"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"

	"github.com/spf13/cobra"
)

type endpointTestResult struct {
	client    string
	profileID string
	status    int
	detail    string
}

// NewTestCommand constructs the live endpoint verification command.
func NewTestCommand(runtime invocation.Context) *cobra.Command {
	var client, profileName string
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test selected service endpoints",
		Args:  cobra.MatchAll(cobra.NoArgs, validateEndpointTestSelection),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Profiles) == 0 {
				return invocation.Problem(runtime, "Not configured", "No service profiles have been created.", "No client endpoint is available to test.", "aigw setup", fmt.Errorf("not configured"))
			}
			clients, err := endpointTestClients(cfg, client, profileName)
			if err != nil {
				return err
			}
			if len(clients) == 0 {
				return invocation.Problem(runtime, "No Route is selected", "Profiles exist, but no client has an active Route.", "There is no selected service endpoint to test.", "aigw use <profile>", fmt.Errorf("no route selected"))
			}
			resolved := make(map[string]configuration.Runtime, len(clients))
			for _, spec := range clients {
				clientRuntime, err := cfg.ResolveRuntime(spec.ID, profileName)
				if err != nil {
					return err
				}
				if !clientRuntime.RequiresAccountToken() {
					return fmt.Errorf(
						"profile %q uses client-owned authentication; run `aigw verify --for %s` to test it through %s",
						clientRuntime.ProfileID,
						spec.ID,
						spec.Label,
					)
				}
				resolved[spec.ID] = clientRuntime
			}
			results := make([]endpointTestResult, 0, len(clients))
			for _, spec := range clients {
				target := spec.ID
				clientRuntime := resolved[target]
				accountName := clientRuntime.AccountID
				token, err := runtime.Secrets.Get(accountName)
				if err != nil {
					instruction, _ := credential.TokenRecovery(runtime.Secrets, accountName)
					return fmt.Errorf("token for account %q is unavailable: %w; %s", accountName, err, instruction)
				}
				status, err := credential.ProbeStatus(cmd.Context(), runtime.HTTP, target, clientRuntime.Endpoint, token)
				if err != nil {
					return err
				}
				if status == http.StatusUnauthorized || status == http.StatusForbidden {
					instruction, _ := credential.TokenRecovery(runtime.Secrets, accountName)
					return fmt.Errorf("%s authentication was rejected (HTTP %d); %s", invocation.Title(target), status, instruction)
				}
				detail := ""
				if status == http.StatusNotFound && target == configuration.ClientClaude {
					detail = "Service is reachable; model discovery is unavailable and credential acceptance is unverified"
				} else if status < http.StatusOK || status >= http.StatusMultipleChoices {
					return fmt.Errorf("%s endpoint returned HTTP %d", invocation.Title(target), status)
				}
				results = append(results, endpointTestResult{client: target, profileID: clientRuntime.ProfileID, status: status, detail: detail})
			}
			r := Renderer(runtime)
			r.ProductTitle("Connectivity test")
			r.Section("Endpoints")
			for _, result := range results {
				value := fmt.Sprintf("%s · HTTP %d", result.profileID, result.status)
				if result.detail != "" {
					value += " · " + result.detail
				}
				r.Status(presentation.OK, invocation.Title(result.client), value)
			}
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "Test the selected Route for "+configuration.AdmittedClientLabelUsage())
	cmd.Flags().StringVar(&profileName, "profile", "", "Test one Profile using its declared client without changing Routes")
	cmd.MarkFlagsMutuallyExclusive("for", "profile")
	return cmd
}

func endpointTestClients(cfg configuration.Config, client, profileName string) ([]configuration.ClientSpec, error) {
	if profileName != "" {
		profileClient, err := cfg.ClientForProfile(profileName)
		if err != nil {
			return nil, err
		}
		client = profileClient
	}
	if client != "" {
		spec, ok := configuration.ClientSpecFor(client)
		if !ok {
			return nil, fmt.Errorf("--for must be %s; run `aigw test --help`", configuration.AdmittedClientUsage())
		}
		return []configuration.ClientSpec{spec}, nil
	}
	selected := make([]configuration.ClientSpec, 0, len(cfg.Routes))
	for _, spec := range configuration.AdmittedClientSpecs() {
		if cfg.Routes[spec.ID] != "" {
			selected = append(selected, spec)
		}
	}
	return selected, nil
}

func validateEndpointTestSelection(cmd *cobra.Command, _ []string) error {
	for _, name := range []string{"for", "profile"} {
		flag := cmd.Flags().Lookup(name)
		if flag.Changed && strings.TrimSpace(flag.Value.String()) == "" {
			return fmt.Errorf("--%s requires a non-empty value; run `aigw test --help`", name)
		}
	}
	client := cmd.Flags().Lookup("for").Value.String()
	if client != "" && !configuration.IsAdmittedClient(client) {
		return fmt.Errorf("--for must be %s; run `aigw test --help`", configuration.AdmittedClientUsage())
	}
	return nil
}
