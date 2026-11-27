// Package client implements `aigw client`, which lists the clients with their
// programs, and enables, disables and edits them.
package client

import (
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// NewCommand constructs the client lifecycle command tree.
func NewCommand(runtime invocation.Context) *cobra.Command {
	root := &cobra.Command{Use: "client", Short: "List, enable, disable and edit clients"}
	root.AddCommand(newListCommand(runtime), newEnableCommand(runtime), newDisableCommand(runtime), newEditCommand(runtime))
	return root
}

// clientArgument checks that a command has its one <client> argument and
// returns the client that it names. An unknown one is refused with the
// closest client IDs.
func clientArgument(cmd *cobra.Command, admission domain.Admission, args []string) (domain.ClientSpec, error) {
	switch len(args) {
	case 0:
		return domain.ClientSpec{}, fmt.Errorf("%s needs a <client> argument: %s; run `%s --help`", cmd.CommandPath(), admission.Usage(), cmd.CommandPath())
	case 1:
		return invocation.ResolveClient(admission, args[0])
	default:
		return domain.ClientSpec{}, fmt.Errorf("%s takes one <client> argument, not %d; run `%s --help`", cmd.CommandPath(), len(args), cmd.CommandPath())
	}
}

// enableExamples enables a client that finds its own configuration file and
// its program, one that declares its program, and previews enabling one that
// is told its file with --target and finds its program in an editor's
// extensions directory.
func enableExamples(admission domain.Admission) string {
	var lines []string
	specs := admission.Specs()
	if index := slices.IndexFunc(specs, func(spec domain.ClientSpec) bool { return spec.TargetFile == "" }); index >= 0 {
		id := specs[index].ID
		lines = append(lines, "aigw client enable "+id, "aigw client enable "+id+" --program ~/.local/bin/"+id)
	}
	if index := slices.IndexFunc(specs, func(spec domain.ClientSpec) bool { return spec.TargetFile != "" && hasExtension(spec) }); index >= 0 {
		id := specs[index].ID
		lines = append(lines, "aigw client enable "+id+" --extensions ~/.vscode/extensions --target ~/."+id+"/"+specs[index].TargetFile+" --dry-run")
	}
	return strings.Join(lines, "\n")
}

// disableExamples previews and then disables the first client.
func disableExamples(admission domain.Admission) string {
	specs := admission.Specs()
	if len(specs) == 0 {
		return ""
	}
	return "aigw client disable " + specs[0].ID + " --dry-run\naigw client disable " + specs[0].ID
}

// completeClient completes the one <client> argument of a command, and
// nothing once it is given.
func completeClient(runtime invocation.Context) cobra.CompletionFunc {
	clients := invocation.CompleteClients(runtime)
	return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return clients(cmd, args, toComplete)
	}
}

// checkEnableArguments checks `aigw client enable`'s client, its program
// declaration and its targets before anything runs: a target file that no
// client can read is refused before any validator, discovery or route check,
// which would report it otherwise.
func checkEnableArguments(cmd *cobra.Command, runtime invocation.Context, args []string, declared programOptions, targets []string) error {
	spec, err := clientArgument(cmd, runtime.Clients.Admission(), args)
	if err != nil {
		return err
	}
	if err := checkProgram(cmd, runtime.Clients.Admission(), spec, declared); err != nil {
		return err
	}
	if spec.TargetFile != "" && len(targets) == 0 {
		return fmt.Errorf("%s requires at least one --target %s; run `aigw client enable --help`", spec.Label, spec.TargetFile)
	}
	for _, target := range targets {
		if strings.TrimSpace(target) == "" {
			return fmt.Errorf("--target requires a non-empty path; run `aigw client enable --help`")
		}
	}
	if err := checkTargetsRead(cmd, runtime.Clients.Admission(), spec, targets); err != nil {
		return err
	}
	absolute, err := absoluteTargets(cmd, targets)
	if err != nil {
		return err
	}
	// A file that no client can read is refused before any validator,
	// discovery or route check, which would report it otherwise.
	return refuseTargetsUnderFiles(absolute, spec, invocation.Home(runtime.Dependencies))
}

func newEnableCommand(runtime invocation.Context) *cobra.Command {
	var declared programOptions
	var targets []string
	var jsonMode, dryRun bool
	cmd := &cobra.Command{Use: "enable <client>", Short: "Enable a client and write AIGW's settings into its files", ValidArgsFunction: completeClient(runtime), Args: func(cmd *cobra.Command, args []string) error {
		return checkEnableArguments(cmd, runtime, args, declared, targets)
	}, RunE: func(cmd *cobra.Command, args []string) error {
		client := args[0]
		spec, _ := runtime.Clients.Admission().Spec(client)
		// Args checked the paths, so they do not fail here. The configuration
		// names them absolute, so that they mean the same from any directory.
		targets, _ = absoluteTargets(cmd, targets)
		declaration, err := declaredProgram(runtime, spec, declared, "enable", targetWords(targets)...)
		if err != nil {
			return err
		}
		cfg, err := runtime.LoadConfiguration()
		if err != nil {
			return err
		}
		before := cfg.Clone()
		current := before.Clients[client]
		if current.Enabled && (current.DeclaresProgram() || len(current.Targets) > 0) {
			return terminal.ProblemError(spec.Label+" is already enabled, so its program and configuration files change through client edit", "", "", editInstead(spec, declaration, targets), nil)
		}
		if err := admitEnablement(runtime, cfg, client, targets); err != nil {
			return err
		}
		if len(targets) == 0 {
			targets = current.Targets
		}
		// The client keeps the program that it declares unless a flag
		// declares another; one that declares none adopts the program that
		// discovery finds.
		cfg.SetClientActivation(client, true, current.Program, targets)
		if declaration.declares() {
			cfg.DeclareProgram(client, declaration.program, declaration.extensions)
		}
		if cfg, err = runtime.Synchronizer().AdoptProgram(cfg, client); err != nil {
			return err
		}
		if dryRun {
			preview, err := invocation.PreviewChange(runtime, before, cfg)
			if err != nil {
				return fmt.Errorf("Client enablement preview could not finish: %w", err)
			}
			if !cfg.Clients[client].DeclaresProgram() {
				return programNotFound(cfg, spec, "A run would save "+cfg.Admission().ProductName(client)+" as enabled and write none of its files.")
			}
			return writeProjectionPreview(runtime, enableSchemaVersion, jsonMode, spec, "enabled", preview, invocation.ApplyCommand(cmd, args))
		}
		if err := runtime.Synchronizer().Commit(cmd.Context(), before, cfg, "client enable"); err != nil {
			return fmt.Errorf("Client enablement could not finish: %w", err)
		}
		// A client whose program is not found stays enabled, as a client that
		// is not installed yet does, and its files wait for its program.
		if !cfg.Clients[client].DeclaresProgram() {
			return programNotFound(cfg, spec, "AIGW saved "+cfg.Admission().ProductName(client)+" as enabled and wrote none of its files.")
		}
		if jsonMode {
			return terminal.WriteDocument(runtime.Out, enableSchemaVersion, true,
				newProjectionDocument(client, "enabled", spec.RestartAfterProjection, enabledNext(spec)))
		}
		renderEnabled(invocation.Renderer(runtime), spec)
		return nil
	}}
	cmd.Example = enableExamples(runtime.Clients.Admission())
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the client files that enabling would change without writing them")
	bindProgram(cmd, &declared, runtime.Clients.Admission())
	cmd.Flags().StringArrayVar(&targets, "target", nil, targetHelp(runtime.Clients.Admission()))
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the enabled client as JSON")
	return cmd
}

// programNotFound is the refusal of enabling a client that declares no
// program and of which discovery found none: the missing program's problem
// and repair, with impact, what became of the client.
func programNotFound(cfg domain.Config, spec domain.ClientSpec, impact string) error {
	stop := adapter.ProgramNotFound(cfg, spec.ID)
	return terminal.ProblemError(stop.Error(), "AIGW looks for "+cfg.Admission().ProductName(spec.ID)+" on the PATH, and at its application path where it has one; it searches no extensions directory.",
		impact, stop.NextAction(), stop)
}

// targetWords are the --target flags of targets.
func targetWords(targets []string) []string {
	words := make([]string, 0, 2*len(targets))
	for _, target := range targets {
		words = append(words, "--target", target)
	}
	return words
}

// holdsToken reports whether the store holds provider's Token: a slot that is
// there and holds a value. A Keychain item that macOS refuses AIGW is no Token
// that a client could read, and its refusal, which names its fix, is the
// error.
func holdsToken(store secret.Store, provider string) (bool, error) {
	observation := secret.ObserveToken(store, provider)
	if observation.Unobserved != nil {
		return false, fmt.Errorf("Cannot inspect provider %q credential: %w", provider, observation.Unobserved)
	}
	if observation.Refusal != nil {
		return false, observation.Refusal
	}
	return observation.Usable(), nil
}

// admitEnablement refuses to enable a client whose Account has no Token, or
// with a configuration target that the client does not accept. Enabling is
// for that client alone, so the refusal of its provider's Keychain item ends
// it.
func admitEnablement(runtime invocation.Context, cfg domain.Config, client string, targets []string) error {
	// A file that no client can read is refused before anything else that
	// enabling needs, such as a selected model.
	spec, _ := cfg.Admission().Spec(client)
	if err := refuseTargetsUnderFiles(targets, spec, invocation.Home(runtime.Dependencies)); err != nil {
		return err
	}
	clientRuntime, err := cfg.ResolveRuntime(client, domain.ModelRef{})
	if err != nil {
		return err
	}
	if clientRuntime.UsesAIGWCredentialStore() {
		held, err := holdsToken(runtime.Secrets, clientRuntime.ProviderID)
		if err != nil {
			return err
		}
		if !held {
			instruction, _ := secret.TokenRecovery(runtime.Secrets, clientRuntime.ProviderID)
			return fmt.Errorf("provider %q is missing a Token; %s", clientRuntime.ProviderID, instruction)
		}
	}
	if !runtime.Clients.ValidatesTargets(client) {
		return nil
	}
	discovered, err := runtime.Discover(client)
	if err != nil {
		return err
	}
	for _, target := range targets {
		if err := runtime.Clients.ValidateTarget(client, discovered, target); err != nil {
			return err
		}
	}
	return nil
}

// checkTargetsRead refuses --target for a client whose adapter reads no
// configuration target, naming the clients that read one, and a second
// --target for a client that reads one configuration file.
func checkTargetsRead(cmd *cobra.Command, admission domain.Admission, spec domain.ClientSpec, targets []string) error {
	switch {
	case len(targets) == 0:
		return nil
	case !spec.ReadsTargets():
		return fmt.Errorf("%s has no configuration file to name; --target is for %s; run `%s --help`",
			spec.Label, clientsWith(admission, domain.ClientSpec.ReadsTargets), cmd.CommandPath())
	case len(targets) > 1 && spec.ReadsOneTarget():
		return fmt.Errorf("%s reads one configuration file, so it takes one --target, not %d; run `%s --help`", spec.Label, len(targets), cmd.CommandPath())
	}
	return nil
}

// targetHelp explains the target option in one sentence: the clients that
// read a configuration file, and each client's repeated-target usage.
func targetHelp(admission domain.Admission) string {
	help := []string{"The client's configuration `file`, for " + clientsWith(admission, domain.ClientSpec.ReadsTargets)}
	for _, spec := range admission.Specs() {
		if spec.TargetsUsage != "" {
			help = append(help, spec.TargetsUsage)
		}
	}
	return strings.Join(help, "; ")
}

func newDisableCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode, dryRun bool
	cmd := &cobra.Command{Use: "disable <client>", Short: "Disable a client and remove AIGW's settings from its files", ValidArgsFunction: completeClient(runtime), Args: func(cmd *cobra.Command, args []string) error {
		_, err := clientArgument(cmd, runtime.Clients.Admission(), args)
		return err
	}, RunE: func(cmd *cobra.Command, args []string) error {
		client := args[0]
		spec, _ := runtime.Clients.Admission().Spec(client)
		cfg, err := runtime.LoadConfiguration()
		if err != nil {
			return err
		}
		before := cfg.Clone()
		binding, ok := cfg.Clients[client]
		if !ok || !binding.Enabled {
			if jsonMode {
				document := newProjectionDocument(client, "already_disabled", false, "")
				document.DryRun = dryRun
				return terminal.WriteDocument(runtime.Out, disableSchemaVersion, true, document)
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Client unchanged")
			r.Rows(terminal.Field{State: terminal.Info, Label: spec.Label, Value: "Already disabled"})
			return nil
		}
		if err := runtime.Synchronizer().Withdraw(&cfg, client); err != nil {
			return err
		}
		// Disabled intent is durable; discovery must not turn it back into activation.
		// Full uninstall uses Withdraw without retaining this client binding.
		binding.Enabled = false
		cfg.Clients[client] = binding

		if dryRun {
			preview, err := invocation.PreviewProjection(runtime, before, cfg, client)
			if err != nil {
				return err
			}
			return writeProjectionPreview(runtime, disableSchemaVersion, jsonMode, spec, "disabled", preview, invocation.ApplyCommand(cmd, args))
		}
		if err := runtime.Synchronizer().CommitProjection(cmd.Context(), before, cfg, "client disable", client); err != nil {
			return err
		}
		if jsonMode {
			return terminal.WriteDocument(runtime.Out, disableSchemaVersion, true,
				newProjectionDocument(client, "disabled", spec.RestartAfterProjection, disabledNext(spec)))
		}
		renderDisabled(invocation.Renderer(runtime), spec)
		return nil
	}}
	cmd.Example = disableExamples(runtime.Clients.Admission())
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the client files that disabling would change without writing them")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the disabled client as JSON")
	return cmd
}

// writeProjectionPreview reports the projection state that enabling or
// disabling a client would reach and the client files it would change.
func writeProjectionPreview(runtime invocation.Context, version int, jsonMode bool, spec domain.ClientSpec, state string, preview invocation.Preview, apply string) error {
	if jsonMode {
		document := newProjectionDocument(spec.ID, state, spec.RestartAfterProjection, apply)
		document.DryRun, document.Preview = true, preview
		return terminal.WriteDocument(runtime.Out, version, true, document)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Client preview")
	r.Rows(terminal.Field{Label: "Client", Value: spec.Label}, terminal.Field{Label: "Planned state", Value: strings.ReplaceAll(state, "_", " ")})
	invocation.RenderPreview(r, preview, apply)
	return nil
}

func renderEnabled(r *terminal.Renderer, spec domain.ClientSpec) {
	rows := []terminal.Field{
		{Label: "Client", Value: spec.Label},
		{State: terminal.OK, Label: "Client files", Value: "Configured"},
	}
	if !spec.RestartAfterProjection {
		r.ProductTitle("Client enabled")
		r.Rows(rows...)
		r.Next(enabledNext(spec))
		return
	}
	r.ProductTitle("Client configured")
	r.Rows(append(rows, terminal.Field{State: terminal.Info, Label: "Activation", Value: "Restart required"})...)
	r.Next(enabledNext(spec))
}

// enabledNext is the follow-up after a client's projection is enabled.
func enabledNext(spec domain.ClientSpec) string {
	if spec.RestartAfterProjection {
		return fmt.Sprintf("Restart %s, then run `aigw check`", spec.Label)
	}
	return "aigw check"
}

// disabledNext is the follow-up after a client's projection is withdrawn, if
// any.
func disabledNext(spec domain.ClientSpec) string {
	if spec.RestartAfterProjection {
		return fmt.Sprintf("Restart %s to finish deactivation", spec.Label)
	}
	return ""
}

func renderDisabled(r *terminal.Renderer, spec domain.ClientSpec) {
	r.ProductTitle("Client disabled")
	rows := []terminal.Field{{Label: "Client", Value: spec.Label}}
	if spec.RestartAfterProjection {
		rows = append(rows, terminal.Field{State: terminal.Info, Label: "Deactivation", Value: "Restart required"})
	}
	r.Rows(rows...)
	r.Success("AIGW's settings were removed from the client's files")
	if next := disabledNext(spec); next != "" {
		r.Next(next)
	}
}

// editInstead is the edit that sets an enabled client's program and
// configuration files, as enabling it again would have: the exact command, or
// the program's repair when a path would need quoting or nothing would
// change. Enable refuses targets for a client that reads none, so the edit
// names none for it.
func editInstead(spec domain.ClientSpec, declaration programOptions, targets []string) string {
	var flags []string
	switch {
	case declaration.program != "":
		flags = []string{"--program", declaration.program}
	case declaration.extensions != "":
		flags = []string{"--extensions", declaration.extensions}
	}
	flags = append(flags, targetWords(targets)...)
	if command := clientCommand("edit", spec.ID, flags...); command != "" && len(flags) > 0 {
		return command
	}
	return adapter.ProgramRepair(spec)
}
