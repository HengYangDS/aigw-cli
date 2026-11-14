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
// build information names one, and the current configuration, which the
// rollback's check gives that program to read.
type rollbackPreview struct {
	DryRun        bool   `json:"dry_run"`
	Program       string `json:"program"`
	Version       string `json:"version,omitempty"`
	VersionUnread string `json:"version_unread,omitempty"`
	Configuration string `json:"configuration"`
	NextAction    string `json:"next_action,omitempty"`
}

// newRollbackCommand constructs `aigw self rollback`, which activates the
// retained previous portable program and moves no credential. Homebrew
// installs an earlier release of a program that it owns.
func newRollbackCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode, dryRun bool
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Restore the retained previous program, not the configuration",
		Long: "Activate the program that the last update or installation retained.\n" +
			"Its check gives the previous program the current configuration, and\n" +
			"a configuration that it cannot read blocks the rollback without\n" +
			"changing files.",
		Example: "aigw self rollback --dry-run\naigw self rollback\naigw self rollback --json",
		Args:    cobra.NoArgs,
		RunE: func(ctx *cobra.Command, _ []string) error {
			if err := program.RequirePortableOwnership(runtime.Executable); err != nil {
				if errors.Is(err, program.ErrHomebrewManaged) {
					return homebrewManaged("Install the earlier release with Homebrew.", "No program file or credential changed.", err)
				}
				return err
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
			result, err := runtime.Updater.Rollback(ctx.Context(), config)
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
// running it, because a preview runs no other program, and the current
// configuration, which the rollback's check gives it.
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
