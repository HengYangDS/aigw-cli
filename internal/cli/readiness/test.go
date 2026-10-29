package readiness

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"aigw-cli/internal/cli/invocation"
	clientverification "aigw-cli/internal/client/verification"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/secrets"

	"github.com/spf13/cobra"
)

type endpointTestResult struct {
	client  string
	routeID string
	status  int
	detail  string
}

// NewTestCommand constructs the live endpoint verification command.
func NewTestCommand(runtime invocation.Context) *cobra.Command {
	var client, routeName, configPath, tokenFormat string
	var tokenStdin bool
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Probe selected endpoints without model inference",
		Long: "Send a model-free HTTP request to selected endpoints without changing\n" +
			"client bindings. --for and --route can target one Route; --token-stdin\n" +
			"uses a one-time Token without storing it. An endpoint response does not\n" +
			"prove model inference or real-client behavior.",
		Args: cobra.MatchAll(cobra.NoArgs, validateEndpointTestSelection(runtime.Clients.Admission())),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if configPath != "" {
				runtime.Config = configuration.NewStore(configPath, runtime.Config.Admission())
			}
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Routes) == 0 {
				return invocation.Problem(runtime, "Not configured", "No Routes have been created.", "No client endpoint is available to test.", "aigw setup", fmt.Errorf("not configured"))
			}
			clients, err := endpointTestClients(cfg, client, routeName)
			if err != nil {
				return err
			}
			if len(clients) == 0 {
				return invocation.Problem(runtime, "No Client Binding is selected", "Routes exist, but no client has a selected Route.", "There is no selected endpoint to test.", "aigw use --for <client> <route>", fmt.Errorf("no Client Binding selected"))
			}
			resolved := make(map[string]configuration.Runtime, len(clients))
			for _, spec := range clients {
				clientRuntime, err := cfg.ResolveRuntime(spec.ID, routeName)
				if err != nil {
					return err
				}
				if !clientRuntime.RequiresAccountToken() {
					return fmt.Errorf(
						"route %q uses client-owned authentication; run `%s` to test it through %s",
						clientRuntime.RouteID,
						clientverification.Command(spec.ID, routeName),
						spec.Label,
					)
				}
				if clientRuntime.CredentialCommand != "" && !tokenStdin {
					return fmt.Errorf("route %q uses an external credential helper; run `%s` or provide an explicit test Token on stdin", clientRuntime.RouteID, clientverification.Command(spec.ID, routeName))
				}
				resolved[spec.ID] = clientRuntime
			}
			results := make([]endpointTestResult, 0, len(clients))
			for _, spec := range clients {
				target := spec.ID
				clientRuntime := resolved[target]
				accountName := clientRuntime.AccountID
				token, instruction, err := endpointTestToken(runtime, accountName, tokenStdin, tokenFormat)
				if err != nil {
					return fmt.Errorf("token for account %q is unavailable: %w; %s", accountName, err, instruction)
				}
				status, err := credential.ProbeStatus(cmd.Context(), runtime.HTTP, runtime.Clients.Admission(), target, clientRuntime.Endpoint, token, clientRuntime.Protocol)
				if err != nil {
					return err
				}
				if status == http.StatusUnauthorized || status == http.StatusForbidden {
					return fmt.Errorf("%s authentication was rejected (HTTP %d); %s", runtime.Clients.Admission().Label(target), status, instruction)
				}
				detail := ""
				if spec, _ := runtime.Clients.Admission().Spec(target); status == http.StatusNotFound && spec.ModelDiscoveryOptional {
					detail = "Endpoint is reachable; model discovery is unavailable and credential acceptance is unverified"
				} else if status < http.StatusOK || status >= http.StatusMultipleChoices {
					return fmt.Errorf("%s endpoint returned HTTP %d", runtime.Clients.Admission().Label(target), status)
				}
				results = append(results, endpointTestResult{client: target, routeID: clientRuntime.RouteID, status: status, detail: detail})
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Connectivity test")
			r.Text("Endpoint HTTP observation, not model inference or native-client verification.")
			r.Section("Endpoints")
			rows := make([]presentation.Field, 0, len(results))
			for _, result := range results {
				value := fmt.Sprintf("%s · HTTP %d", result.routeID, result.status)
				if result.detail != "" {
					value += " · " + result.detail
				}
				rows = append(rows, presentation.Field{State: presentation.OK, Label: runtime.Clients.Admission().Label(result.client), Value: value})
			}
			r.Rows(rows...)
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "Client whose selected Route to test: "+runtime.Clients.Admission().Usage())
	cmd.Flags().StringVar(&routeName, "route", "", "Test this Route for the explicit client without changing its binding")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read one bounded Token through EOF for this request only; never store it")
	cmd.Flags().StringVar(&tokenFormat, "token-format", "raw", "Explicit stdin encoding: raw or go-keyring-base64")
	cmd.Flags().StringVar(&configPath, "config", "", "Read an absolute configuration file for a one-time stdin Token test")
	return cmd
}

func endpointTestToken(runtime invocation.Context, account string, stdinMode bool, format string) (string, string, error) {
	if stdinMode {
		token, err := invocation.ReadToken(runtime, true, false)
		if err == nil && format == "go-keyring-base64" {
			token, err = secrets.DecodeKeyringBase64(token)
			if err == nil {
				err = invocation.ValidateToken(token)
			}
		}
		return token, "supply an authorized Token for this endpoint", err
	}
	instruction, _ := credential.TokenRecovery(runtime.Secrets, account)
	token, err := runtime.Secrets.Get(account)
	return token, instruction, err
}

func endpointTestClients(cfg configuration.Config, client, routeName string) ([]configuration.ClientSpec, error) {
	if client != "" {
		spec, ok := cfg.Admission().Spec(client)
		if !ok {
			return nil, fmt.Errorf("--for must be %s; run `aigw test --help`", cfg.Admission().Usage())
		}
		return []configuration.ClientSpec{spec}, nil
	}
	selected := make([]configuration.ClientSpec, 0, len(cfg.Clients))
	for _, spec := range cfg.Admission().Specs() {
		if cfg.SelectedRoute(spec.ID) != "" {
			selected = append(selected, spec)
		}
	}
	return selected, nil
}

// validateEndpointTestSelection refuses an incomplete or inconsistent endpoint
// test selection; a client must be one that admission names.
func validateEndpointTestSelection(admission configuration.Admission) cobra.PositionalArgs {
	return func(cmd *cobra.Command, _ []string) error {
		for _, name := range []string{"for", "route", "config", "token-format"} {
			flag := cmd.Flags().Lookup(name)
			if flag.Changed && strings.TrimSpace(flag.Value.String()) == "" {
				return fmt.Errorf("--%s requires a non-empty value; run `aigw test --help`", name)
			}
		}
		client := cmd.Flags().Lookup("for").Value.String()
		if client != "" && !admission.Admits(client) {
			return fmt.Errorf("--for must be %s; run `aigw test --help`", admission.Usage())
		}
		routeName := cmd.Flags().Lookup("route").Value.String()
		if routeName != "" && client == "" {
			return fmt.Errorf("--route requires one explicit --for client; run `aigw test --help`")
		}
		stdinMode, err := cmd.Flags().GetBool("token-stdin")
		if err != nil {
			return err
		}
		if stdinMode && client == "" {
			return fmt.Errorf("--token-stdin requires one explicit --for client")
		}
		format := cmd.Flags().Lookup("token-format")
		if format.Value.String() != "raw" && format.Value.String() != "go-keyring-base64" {
			return fmt.Errorf("--token-format must be raw or go-keyring-base64")
		}
		if format.Changed && !stdinMode {
			return fmt.Errorf("--token-format requires --token-stdin")
		}
		configPath := cmd.Flags().Lookup("config").Value.String()
		if configPath != "" && (!stdinMode || !filepath.IsAbs(configPath)) {
			return fmt.Errorf("--config requires an absolute path and --token-stdin")
		}
		return nil
	}
}
