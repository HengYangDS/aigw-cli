package self

import (
	"debug/buildinfo"
	"errors"
	"fmt"
	"os"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/program"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// versionVariable is the linker variable that an official build sets to the
// program's version.
const versionVariable = "aigw-cli/internal/cli.Version="

// rollbackPreview is `aigw self rollback --dry-run`'s JSON document: the
// retained program that a rollback would activate, its version when its
// build information names one, and the configuration file that the
// rollback's check stages for that program to read.
type rollbackPreview struct {
	DryRun        bool   `json:"dry_run"`
	Program       string `json:"program"`
	Version       string `json:"version,omitempty"`
	VersionUnread string `json:"version_unread,omitempty"`
	Configuration string `json:"configuration"`
	NextAction    string `json:"next_action,omitempty"`
}

// newRollbackCommand constructs `aigw self rollback`, which activates the
// retained previous portable program, or, for a program that Homebrew owns,
// returns the credentials to the reader of an earlier release before Homebrew
// installs it.
func newRollbackCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode, dryRun bool
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Restore the retained previous program, not the configuration",
		Long: "Activate the program that the last update or installation retained.\n" +
			"Its check stages the configuration file that the previous program reads,\n" +
			"and a configuration that it cannot read blocks the rollback without\n" +
			"changing files. Before Homebrew installs an earlier release, this returns\n" +
			"macOS Keychain items to that release's reader.",
		Example: "aigw self rollback --dry-run\naigw self rollback\naigw self rollback --json",
		Args:    cobra.NoArgs,
		RunE: func(ctx *cobra.Command, _ []string) error {
			if err := program.RequirePortableOwnership(runtime.Executable); err != nil {
				if !errors.Is(err, program.ErrHomebrewManaged) {
					return err
				}
				if dryRun {
					return previewHomebrewRollback(runtime, jsonMode)
				}
				return prepareHomebrewRollback(runtime, jsonMode)
			}
			if dryRun {
				return previewRollback(runtime, jsonMode)
			}
			if runtime.Updater == nil {
				return fmt.Errorf("program rollback is unavailable in this build")
			}
			config, readErr := runtime.Config.RollbackConfiguration()
			if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
				return fmt.Errorf("read configuration before program rollback: %w", readErr)
			}
			// A configuration this program cannot load names no credential;
			// the retained program's own compatibility check then decides.
			custody, _ := credentialCustody(runtime.Dependencies)
			result, err := runtime.Updater.Rollback(ctx.Context(), config, custody)
			if err != nil && result.State != "" {
				return reportIncompleteCleanup(runtime, jsonMode, true, result, err)
			}
			if errors.Is(err, program.ErrRollbackConfiguration) {
				return terminal.ProblemError(
					"Program rollback did not complete",
					"The retained program did not complete the isolated configuration-read check.",
					"The active program, retained program and configuration remain unchanged.",
					"Keep a compatible program, or use `aigw config rollback` to restore the accepted configuration before you run it again; do not delete policy fields by hand.", err)
			}
			if err != nil {
				return programProblem("Program rollback did not complete", "AIGW could not activate the retained previous program.",
					"No previous program version was confirmed active.", "aigw self status", err)
			}
			return reportTransition(runtime, jsonMode, "Program rollback", result)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the retained program, its version and the configuration file that a rollback would use, without running or changing anything")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the program transition as JSON")
	return cmd
}

// previewRollback reports the retained program that a rollback would
// activate, its version, read from the program's build information without
// running it, because a preview runs no other program, and the configuration
// file that the rollback's check stages for it.
func previewRollback(runtime invocation.Context, jsonMode bool) error {
	retained := program.RollbackPath(runtime.Executable)
	if _, err := os.Lstat(retained); err != nil {
		if failure := platform.ClassifyPath(retained, platform.FileExpected, err); !failure.Absent() {
			return failure
		}
		return terminal.ProblemError("No previous program to roll back to", "No retained program is beside "+runtime.Executable+".",
			"Nothing changed.", "aigw self status", err)
	}
	version, unread := retainedVersion(retained)
	document := rollbackPreview{DryRun: true, Program: retained, Version: version, VersionUnread: unread,
		Configuration: runtime.Config.Path(), NextAction: "aigw self rollback"}
	if jsonMode {
		return terminal.WriteDocument(runtime.Out, updateSchemaVersion, true, document)
	}
	shown := version
	if shown == "" {
		shown = "Unknown: " + unread
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Program rollback preview")
	r.Rows(
		terminal.Field{Label: "Previous program", Value: retained},
		terminal.Field{Label: "Version", Value: shown},
		terminal.Field{Label: "Configuration", Value: document.Configuration},
	)
	r.Success("Preview did not run or change a program, the configuration or a Keychain item")
	r.Next(document.NextAction)
	return nil
}

// retainedVersion reads the version that the official build of program set,
// from its embedded build information, without running it. When it cannot,
// it says why instead.
func retainedVersion(program string) (version, unread string) {
	info, err := buildinfo.ReadFile(program)
	if err != nil {
		return "", "its build information cannot be read"
	}
	for _, setting := range info.Settings {
		if setting.Key != "-ldflags" {
			continue
		}
		words := strings.Fields(setting.Value)
		for index, word := range words {
			if word == "-X" && index+1 < len(words) {
				word = words[index+1]
			}
			if value, found := strings.CutPrefix(strings.TrimPrefix(word, "-X="), versionVariable); found {
				return strings.Trim(value, `'"`), ""
			}
		}
	}
	return "", "its build information names no version"
}

// prepareHomebrewRollback returns macOS Keychain items to the reader of an
// earlier release before Homebrew installs it; Homebrew keeps its program
// files. If the operator keeps this release, its next Token use takes the
// items back. Without a readable configuration it cannot name those items, so
// it stops before any native request.
func prepareHomebrewRollback(runtime invocation.Context, jsonMode bool) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return homebrewConfigurationProblem(runtime, "Configuration is not ready for a Homebrew rollback",
			"AIGW could not read the configuration, including each client's retry policy.",
			"No credential, policy or program changed.", err)
	}
	for _, binding := range cfg.Clients {
		if binding.RetryPolicy != nil {
			return terminal.ProblemError(
				"Homebrew rollback cannot prove the retry policy",
				"No retained Homebrew program was supplied to the native configuration check.",
				"No credential, policy or program changed.",
				"Keep this compatible AIGW release, or use `aigw config rollback` to restore the accepted configuration before choosing a predecessor.",
				fmt.Errorf("Homebrew rollback cannot prove the selected retry policy"))
		}
	}
	custody, err := credentialCustody(runtime.Dependencies)
	if err != nil {
		return homebrewConfigurationProblem(runtime, "Credentials are not ready for a Homebrew rollback",
			"AIGW could not read the configuration that names its credentials.",
			"No credential or program changed.", err)
	}
	if err := custody.Release(""); err != nil {
		return terminal.ProblemError(
			"Credentials are not ready for a Homebrew rollback",
			"AIGW could not return every Keychain item to the reader of earlier releases.",
			"The installed program is unchanged and can still read every credential.",
			"Run `aigw check`, then run `aigw self rollback` again before Homebrew installs the earlier release.",
			errors.Join(err, custody.Reclaim()))
	}
	if jsonMode {
		return terminal.WriteDocument(runtime.Out, updateSchemaVersion, true, updateDocument{State: "prepared",
			Summary:    "Credentials are ready for an earlier AIGW release.",
			NextDetail: "Install the earlier release with Homebrew, then run `aigw sync` and `aigw check`."})
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Homebrew rollback")
	r.Success("Credentials are ready for an earlier AIGW release.")
	r.Text("Install the earlier release with Homebrew, then run its sync and check commands. If you keep this release, its next Token use takes the credentials back.")
	r.Next("aigw sync")
	return nil
}

// previewHomebrewRollback says what a Homebrew rollback would prepare, and
// changes nothing.
func previewHomebrewRollback(runtime invocation.Context, jsonMode bool) error {
	const summary = "A rollback would return the Keychain items to the reader of earlier releases before Homebrew installs one."
	if jsonMode {
		return terminal.WriteDocument(runtime.Out, updateSchemaVersion, true, updateDocument{DryRun: true, State: "preview", Summary: summary, NextAction: "aigw self rollback"})
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Homebrew rollback preview")
	r.Text(summary)
	r.Success("Preview did not change a credential, a policy or a program")
	r.Next("aigw self rollback")
	return nil
}

// homebrewConfigurationProblem refuses a Homebrew rollback whose configuration
// did not load: its evidence names the stage that failed, and its fix repairs
// the configuration before the rollback runs again.
func homebrewConfigurationProblem(runtime invocation.Context, title, evidence, impact string, err error) error {
	if stage, ok := terminal.ConfigurationLoadEvidence("AIGW "+runtime.Version, err); ok {
		evidence = stage
	}
	fix := terminal.ConfigurationLoadFix(err) + " Then run `aigw self rollback` again before Homebrew installs the earlier release."
	return terminal.ProblemError(title, evidence, impact, fix, err)
}
