package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/providers"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
)

func newAddCommand(app *App) *cobra.Command {
	var label, openAIURL, anthropicURL string
	var tokenStdin bool
	cmd := &cobra.Command{
		Use:   "add <profile>",
		Short: "Add a service and its token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !domain.ValidProfileName(name) {
				return fmt.Errorf("Invalid service ID %q; use letters, numbers, dots, hyphens, or underscores", name)
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			if _, exists := cfg.Profiles[name]; exists {
				return fmt.Errorf("Profile %q already exists; use `aigw profile edit %s` or `aigw rotate %s`", name, name, name)
			}
			if label == "" {
				label = name
			}
			account := domain.Account{Label: label, Endpoints: domain.Endpoints{
				OpenAIResponses: strings.TrimRight(openAIURL, "/"),
				Anthropic:       strings.TrimRight(anthropicURL, "/"),
			}}
			profile := domain.Profile{Label: label, Account: name}
			token, err := app.readToken(tokenStdin, true)
			if err != nil {
				return err
			}
			cfg.Accounts[name] = account
			cfg.Profiles[name] = profile
			if cfg.Routes.Default == "" {
				cfg.Routes.Default = name
			}
			if err := cfg.Validate(); err != nil {
				return err
			}
			if err := app.Secrets.Set(name, token); err != nil {
				return err
			}
			if err := app.Config.Save(cfg); err != nil {
				_ = app.Secrets.Delete(name)
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "Service added")
			r.Section("Service")
			r.Row("Name", label)
			r.Row("Configuration", name)
			r.Status(presentation.OK, "System secret", "Securely stored")
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "Provider display name")
	cmd.Flags().StringVar(&openAIURL, "openai-url", "", "OpenAI Responses base URL")
	cmd.Flags().StringVar(&anthropicURL, "anthropic-url", "", "Anthropic base URL")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read one token line from standard input")
	return cmd
}

func newUseCommand(app *App) *cobra.Command {
	var client string
	var all bool
	cmd := &cobra.Command{
		Use:   "use <profile>",
		Short: "Switch the active AI service",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && client != "" {
				return fmt.Errorf("--all and --for cannot be used together; run `aigw use --help`")
			}
			if client != "" && !domain.IsAdmittedClient(client) {
				return fmt.Errorf("--for must be claude or codex; run `aigw use --help`")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			before := cloneConfig(cfg)
			name := ""
			if len(args) == 1 {
				name = args[0]
			} else {
				if !app.Interactive {
					return fmt.Errorf("Non-interactive use requires a profile; run `aigw use <profile>`")
				}
				name, err = chooseProfile(app, cfg, "Select the AI service to use: ")
				if err != nil {
					return err
				}
			}
			if _, ok := cfg.Profiles[name]; !ok {
				return fmt.Errorf("Unknown profile %q; run `aigw profile list`", name)
			}
			accountName, providerAccount, err := accountForInput(cfg, name)
			if err != nil {
				return err
			}
			addedToken := false
			if !app.Secrets.Has(accountName) {
				if !app.Interactive {
					return fmt.Errorf("Account %q is missing a token; run `aigw rotate %s`", accountName, accountName)
				}
				token, err := app.Prompt.Secret("Paste " + providerAccount.Label + " token: ")
				if err != nil {
					return err
				}
				providerAccount.ID = accountName
				if err := verifyCredential(context.Background(), app, providerAccount, token); err != nil {
					return fmt.Errorf("Token validation failed: %w", err)
				}
				if err := app.Secrets.Set(accountName, token); err != nil {
					return err
				}
				addedToken = true
			}
			switch {
			case all:
				cfg.Routes.Default = name
				cfg.Routes.Overrides = map[string]string{}
			case client != "":
				cfg.Routes.Overrides[client] = name
			default:
				cfg.Routes.Default = name
			}
			if err := commitConfigAndSync(cmd.Context(), app, before, cfg, "route"); err != nil {
				if addedToken {
					_ = app.Secrets.Delete(accountName)
				}
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "Service switched")
			r.Section("Current selection")
			r.Row("Service", cfg.Profiles[name].Label)
			if purpose := strings.TrimSpace(cfg.Profiles[name].Purpose); purpose != "" {
				r.Row("Purpose", purpose)
			}
			scope := "Default route"
			if client != "" {
				scope = title(client)
			} else if all {
				scope = "All clients"
			}
			r.Row("Scope", scope)
			r.Success("Client configuration synchronized")
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "Set only Claude or Codex")
	cmd.Flags().BoolVar(&all, "all", false, "Set the default route and clear client overrides")
	return cmd
}

func newRotateCommand(app *App) *cobra.Command {
	var tokenStdin bool
	cmd := &cobra.Command{
		Use:   "rotate [account]",
		Short: "Update the current account token",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			name := cfg.Routes.Default
			if len(args) == 1 {
				name = args[0]
			}
			accountName, account, err := accountForInput(cfg, name)
			if err != nil {
				return err
			}
			oldToken, oldErr := app.Secrets.Get(accountName)
			if oldErr != nil && !errors.Is(oldErr, secrets.ErrNotFound) {
				return oldErr
			}
			var token string
			if tokenStdin {
				token, err = app.readToken(true, false)
			} else if app.Interactive {
				token, err = app.Prompt.Secret("Paste " + account.Label + " token: ")
			} else {
				return fmt.Errorf("Token input requires an interactive terminal; pipe one token line to `aigw rotate %s --token-stdin`", accountName)
			}
			if err != nil {
				return err
			}
			account.ID = accountName
			if err := verifyCredential(context.Background(), app, account, token); err != nil {
				return fmt.Errorf("Token validation failed: %w", err)
			}
			if err := app.Secrets.Set(accountName, token); err != nil {
				return err
			}
			syncCodex := codexRouteUsesAccount(cfg, accountName)
			if syncCodex {
				if err := syncCodexProjection(cmd.Context(), app, cfg); err != nil {
					var rollbackErr error
					if errors.Is(oldErr, secrets.ErrNotFound) {
						rollbackErr = app.Secrets.Delete(accountName)
					} else {
						rollbackErr = app.Secrets.Set(accountName, oldToken)
					}
					if rollbackErr == nil {
						rollbackErr = syncCodexProjection(cmd.Context(), app, cfg)
					}
					if rollbackErr != nil {
						return fmt.Errorf("Token synchronization failed: %w; rollback also failed: %v", err, rollbackErr)
					}
					return fmt.Errorf("Token synchronization failed and was rolled back: %w", err)
				}
				if err := bindCodexAuthentication(cmd.Context(), app, cfg); err != nil {
					var rollbackErr error
					if errors.Is(oldErr, secrets.ErrNotFound) {
						rollbackErr = app.Secrets.Delete(accountName)
					} else {
						rollbackErr = app.Secrets.Set(accountName, oldToken)
					}
					if rollbackErr == nil {
						rollbackErr = syncCodexProjection(cmd.Context(), app, cfg)
						if rollbackErr == nil {
							rollbackErr = bindCodexAuthentication(cmd.Context(), app, cfg)
						}
					}
					if rollbackErr != nil {
						return fmt.Errorf("Token authentication synchronization failed: %w; rollback also failed: %v", err, rollbackErr)
					}
					return fmt.Errorf("Token authentication synchronization failed and was rolled back: %w", err)
				}
			}
			r := renderer(app)
			r.Title("AIGW", "Token updated")
			r.Section("Service")
			r.Row("Account", account.Label)
			r.Row("Account", accountName)
			r.Status(presentation.OK, "Token", "Validated and securely stored")
			if syncCodex {
				r.Success("Codex authentication synchronized")
			} else {
				r.Success("Not related to Codex; Codex configuration and authentication were not changed")
			}
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read one token line from standard input")
	return cmd
}

func chooseProfile(app *App, cfg domain.Config, label string) (string, error) {
	names := sortedProfileNames(cfg)
	choices := make([]Choice, 0, len(names))
	for _, name := range names {
		choices = append(choices, Choice{Value: name, Label: profileChoiceLabel(cfg.Profiles[name])})
	}
	return app.Prompt.Select(label, choices)
}

func profileChoiceLabel(profile domain.Profile) string {
	if purpose := strings.TrimSpace(profile.Purpose); purpose != "" {
		return profile.Label + " · " + purpose
	}
	return profile.Label
}

type routeStatus struct {
	Profile          string `json:"profile,omitempty"`
	Inherited        bool   `json:"inherited"`
	SecretAvailable  bool   `json:"secret_available"`
	EndpointReady    bool   `json:"endpoint_ready"`
	Transport        string `json:"transport,omitempty"`
	TransportReady   bool   `json:"transport_ready,omitempty"`
	AdapterReady     bool   `json:"adapter_ready"`
	AdapterIssue     string `json:"adapter_issue,omitempty"`
	NeedsSelection   bool   `json:"needs_selection,omitempty"`
	SuggestedProfile string `json:"suggested_profile,omitempty"`
}

type endpointTestResult struct {
	client    string
	profileID string
	status    int
	detail    string
}

type statusOutput struct {
	ConfigPath string                 `json:"config_path"`
	Default    string                 `json:"default,omitempty"`
	Routes     map[string]routeStatus `json:"routes"`
	Profiles   int                    `json:"profiles"`
}

func newStatusCommand(app *App) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{Use: "status", Short: "Show the active service and the next useful action", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error { return runStatus(cmd, app, jsonMode) }
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return cmd
}

func runStatus(_ *cobra.Command, app *App, jsonMode bool) error {
	cfg, err := app.Config.Load()
	if err != nil {
		return err
	}
	result := statusOutput{ConfigPath: app.Config.Path(), Default: cfg.Routes.Default, Profiles: len(cfg.Profiles), Routes: map[string]routeStatus{}}
	for _, client := range domain.AdmittedClientIDs() {
		runtime, inherited, resolveErr := cfg.ResolveRuntime(client, "")
		if resolveErr != nil {
			suggested := firstProfileForClient(cfg, client)
			result.Routes[client] = routeStatus{Inherited: true, NeedsSelection: suggested != "", SuggestedProfile: suggested}
			continue
		}
		adapterReady, adapterIssue := adapterRouteReady(app, cfg, client, runtime)
		transport := transportStatus(runtime.Endpoint)
		result.Routes[client] = routeStatus{
			Profile:         runtime.ProfileID,
			Inherited:       inherited,
			SecretAvailable: app.Secrets.Has(runtime.AccountID),
			EndpointReady:   runtime.Endpoint != "",
			Transport:       transport.Kind,
			AdapterReady:    adapterReady,
			AdapterIssue:    adapterIssue,
		}
	}
	if jsonMode {
		enc := json.NewEncoder(app.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	if len(cfg.Profiles) == 0 {
		r := renderer(app)
		r.Title("AIGW", "Not configured")
		r.Section("Get started")
		r.Text("Run the guided setup once to add a service, token, and first model profile.")
		r.Next("aigw setup")
		return nil
	}
	r := renderer(app)
	r.Title("AIGW", "Ready view")
	r.Text("The active service, client readiness, and the smallest next action.")
	r.Section("Active service")
	current := cfg.Profiles[result.Default]
	accountName := current.Account
	account := cfg.Accounts[accountName]
	r.Row("Current profile", current.Label)
	r.Row("Configuration", result.Default)
	if purpose := strings.TrimSpace(current.Purpose); purpose != "" {
		r.Row("Purpose", purpose)
	}
	r.Row("Account", accountName)
	if current.ModelFor(domain.ClientCodex) != "" {
		r.Row("Codex model", current.ModelFor(domain.ClientCodex))
	}
	if current.ModelFor(domain.ClientClaude) != "" {
		r.Row("Claude model", current.ModelFor(domain.ClientClaude))
	}
	r.Row("Model profiles", fmt.Sprintf("%d", result.Profiles))
	r.Section("Clients")
	attention := false
	selectionCommand := ""
	for _, client := range domain.AdmittedClientIDs() {
		route := result.Routes[client]
		if route.NeedsSelection {
			state := presentation.Warn
			message := "No " + title(client) + " profile selected"
			if route.SuggestedProfile != "" {
				cmd := "aigw use " + route.SuggestedProfile + " --for " + client
				message += " · " + cmd
				if selectionCommand == "" {
					selectionCommand = cmd
				}
			}
			r.Status(state, title(client), message)
			attention = true
			continue
		}
		mode := "Explicit override"
		if route.Inherited {
			mode = "Inherits default"
		}
		readiness := route.Profile + " · " + mode + " · Ready"
		state := presentation.OK
		if !route.SecretAvailable || !route.EndpointReady || !route.AdapterReady {
			readiness = route.Profile + " · " + mode + " · Action required"
			if route.AdapterIssue != "" {
				readiness = route.Profile + " · " + mode + " · " + route.AdapterIssue
			}
			state = presentation.Warn
			attention = true
		}
		r.Status(state, title(client), readiness)
	}
	for _, client := range domain.AdmittedClientIDs() {
		route := result.Routes[client]
		if route.Transport != "external_loopback" {
			continue
		}
		r.Section("Transport")
		r.Status(presentation.Info, title(client), "External loopback compatibility layer")
		r.Detail("AIGW does not start, stop, or configure it")
		break
	}
	r.Section("Optional diagnostics")
	if account.AccountProbe != nil && providers.Supports(account.AccountProbe.Kind) && app.Accounts.Has(accountName) {
		r.Status(presentation.OK, "Precise balance", "Enabled")
	} else if account.AccountProbe != nil && providers.Supports(account.AccountProbe.Kind) {
		r.Status(presentation.Warn, "Precise balance", "Disabled")
		r.Detail("aigw account connect " + accountName)
	} else if account.AccountProbe != nil {
		r.Status(presentation.Info, "Precise balance", "This version does not provide diagnostics for this provider")
	} else {
		r.Status(presentation.Info, "Precise balance", "Provider does not expose a probe")
	}
	if selectionCommand != "" {
		r.Next(selectionCommand)
	} else if attention {
		r.Next("aigw repair")
	} else {
		r.Next("aigw check")
	}
	return nil
}

type transportState struct {
	Kind string
}

func transportStatus(endpoint string) transportState {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return transportState{}
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "127.0.0.1", "::1", "localhost":
		return transportState{Kind: "external_loopback"}
	default:
		return transportState{}
	}
}

// runRouteList answers the narrow question "which Profile will each client
// use?". Status intentionally remains the operational overview (adapter,
// endpoint, and secret readiness); keeping this view separate prevents a
// routine route inspection from being buried under unrelated diagnostics.
func runRouteList(app *App) error {
	cfg, err := app.Config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 {
		return problem("Not configured", "No service profiles have been created.", "No default route or client override is available to inspect.", "aigw setup", fmt.Errorf("not configured"))
	}
	r := renderer(app)
	r.Title("AIGW", "Current routes")
	r.Section("Default route")
	if profile, ok := cfg.Profiles[cfg.Routes.Default]; ok {
		r.Status(presentation.OK, "Default profile", cfg.Routes.Default)
		r.Detail(profileChoiceLabel(profile))
	} else {
		r.Status(presentation.Fail, "Default profile", "Not found: "+cfg.Routes.Default)
		r.Detail("Run aigw use to select an available profile")
	}
	r.Section("Client")
	nextCommand := ""
	for _, client := range domain.AdmittedClientIDs() {
		runtime, inherited, resolveErr := cfg.ResolveRuntime(client, "")
		if resolveErr != nil {
			state := presentation.Warn
			message := "No " + title(client) + " profile selected"
			if suggested := firstProfileForClient(cfg, client); suggested != "" {
				command := "aigw use " + suggested + " --for " + client
				message += " · " + command
				if nextCommand == "" {
					nextCommand = command
				}
			}
			r.Status(state, title(client), message)
			continue
		}
		profileName := runtime.ProfileID
		mode := "Inherits default"
		if !inherited {
			mode = "Explicit override"
		}
		profile, ok := cfg.Profiles[profileName]
		if !ok {
			r.Status(presentation.Fail, title(client), profileName+" · "+mode+" · Profile does not exist")
			continue
		}
		r.Status(presentation.OK, title(client), profileName+" · "+mode)
		r.Detail(profileChoiceLabel(profile))
	}
	if nextCommand == "" {
		nextCommand = "aigw use <profile> --for <claude|codex>"
	}
	r.Next(nextCommand)
	return nil
}

// adapterRouteReady checks all local conditions that make an enabled adapter
// usable by the selected route. It is deliberately read-only and never starts
// or reloads a client process.
func adapterRouteReady(app *App, cfg domain.Config, client string, runtime domain.Runtime) (bool, string) {
	adapter := cfg.Adapters[client]
	if !adapter.Enabled {
		return false, title(client) + " adapter is disabled"
	}
	if adapter.Executable == "" {
		return false, title(client) + " executable is not configured"
	}
	switch client {
	case domain.ClientClaude:
		ready, err := app.Shims.ClaudeShimReady()
		if err != nil {
			return false, "Cannot read Claude shim"
		}
		if !ready {
			return false, "Claude shim is missing"
		}
		active, err := app.Shims.ClaudeActivationReady()
		if err != nil {
			return false, "Cannot read Claude PATH activation"
		}
		if !active {
			return false, "Claude PATH activation is missing"
		}
	case domain.ClientCodex:
		if len(adapter.Targets) == 0 {
			return false, "Codex configuration target is missing"
		}
		for _, target := range adapter.Targets {
			if err := adapters.ValidateCodexConfig(target, runtime); err != nil {
				return false, "Codex configuration projection drift: " + err.Error()
			}
		}
	}
	return true, ""
}

func codexModelsEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(endpoint, "/models") {
		return endpoint
	}
	return endpoint + "/models"
}

func firstProfileForClient(cfg domain.Config, client string) string {
	for _, name := range sortedProfileNames(cfg) {
		profile := cfg.Profiles[name]
		if profile.Client != "" && profile.Client != client {
			continue
		}
		if profile.ModelFor(client) != "" {
			return name
		}
		if account, ok := cfg.Accounts[profile.Account]; ok {
			if _, err := account.EndpointFor(client); err == nil {
				return name
			}
		}
	}
	return ""
}

func newTestCommand(app *App) *cobra.Command {
	var client, profileName string
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test current service endpoints",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			clients := domain.AdmittedClientIDs()
			if client != "" {
				if !domain.IsAdmittedClient(client) {
					return fmt.Errorf("--for must be claude or codex; run `aigw test --help`")
				}
				clients = []string{client}
			}
			if len(cfg.Profiles) == 0 {
				return problem("Not configured", "No service profiles have been created.", "No client endpoint is available to test.", "aigw setup", fmt.Errorf("not configured"))
			}
			results := make([]endpointTestResult, 0, len(clients))
			for _, target := range clients {
				runtime, _, err := cfg.ResolveRuntime(target, profileName)
				if err != nil {
					return err
				}
				endpoint := runtime.Endpoint
				if endpoint == "" {
					if client == "" {
						continue
					}
					return fmt.Errorf("Profile %q has no %s endpoint", runtime.ProfileID, title(target))
				}
				testURL := endpoint
				if target == domain.ClientCodex {
					testURL = codexModelsEndpoint(endpoint)
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), 12*time.Second)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
				if err != nil {
					cancel()
					return err
				}
				accountName := runtime.AccountID
				token, err := app.Secrets.Get(accountName)
				if err != nil {
					cancel()
					return fmt.Errorf("Token for account %q is unavailable: %w; run `aigw rotate %s`", accountName, err, accountName)
				}
				req.Header.Set("Authorization", "Bearer "+token)
				resp, err := app.HTTP.Do(req)
				cancel()
				if err != nil {
					return fmt.Errorf("%s endpoint is unreachable: %w", title(target), err)
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
					return fmt.Errorf("%s authentication was rejected (HTTP %d); run `aigw rotate %s`", title(target), resp.StatusCode, accountName)
				}
				detail := ""
				if resp.StatusCode == http.StatusNotFound && target == domain.ClientClaude {
					detail = "Service is reachable; the base URL does not provide a GET probe"
				} else if resp.StatusCode < 200 || resp.StatusCode >= 400 {
					return fmt.Errorf("%s endpoint returned HTTP %d", title(target), resp.StatusCode)
				}
				results = append(results, endpointTestResult{client: target, profileID: runtime.ProfileID, status: resp.StatusCode, detail: detail})
			}
			if len(results) == 0 {
				return fmt.Errorf("Resolved configuration has no client endpoint to test")
			}
			r := renderer(app)
			r.Title("AIGW", "Connectivity test")
			r.Section("Endpoints")
			for _, result := range results {
				value := fmt.Sprintf("%s · HTTP %d", result.profileID, result.status)
				if result.detail != "" {
					value += " · " + result.detail
				}
				r.Status(presentation.OK, title(result.client), value)
			}
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "Test only Claude or Codex")
	cmd.Flags().StringVar(&profileName, "profile", "", "Test a specified profile without changing routes")
	return cmd
}

func newVerifyCommand(app *App) *cobra.Command {
	var client, profileName string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Run one minimal live request to verify the model protocol path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clients := []string{}
			switch client {
			case domain.ClientClaude, domain.ClientCodex:
				clients = []string{client}
			case "all":
				if profileName != "" {
					return fmt.Errorf("--profile cannot be used with --for all; run `aigw verify --help`")
				}
				clients = domain.AdmittedClientIDs()
			default:
				return fmt.Errorf("--for must be claude, codex, or all; run `aigw verify --help`")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			if client == "all" {
				if err := validateFullVerificationReadiness(app, cfg); err != nil {
					return err
				}
			}
			r := renderer(app)
			r.Title("AIGW", "Live protocol verification")
			r.Section("Minimal request")
			r.Detail("This makes one minimal model request; it does not modify client configuration or restart clients.")
			for _, target := range clients {
				runtime, _, err := cfg.ResolveRuntime(target, profileName)
				if err != nil {
					return err
				}
				accountName := runtime.AccountID
				token, err := app.Secrets.Get(accountName)
				if err != nil {
					return fmt.Errorf("Token for account %q is unavailable: %w; run `aigw rotate %s`", accountName, err, accountName)
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), protocolVerificationTimeout)
				if target == domain.ClientCodex {
					err = verifyCodexResponse(ctx, app, runtime, token)
				} else {
					err = verifyClaudeInvocation(ctx, app, cfg, runtime, token)
				}
				cancel()
				if err != nil {
					return err
				}
				r.Status(presentation.OK, title(target), runtime.ProfileID+" · Completed")
			}
			if client == "all" {
				if err := app.Config.SaveVerifiedCheckpoint(cfg, clients); err != nil {
					return err
				}
				r.Detail("Updated the latest full verification checkpoint.")
			}
			r.Next("aigw doctor")
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "Verify Claude, Codex, or all clients")
	cmd.Flags().StringVar(&profileName, "profile", "", "Verify a specified profile without changing routes")
	return cmd
}

// protocolVerificationTimeout allows a cold Claude CLI process to initialize
// and complete one bounded upstream request without turning a healthy, slower
// response into an exec.CommandContext SIGKILL.
const protocolVerificationTimeout = time.Minute

const verificationSentinel = "AIGW_OK"
const verificationResponseLimit = 256 * 1024

type verificationResponse struct {
	Status     string `json:"status"`
	OutputText string `json:"output_text"`
	Output     []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

func validateFullVerificationReadiness(app *App, cfg domain.Config) error {
	claude := cfg.Adapters[domain.ClientClaude]
	if !claude.Enabled || claude.Executable == "" {
		return fmt.Errorf("Full verification requires an enabled Claude adapter; run `aigw repair`")
	}
	ready, err := app.Shims.ClaudeShimReady()
	if err != nil {
		return fmt.Errorf("Failed to read Claude launcher: %w", err)
	}
	if !ready {
		return fmt.Errorf("Full verification requires the AIGW-managed Claude launcher; run `aigw repair`")
	}
	codex := cfg.Adapters[domain.ClientCodex]
	if !codex.Enabled || codex.Executable == "" || len(codex.Targets) == 0 {
		return fmt.Errorf("Full verification requires an enabled Codex adapter with at least one configuration target; run `aigw repair`")
	}
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		return fmt.Errorf("Failed to resolve the Codex route required for full verification: %w", err)
	}
	for _, target := range codex.Targets {
		if err := adapters.ValidateCodexConfig(target, runtime); err != nil {
			return fmt.Errorf("Full verification requires a synchronized Codex configuration target %s: %w; run `aigw sync`", target, err)
		}
	}
	return nil
}

func verifyCodexResponse(ctx context.Context, app *App, runtime domain.Runtime, token string) error {
	endpoint := runtime.Endpoint
	model := runtime.Model
	if model == "" {
		return fmt.Errorf("Profile %q has no Codex model", runtime.ProfileID)
	}
	body, err := json.Marshal(map[string]any{
		"model":             model,
		"input":             "Reply with exactly: AIGW_OK",
		"max_output_tokens": 16,
		"store":             false,
	})
	if err != nil {
		return fmt.Errorf("Failed to encode Codex verification request: %w", err)
	}
	requestURL := strings.TrimRight(endpoint, "/")
	if !strings.HasSuffix(requestURL, "/responses") {
		requestURL += "/responses"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("Codex model request failed: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, verificationResponseLimit+1))
	if err != nil {
		return fmt.Errorf("Failed to read Codex verification response: %w", err)
	}
	if len(responseBody) > verificationResponseLimit {
		return fmt.Errorf("Codex verification response exceeds %d bytes", verificationResponseLimit)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("Codex model authentication was rejected (HTTP %d); run `aigw rotate %s`", resp.StatusCode, runtime.AccountID)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Codex model request returned HTTP %d", resp.StatusCode)
	}
	if !hasVerificationSentinel(responseBody) {
		return fmt.Errorf("Codex model response did not return the expected AIGW_OK verification marker")
	}
	return nil
}

func hasVerificationSentinel(data []byte) bool {
	var response verificationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return false
	}
	if response.Status != "" && response.Status != "completed" {
		return false
	}
	if strings.TrimSpace(response.OutputText) == verificationSentinel {
		return true
	}
	for _, output := range response.Output {
		for _, content := range output.Content {
			if (content.Type == "output_text" || content.Type == "text") && strings.TrimSpace(content.Text) == verificationSentinel {
				return true
			}
		}
	}
	return false
}

func verifyClaudeInvocation(ctx context.Context, app *App, cfg domain.Config, runtime domain.Runtime, token string) error {
	adapter := cfg.Adapters[domain.ClientClaude]
	if !adapter.Enabled || adapter.Executable == "" {
		return fmt.Errorf("Claude adapter is disabled; run `aigw repair`")
	}
	ready, err := app.Shims.ClaudeShimReady()
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("Claude launcher is missing; run `aigw repair`")
	}
	if runtime.Model == "" {
		return fmt.Errorf("Profile %q has no Claude model", runtime.ProfileID)
	}
	plan, err := adapters.ClaudePlan(adapter.Executable, []string{"--safe-mode", "--disable-slash-commands", "--no-session-persistence", "--print", "--model", runtime.Model, "Reply with exactly: AIGW_OK"}, os.Environ(), runtime, token)
	if err != nil {
		return err
	}
	// Verification must capture the bounded child output. Interactive Claude
	// launches still replace AIGW through the normal adapter path.
	plan.Replace = false
	runner, ok := app.Runner.(CaptureRunner)
	if !ok {
		return fmt.Errorf("Claude verification runner is unavailable")
	}
	output, err := runner.RunCapture(ctx, plan)
	if err != nil {
		return fmt.Errorf("Claude minimal verification request failed: %w", err)
	}
	if strings.TrimSpace(string(output)) != verificationSentinel {
		return fmt.Errorf("Claude model response did not return the expected AIGW_OK verification marker")
	}
	return nil
}

func newSyncCommand(app *App) *cobra.Command {
	var dryRun bool
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Resynchronize client configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			if dryRun {
				plans, err := planCodexProjection(cfg)
				if err != nil {
					return err
				}
				if jsonMode {
					enc := json.NewEncoder(app.Out)
					enc.SetIndent("", "  ")
					return enc.Encode(map[string]any{"dry_run": true, "targets": plans})
				}
				r := renderer(app)
				r.Title("AIGW", "Synchronization preview")
				if len(plans) == 0 {
					r.Status(presentation.OK, "Codex", "Adapter is disabled; no configuration projection needs changing")
				} else {
					for _, plan := range plans {
						r.Row(plan.Target, plan.Action)
					}
				}
				r.Success("Preview did not write configuration, state files, authentication, or conversations")
				r.Next("aigw sync")
				return nil
			}
			if err := syncCodexProjection(cmd.Context(), app, cfg); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "Synchronization completed")
			r.Success("Client configuration is aligned; authentication was unchanged")
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the synchronization plan without writing configuration")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write synchronization preview as JSON")
	return cmd
}

func newRollbackCommand(app *App) *cobra.Command {
	var lastChange bool
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Roll back to the latest fully verified configuration or the previous configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			current, err := app.Config.Load()
			if err != nil {
				return err
			}
			restored := domain.Config{}
			source := ""
			if !lastChange {
				checkpoint, checkpointErr := app.Config.LoadVerifiedCheckpoint()
				if checkpointErr == nil {
					restored = checkpoint.Config
					source = "Latest fully verified configuration"
				} else if !errors.Is(checkpointErr, os.ErrNotExist) {
					return checkpointErr
				}
			}
			if source == "" {
				restored, err = app.Config.LoadBackup()
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						return fmt.Errorf("No fully verified checkpoint or previous configuration backup is available")
					}
					return err
				}
				source = "Previous configuration"
			}
			if err := commitConfigAndSync(cmd.Context(), app, current, restored, "rollback"); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "Rolled back safely")
			r.Section("Restore source")
			r.Row("Configuration", source)
			r.Success("Routes and client projections were restored; clients were not restarted.")
			r.Next("aigw doctor")
			return nil
		},
	}
	cmd.Flags().BoolVar(&lastChange, "last-change", false, "Restore only the immediately previous configuration backup")
	return cmd
}

const codexAuthenticationTimeout = 20 * time.Second

// syncCodexProjection is deliberately Codex-only. Claude resolves the current
// Route inside its own process-bound shim and has no persistent projection to
// rewrite.
func planCodexProjection(cfg domain.Config) ([]adapters.CodexProjectionPlan, error) {
	adapter := cfg.Adapters[domain.ClientCodex]
	if !adapter.Enabled {
		return nil, nil
	}
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		return nil, err
	}
	return adapters.PlanCodexConfigs(adapter.Targets, runtime)
}

func syncCodexProjection(_ context.Context, app *App, cfg domain.Config) error {
	adapter := cfg.Adapters[domain.ClientCodex]
	if !adapter.Enabled {
		return nil
	}
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		return err
	}
	return adapters.SyncCodexConfigs(adapter.Targets, runtime)
}

// bindCodexAuthentication updates Codex's native credential store. It is
// intentionally separate from config sync so a model-only switch cannot
// start a second Codex process or disturb a running desktop session.
func bindCodexAuthentication(ctx context.Context, app *App, cfg domain.Config) error {
	adapter := cfg.Adapters[domain.ClientCodex]
	if !adapter.Enabled {
		return nil
	}
	if adapter.Executable == "" || app.Runner == nil {
		return fmt.Errorf("Codex authentication requires an enabled adapter executable")
	}
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		return err
	}
	accountName := runtime.AccountID
	token, err := app.Secrets.Get(accountName)
	if err != nil {
		return fmt.Errorf("Token for the Codex route is unavailable: %w", err)
	}
	for _, target := range adapter.Targets {
		plan, err := adapters.CodexLoginPlan(adapter.Executable, filepath.Dir(target), token)
		if err != nil {
			return err
		}
		targetCtx, cancel := context.WithTimeout(ctx, codexAuthenticationTimeout)
		err = app.Runner.Run(targetCtx, plan)
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}

func codexRouteAccount(cfg domain.Config) (string, bool) {
	if !cfg.Adapters[domain.ClientCodex].Enabled {
		return "", false
	}
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		return "", false
	}
	return runtime.AccountID, runtime.AccountID != ""
}

func codexRouteUsesAccount(cfg domain.Config, accountName string) bool {
	activeAccount, ok := codexRouteAccount(cfg)
	return ok && activeAccount == accountName
}

func codexAuthenticationChanged(before, after domain.Config) bool {
	beforeAccount, beforeOK := codexRouteAccount(before)
	afterAccount, afterOK := codexRouteAccount(after)
	return afterOK && (!beforeOK || beforeAccount != afterAccount)
}

func codexProjectionChanged(before, after domain.Config) bool {
	beforeAdapter := before.Adapters[domain.ClientCodex]
	afterAdapter := after.Adapters[domain.ClientCodex]
	if !afterAdapter.Enabled {
		return false
	}
	if !beforeAdapter.Enabled || !slices.Equal(beforeAdapter.Targets, afterAdapter.Targets) {
		return true
	}
	beforeRuntime, _, beforeErr := before.ResolveRuntime(domain.ClientCodex, "")
	afterRuntime, _, afterErr := after.ResolveRuntime(domain.ClientCodex, "")
	if beforeErr != nil || afterErr != nil {
		return true
	}
	return beforeRuntime.ProfileID != afterRuntime.ProfileID ||
		beforeRuntime.ProfileLabel != afterRuntime.ProfileLabel ||
		beforeRuntime.Endpoint != afterRuntime.Endpoint ||
		beforeRuntime.Model != afterRuntime.Model
}

func cloneConfig(cfg domain.Config) domain.Config {
	clone := cfg
	clone.Accounts = make(map[string]domain.Account, len(cfg.Accounts))
	for name, account := range cfg.Accounts {
		clone.Accounts[name] = account
	}
	clone.Profiles = make(map[string]domain.Profile, len(cfg.Profiles))
	for name, profile := range cfg.Profiles {
		clone.Profiles[name] = profile
	}
	clone.Routes.Overrides = make(map[string]string, len(cfg.Routes.Overrides))
	for client, profile := range cfg.Routes.Overrides {
		clone.Routes.Overrides[client] = profile
	}
	clone.Adapters = make(map[string]domain.AdapterConfig, len(cfg.Adapters))
	for name, adapter := range cfg.Adapters {
		adapter.Targets = append([]string(nil), adapter.Targets...)
		clone.Adapters[name] = adapter
	}
	return clone
}

func rollbackConfigAndAdapters(ctx context.Context, app *App, before domain.Config, rebindNativeAuthentication bool) error {
	if err := app.Config.Save(before); err != nil {
		return err
	}
	if err := syncCodexProjection(ctx, app, before); err != nil {
		return err
	}
	if rebindNativeAuthentication {
		return bindCodexAuthentication(ctx, app, before)
	}
	return nil
}

func commitConfigAndSync(ctx context.Context, app *App, before, after domain.Config, subject string) error {
	if err := app.Config.Save(after); err != nil {
		return err
	}
	if codexProjectionChanged(before, after) {
		if err := syncCodexProjection(ctx, app, after); err != nil {
			rollbackErr := rollbackConfigAndAdapters(ctx, app, before, false)
			if rollbackErr != nil {
				return fmt.Errorf("%s synchronization failed: %w; rollback also failed: %v", subject, err, rollbackErr)
			}
			return fmt.Errorf("%s synchronization failed and was rolled back: %w", subject, err)
		}
	}
	if codexAuthenticationChanged(before, after) {
		if err := bindCodexAuthentication(ctx, app, after); err != nil {
			rollbackErr := rollbackConfigAndAdapters(ctx, app, before, true)
			if rollbackErr != nil {
				return fmt.Errorf("%s authentication failed: %w; rollback also failed: %v", subject, err, rollbackErr)
			}
			return fmt.Errorf("%s authentication failed and was rolled back: %w", subject, err)
		}
	}
	return nil
}

func _processPlanCompileGuard(_ adapters.ProcessPlan) {}

func accountForInput(cfg domain.Config, name string) (string, domain.Account, error) {
	cfg.Normalize()
	if account, ok := cfg.Accounts[name]; ok {
		account.ID = name
		return name, account, nil
	}
	if profile, ok := cfg.Profiles[name]; ok {
		account, exists := cfg.Accounts[profile.Account]
		if !exists {
			return "", domain.Account{}, fmt.Errorf("Profile %q references unknown account %q", name, profile.Account)
		}
		account.ID = profile.Account
		return profile.Account, account, nil
	}
	return "", domain.Account{}, fmt.Errorf("Unknown account or profile %q", name)
}
