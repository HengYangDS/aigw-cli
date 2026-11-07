package provider

import (
	"errors"
	"fmt"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// addOptions are the flags of `aigw provider add`.
type addOptions struct {
	label      string
	endpoints  []string
	tokenStdin bool
	dryRun     bool
	jsonMode   bool
}

func newAddCommand(runtime invocation.Context) *cobra.Command {
	var options addOptions
	cmd := &cobra.Command{
		Use:   "add <provider>",
		Short: "Add a provider with its endpoints and a Token that it accepts",
		Long: "Add a provider with its endpoints and a Token that it accepts.\n" +
			"AIGW sends one authenticated request that names no model, and stores the Token only when the provider accepts it.",
		Example: "aigw provider add dmxapi --endpoint responses=https://www.dmxapi.cn/v1 --token-stdin\n" +
			"aigw provider add ucloud --label UCloud --endpoint anthropic=https://api.modelverse.cn\n" +
			"aigw provider add dmxapi --endpoint responses=https://www.dmxapi.cn/v1 --dry-run",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := providerArgument(cmd, args); err != nil {
				return err
			}
			if id := args[0]; !domain.ValidIdentifier(id) || id != strings.ToLower(id) {
				return fmt.Errorf("%q is not a provider ID; use lowercase letters, numbers, dots, hyphens or underscores; run `%s --help`", id, cmd.CommandPath())
			}
			if cmd.Flags().Changed("label") && strings.TrimSpace(options.label) == "" {
				return fmt.Errorf("--label requires a non-empty value; run `%s --help`", cmd.CommandPath())
			}
			if len(options.endpoints) == 0 {
				return fmt.Errorf("%s needs an --endpoint <protocol>=<url>; run `%s --help`", cmd.CommandPath(), cmd.CommandPath())
			}
			if _, err := parseEndpoints(cmd, options.endpoints); err != nil {
				return err
			}
			return requireTokenInput(cmd, runtime, options.tokenStdin, options.jsonMode, options.dryRun)
		},
		ValidArgsFunction: invocation.CompleteNothing,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, runtime, args, options)
		},
	}
	cmd.Flags().StringVar(&options.label, "label", "", "Display name; the provider ID by default")
	bindEndpoints(cmd, &options.endpoints, "An endpoint of the provider")
	cmd.Flags().BoolVar(&options.tokenStdin, "token-stdin", false, "Read the Token from standard input")
	cmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "Show the provider without reading a Token, sending a request or writing anything")
	cmd.Flags().BoolVar(&options.jsonMode, "json", false, "Write the added provider as JSON")
	invocation.CompleteFlag(cmd, "label", cobra.NoFileCompletions)
	return cmd
}

func runAdd(cmd *cobra.Command, runtime invocation.Context, args []string, options addOptions) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	id := args[0]
	if _, exists := cfg.Providers[id]; exists {
		return terminal.ProblemError("provider "+id+" already exists; `aigw provider edit` changes it and `aigw provider rotate` replaces its Token", "", "",
			"aigw provider edit "+id+" --help", nil)
	}
	if len(cfg.Models) == 0 {
		return terminal.ProblemError("AIGW is not set up yet; setup connects the first provider together with a model", "", "", "aigw setup", nil)
	}
	provider := domain.Provider{Label: strings.TrimSpace(options.label)}
	if provider.Label == "" {
		provider.Label = id
	}
	endpoints, _ := parseEndpoints(cmd, options.endpoints)
	for _, given := range endpoints {
		setEndpoint(&provider.Endpoints, given.protocol, given.url)
	}
	before := cfg.Clone()
	cfg.Providers[id] = provider
	preview, err := invocation.PreviewChange(runtime, before, cfg)
	if err != nil {
		return err
	}
	validated, stored := false, false
	next := "aigw model add " + id + "/<model>"
	if options.dryRun {
		next = invocation.ApplyCommand(cmd, args)
	} else {
		if stored, err = connect(cmd, runtime, args, options, before, cfg, id); err != nil {
			return err
		}
		validated = true
	}
	return result{
		version: addSchemaVersion, title: "Provider added", next: next,
		rows: append(append(providerFields(id, provider), endpointFields(endpointItems(provider.Endpoints))...), tokenField(runtime.Secrets, id, options.dryRun, stored)),
		document: changeDocument{
			providerItem: describe(cfg, id), TokenValidated: &validated, TokenStored: &stored,
			ClientFiles: invocation.ClientFiles(runtime, preview), CredentialReader: preview.CredentialEntrypoint,
		},
		success: "Saved; no model was added and no client changed",
	}.write(runtime, options.dryRun, options.jsonMode)
}

// connect validates the provider's Token with one authenticated request that
// names no model, and once the provider accepts it stores it and saves the
// provider: a save that fails restores the previous Token, so a refusal stores
// nothing. The read-only environment backend supplies the Token, which connect
// validates and leaves where it is.
func connect(cmd *cobra.Command, runtime invocation.Context, args []string, options addOptions, before, after domain.Config, id string) (bool, error) {
	token, err := addToken(cmd, runtime, args, options, after.Providers[id].Label, id)
	if err != nil {
		return false, err
	}
	provider := after.Providers[id]
	provider.ID = id
	if err := probe.Validate(cmd.Context(), runtime.HTTP, runtime.Clients.Admission(), provider, token); err != nil {
		return false, invocation.TokenValidationProblem(cmd, args, runtime.Secrets, id, func(protocol domain.EndpointProtocol) string {
			return correctEndpoint(cmd, args, options.endpoints, protocol)
		}, err)
	}
	if err := cmd.Context().Err(); err != nil {
		return false, err
	}
	if secret.IsReadOnly(runtime.Secrets) {
		return false, runtime.Synchronizer().Commit(cmd.Context(), before, after, "provider add")
	}
	restore, err := secret.Replace(runtime.Secrets, map[string]string{id: token})
	if err != nil {
		return false, err
	}
	if err := runtime.Synchronizer().Commit(cmd.Context(), before, after, "provider add"); err != nil {
		return false, errors.Join(err, restore())
	}
	return true, nil
}

// addToken is the Token that add validates: one read from standard input or a
// prompt, or with the read-only environment backend the one that its variable
// supplies, which --token-stdin cannot replace.
func addToken(cmd *cobra.Command, runtime invocation.Context, args []string, options addOptions, label, id string) (string, error) {
	if !secret.IsReadOnly(runtime.Secrets) {
		return readToken(runtime, "Paste the "+label+" Token: ", options.tokenStdin)
	}
	if options.tokenStdin {
		return "", readOnlyToken(id, "store", invocation.Rerun(cmd, args, []string{"token-stdin"}))
	}
	token, err := runtime.Secrets.Get(id)
	if errors.Is(err, secret.ErrNotFound) {
		key := secret.EnvironmentKey(id)
		fix := "Set " + key + " in aigw's environment, then run the same command again."
		if line := invocation.Rerun(cmd, args, nil); line != "" {
			fix = "Set " + key + " in aigw's environment, then run `" + line + "`."
		}
		return "", terminal.ProblemError("the environment backend has no "+id+" Token: "+key+" is not set", "", "", fix, nil)
	}
	return token, err
}

// correctEndpoint is the fix for an endpoint that neither accepted nor
// rejected the Token: the same command with that protocol's --endpoint as a
// placeholder to correct, which makes the fix guidance, and the other
// endpoints as given.
func correctEndpoint(cmd *cobra.Command, args, values []string, protocol domain.EndpointProtocol) string {
	words := make([]string, 0, 2*len(values))
	for _, value := range values {
		name, _, _ := strings.Cut(value, "=")
		if name == protocol.Name() {
			value = name + "=<url>"
		} else if invocation.Command(value) == "" {
			return "Run the same command with a corrected --endpoint " + protocol.Name() + "=<url>."
		}
		words = append(words, "--endpoint", value)
	}
	if line := invocation.Rerun(cmd, args, []string{"endpoint"}, words...); line != "" {
		return line
	}
	return "Run the same command with a corrected --endpoint " + protocol.Name() + "=<url>."
}

// tokenField says what add did with the Token: nothing in a preview, stored it
// once the provider accepted it, or validated the one that the read-only
// environment backend supplies.
func tokenField(store secret.Store, id string, dryRun, stored bool) terminal.Field {
	switch {
	case dryRun:
		return terminal.Field{State: terminal.Info, Label: "Token", Value: "Not read in a preview, and no request was sent"}
	case stored:
		return terminal.Field{State: terminal.OK, Label: "Token", Value: "Accepted by the provider and stored"}
	default:
		return terminal.Field{State: terminal.OK, Label: "Token", Value: "Accepted by the provider; " + secret.EnvironmentKey(id) + " supplies it"}
	}
}

// result is the outcome of one provider change: its document and the rows,
// success and next step of its human output. A preview's next step applies it.
type result struct {
	version  int
	document changeDocument
	title    string
	rows     []terminal.Field
	success  string
	next     string
}

// write reports a change as JSON or for a person, after it was applied or as
// the preview that dryRun marks.
func (c result) write(runtime invocation.Context, dryRun, jsonMode bool) error {
	c.document.DryRun = dryRun
	if jsonMode {
		c.document.NextAction, c.document.NextDetail = terminal.NextSteps(c.next)
		return terminal.WriteDocument(runtime.Out, c.version, true, c.document)
	}
	r := invocation.Renderer(runtime)
	title := c.title
	if dryRun {
		title += " (preview)"
	}
	r.ProductTitle(title)
	r.Rows(c.rows...)
	invocation.RenderClientFiles(r, runtime, c.document.ClientFiles)
	if reader := c.document.CredentialReader; reader != nil {
		r.Section("Credential reader")
		r.Text("Install " + reader.Path)
	}
	if dryRun {
		r.Success("Nothing was written: configuration, credentials and client files are unchanged")
	} else {
		r.Success(c.success)
	}
	r.Next(c.next)
	return r.Err()
}
