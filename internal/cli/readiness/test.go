package readiness

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
	"github.com/spf13/cobra"
)

func NewTestCommand(runtime invocation.Context) *cobra.Command {
	var client, profileName string
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test current service endpoints",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			if profileName != "" && client == "" {
				client, err = cfg.ClientForProfile(profileName)
				if err != nil {
					return err
				}
			}
			clients := configuration.AdmittedClientSpecs()
			if client != "" {
				spec, ok := configuration.ClientSpecFor(client)
				if !ok {
					return fmt.Errorf("--for must be %s; run `aigw test --help`", configuration.AdmittedClientUsage())
				}
				clients = []configuration.ClientSpec{spec}
			}
			if len(cfg.Profiles) == 0 {
				return invocation.Problem(runtime, "Not configured", "No service profiles have been created.", "No client endpoint is available to test.", "aigw setup", fmt.Errorf("not configured"))
			}
			results := make([]endpointTestResult, 0, len(clients))
			for _, spec := range clients {
				target := spec.ID
				clientRuntime, err := cfg.ResolveRuntime(target, profileName)
				if err != nil {
					return err
				}
				endpoint := clientRuntime.Endpoint
				testURL := endpoint
				if target == configuration.ClientCodex {
					testURL = CodexModelsEndpoint(endpoint)
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), 12*time.Second)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
				if err != nil {
					cancel()
					return err
				}
				accountName := clientRuntime.AccountID
				token, err := runtime.Secrets.Get(accountName)
				if err != nil {
					cancel()
					instruction, _ := credential.TokenRecovery(runtime.Secrets, accountName)
					return fmt.Errorf("Token for account %q is unavailable: %w; %s", accountName, err, instruction)
				}
				if err := authenticateRequest(req, spec, token); err != nil {
					cancel()
					return err
				}
				resp, err := runtime.HTTP.Do(req)
				if err != nil {
					cancel()
					return fmt.Errorf("%s endpoint is unreachable: %w", invocation.Title(target), err)
				}
				if _, err := io.Copy(io.Discard, resp.Body); err != nil {
					_ = resp.Body.Close()
					cancel()
					return fmt.Errorf("read %s endpoint response: %w", invocation.Title(target), err)
				}
				if err := resp.Body.Close(); err != nil {
					cancel()
					return fmt.Errorf("close %s endpoint response: %w", invocation.Title(target), err)
				}
				cancel()
				if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
					instruction, _ := credential.TokenRecovery(runtime.Secrets, accountName)
					return fmt.Errorf("%s authentication was rejected (HTTP %d); %s", invocation.Title(target), resp.StatusCode, instruction)
				}
				detail := ""
				if resp.StatusCode == http.StatusNotFound && target == configuration.ClientClaude {
					detail = "Service is reachable; the base URL does not provide a GET probe"
				} else if resp.StatusCode < 200 || resp.StatusCode >= 400 {
					return fmt.Errorf("%s endpoint returned HTTP %d", invocation.Title(target), resp.StatusCode)
				}
				results = append(results, endpointTestResult{client: target, profileID: clientRuntime.ProfileID, status: resp.StatusCode, detail: detail})
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
	cmd.Flags().StringVar(&client, "for", "", "Test only Claude or Codex")
	cmd.Flags().StringVar(&profileName, "profile", "", "Test a specified profile without changing routes; infer its declared client when --for is omitted")
	return cmd
}

func authenticateRequest(request *http.Request, spec configuration.ClientSpec, token string) error {
	switch spec.EndpointProtocol {
	case configuration.ProtocolAnthropic:
		request.Header.Set("X-Api-Key", token)
	case configuration.ProtocolOpenAIResponses:
		request.Header.Set("Authorization", "Bearer "+token)
	default:
		return fmt.Errorf("%s endpoint protocol %q is unsupported", spec.ID, spec.EndpointProtocol)
	}
	return nil
}
