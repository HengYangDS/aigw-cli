package provider

import (
	"fmt"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

func newListCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List the providers; reads no Token and sends no request",
		Example: "aigw provider list\naigw provider list --json",
		Args:    cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			document := listDocument{Providers: []providerItem{}}
			for _, id := range cfg.CatalogProviderIDs() {
				document.Providers = append(document.Providers, describe(cfg, id))
			}
			if jsonMode {
				return terminal.WriteDocument(runtime.Out, listSchemaVersion, true, document)
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Providers")
			if len(document.Providers) == 0 {
				r.Text("No provider is configured.")
				r.Next("aigw setup")
				return r.Err()
			}
			rows := make([]terminal.Field, 0, len(document.Providers))
			for _, item := range document.Providers {
				protocols := make([]string, 0, len(item.Endpoints))
				for _, endpoint := range item.Endpoints {
					protocols = append(protocols, protocolLabel(endpoint.Protocol))
				}
				rows = append(rows, terminal.Field{Label: item.ID, Value: strings.Join([]string{item.Label, strings.Join(protocols, ", "), count(len(item.Models), "model")}, " · ")})
			}
			r.Rows(rows...)
			r.Next("aigw provider show <provider>")
			return r.Err()
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the providers as JSON")
	return cmd
}

// count states a number of things, such as "1 model" or "3 models".
func count(number int, thing string) string {
	if number == 1 {
		return "1 " + thing
	}
	return fmt.Sprintf("%d %ss", number, thing)
}

func newShowCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "show <provider>",
		Short: "Show one provider: its endpoints, its models and whether its Token is stored",
		Long: "Show one provider: its endpoints, its models and whether its Token is stored.\n" +
			"It reads no Token: it asks the credential backend only whether one is stored.",
		Example:           "aigw provider show dmxapi\naigw provider show ucloud --json",
		Args:              providerArgument,
		ValidArgsFunction: completeProvider(runtime),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, provider, err := loadProvider(runtime, args[0])
			if err != nil {
				return err
			}
			backend, _ := secret.Inspect(runtime.Secrets)
			document := showDocument{
				providerItem: describe(cfg, provider.ID), SelectedBy: selectingClients(cfg, provider.ID),
				Token:             credentialState(runtime.Secrets.Exists, provider.ID),
				CredentialBackend: credentialBackend{Kind: backend.Kind, Availability: backend.Availability, Mutability: backend.Mutability},
			}
			// A store that cannot tell whether it holds the Token, such as one
			// whose folder a file replaced, does not answer either.
			if document.Token == credentialUnobserved {
				document.CredentialBackend.Availability = "unavailable"
			}
			if provider.BalanceProbe != nil {
				document.BalanceCredential = credentialState(runtime.Providers.Exists, provider.ID)
			}
			if jsonMode {
				return terminal.WriteDocument(runtime.Out, showSchemaVersion, true, document)
			}
			return renderShow(runtime, document)
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the provider as JSON")
	return cmd
}

func renderShow(runtime invocation.Context, document showDocument) error {
	r := invocation.Renderer(runtime)
	r.ProductTitle("Provider")
	models := strings.Join(document.Models, ", ")
	if models == "" {
		models = "None added"
	}
	fields := append([]terminal.Field{{Label: "Provider", Value: document.ID}, {Label: "Label", Value: document.Label}}, endpointFields(document.Endpoints)...)
	fields = append(fields, terminal.Field{Label: "Models", Value: models})
	if len(document.SelectedBy) > 0 {
		fields = append(fields, terminal.Field{Label: "Selected by", Value: clientLabels(runtime.Clients.Admission(), document.SelectedBy)})
	}
	fields = append(fields, credentialField("Token", document.Token, tokenVariables(runtime.Secrets, document.ID)))
	if document.BalanceProbe != "" {
		fields = append(fields, terminal.Field{Label: "Balance probe", Value: document.BalanceProbe},
			credentialField("Balance token", document.BalanceCredential, balanceVariables(runtime.Secrets, document.ID)))
	}
	fields = append(fields, backendField(document.CredentialBackend))
	r.Rows(fields...)
	next := "aigw model list --provider " + document.ID
	if document.Token == credentialAbsent && !secret.IsReadOnly(runtime.Secrets) {
		next = "aigw provider rotate " + document.ID
	}
	r.Next(next)
	return r.Err()
}

// backendField is the row of the credential backend: its kind, whether it
// only reads credentials, and whether it answers, which tells a backend that
// failed from a credential that is missing.
func backendField(backend credentialBackend) terminal.Field {
	value := backend.Kind + " backend"
	if backend.Mutability == "read_only" {
		value += ", read-only"
	}
	if backend.Availability != "available" {
		return terminal.Field{State: terminal.Warn, Label: "Credential backend", Value: value + ", " + strings.ReplaceAll(backend.Availability, "_", " ")}
	}
	return terminal.Field{State: terminal.OK, Label: "Credential backend", Value: value + ", available"}
}

// credentialState observes whether a credential slot holds a value, without
// reading it, through the slot's exists function. A backend that cannot tell
// is reported as such, never as an absent credential.
func credentialState(exists func(string) (bool, error), id string) string {
	present, err := exists(id)
	switch {
	case err != nil:
		return credentialUnobserved
	case present:
		return credentialPresent
	default:
		return credentialAbsent
	}
}

// credentialField is the row of a credential slot's state. The read-only
// environment backend names the variables that supply it.
func credentialField(label, state string, variables []string) terminal.Field {
	switch {
	case state == credentialUnobserved:
		return terminal.Field{State: terminal.Warn, Label: label, Value: "Unknown: the credential backend could not tell"}
	case len(variables) > 0 && state == credentialPresent:
		return terminal.Field{State: terminal.OK, Label: label, Value: "Set in " + strings.Join(variables, " and ")}
	case len(variables) > 0:
		return terminal.Field{State: terminal.Warn, Label: label, Value: "Not set: " + strings.Join(variables, " and ")}
	case state == credentialPresent:
		return terminal.Field{State: terminal.OK, Label: label, Value: "Stored"}
	default:
		return terminal.Field{State: terminal.Warn, Label: label, Value: "Missing"}
	}
}

// tokenVariables names the variable that supplies the provider's Token when
// the read-only environment backend is selected, and nothing otherwise.
func tokenVariables(store secret.Store, id string) []string {
	if !secret.IsReadOnly(store) {
		return nil
	}
	return []string{secret.EnvironmentKey(id)}
}

// balanceVariables names the variables that supply the provider's balance
// credentials when the read-only environment backend is selected, and nothing
// otherwise.
func balanceVariables(store secret.Store, id string) []string {
	if !secret.IsReadOnly(store) {
		return nil
	}
	return []string{secret.DiagnosticSystemTokenEnvironmentKey(id), secret.DiagnosticUserIDEnvironmentKey(id)}
}

// providerFields are the rows that identify a provider in a command's result.
func providerFields(id string, provider domain.Provider) []terminal.Field {
	return []terminal.Field{{Label: "Provider", Value: id}, {Label: "Label", Value: provider.Label}}
}
