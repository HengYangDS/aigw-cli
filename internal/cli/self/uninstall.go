package self

import (
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

// uninstallSchemaVersion is the version of `aigw self uninstall`'s JSON
// document, which continues from `aigw uninstall`'s 1.
const uninstallSchemaVersion = 2

// uninstallDocument is `aigw self uninstall`'s JSON document: the program
// path, whether a program was there, and the clients whose AIGW settings were
// removed. When that removal was committed but the program could not be
// removed, problem names that, and the document's ok is false.
type uninstallDocument struct {
	DryRun           bool     `json:"dry_run,omitempty"`
	Path             string   `json:"path"`
	ProgramFound     bool     `json:"program_found"`
	WithdrawnClients []string `json:"withdrawn_clients"`
	// Removed is a preview's files that uninstalling would remove, and its
	// Preview what removing the client settings would change.
	Removed []string `json:"removed,omitempty"`
	invocation.Preview
	Problem    string `json:"problem,omitempty"`
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// admitUninstallTarget refuses a program that Homebrew owns, and a directory
// at the program's path or at its rollback copy's path, before any client
// changes. The fix for a directory names no file: uninstalling removes the
// client settings even when no program is at the path, so it cannot guess.
func admitUninstallTarget(target string) error {
	if err := program.RequirePortableOwnership(target); err != nil {
		if !errors.Is(err, program.ErrHomebrewManaged) {
			return err
		}
		return homebrewManaged("brew uninstall --cask aigw", "No client file or program file changed; clients that AIGW configured still call it.", err)
	}
	err := program.RefuseDirectories(target)
	if directory, refused := errors.AsType[*program.TargetDirectoryError](err); refused {
		return terminal.ProblemError(err.Error(), "", "", "Run the same command with --path naming the program file inside "+directory.Path+".", err)
	}
	return err
}

// newUninstallCommand constructs `aigw self uninstall`, which removes AIGW's
// settings from the client files, then the portable program and its rollback
// copy.
func newUninstallCommand(runtime invocation.Context) *cobra.Command {
	var target string
	var jsonMode, dryRun bool
	command := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove AIGW's client settings and this portable program",
		Long: "Remove AIGW's settings from the client files, then the portable program\n" +
			"and its rollback copy. Providers, models and stored Tokens stay. Homebrew\n" +
			"removes a program that it installed.",
		Example: "aigw self uninstall --dry-run\naigw self uninstall\naigw self uninstall --path ~/bin/aigw --json",
		// The target is the command's argument, refused before any client
		// changes.
		Args: cobra.MatchAll(cobra.NoArgs, func(*cobra.Command, []string) error {
			if strings.TrimSpace(target) == "" {
				target = runtime.Executable
			}
			return admitUninstallTarget(target)
		}),
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, targetErr := os.Stat(target)
			absentTarget, refusal := platform.MissingAt(target, platform.FileExpected, targetErr)
			if refusal != nil {
				return fmt.Errorf("inspect installed AIGW executable: %w", refusal)
			}
			if targetErr != nil && !absentTarget {
				return fmt.Errorf("inspect installed AIGW executable: %w", targetErr)
			}
			programFound := targetErr == nil
			_, statErr := os.Stat(runtime.Config.Path())
			if absent, refusal := platform.MissingAt(runtime.Config.Path(), platform.FileExpected, statErr); refusal != nil {
				return fmt.Errorf("inspect AIGW configuration: %w", refusal)
			} else if !absent && statErr != nil {
				return fmt.Errorf("inspect AIGW configuration: %w", statErr)
			}
			if dryRun {
				removable, err := program.PlanUninstall(target)
				if err != nil {
					return err
				}
				return previewUninstall(cmd, runtime, target, removable, statErr == nil, jsonMode)
			}
			withdrawn := []string{}
			if statErr == nil {
				before, err := runtime.Config.Load()
				if err != nil {
					return err
				}
				after := before.Clone()
				synchronizer := runtime.Synchronizer()
				if err := synchronizer.Withdraw(&after); err != nil {
					return err
				}
				if err := synchronizer.CommitProjection(cmd.Context(), before, after, "uninstall"); err != nil {
					return err
				}
				withdrawn = before.EnabledClientIDs()
			}
			if err := program.Uninstall(target); err != nil {
				return incompleteUninstall(runtime, target, programFound, withdrawn, statErr == nil, jsonMode, err)
			}
			if jsonMode {
				return terminal.WriteDocument(runtime.Out, uninstallSchemaVersion, true,
					uninstallDocument{Path: target, ProgramFound: programFound, WithdrawnClients: withdrawn})
			}
			render := invocation.Renderer(runtime)
			render.ProductTitle("Portable uninstall")
			render.Success("Removed AIGW's client settings, the program and its rollback copy")
			render.Text("The configuration and the stored Tokens were kept.")
			render.Text("Versioned credential readers were kept for running clients and other installations.")
			return nil
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Show the client files and program files that uninstalling would change or remove without changing them")
	command.Flags().StringVar(&target, "path", "", "installed executable path; defaults to the running executable")
	command.Flags().BoolVar(&jsonMode, "json", false, "Write the removal as JSON")
	return command
}

// incompleteUninstall reports a program that uninstalling could not remove,
// after it removed the client settings when a configuration named them.
func incompleteUninstall(runtime invocation.Context, target string, programFound bool, withdrawn []string, configured, jsonMode bool, err error) error {
	title := "Portable program removal is incomplete"
	impact := "No AIGW configuration existed, so no client file changed. Tokens and versioned credential readers are kept."
	if configured {
		title = "Client settings were removed; program removal is incomplete"
		impact = "AIGW's client settings are removed; the configuration, Tokens and versioned credential readers are kept."
	}
	fix := "Inspect the program file and the retained files; once you can change them, run `aigw self uninstall --path <path>` from another verified AIGW program."
	problem := terminal.ProblemError(title, "AIGW could not remove every selected portable program file.", impact, fix, err)
	if !jsonMode || !configured {
		return problem
	}
	document := uninstallDocument{Path: target, ProgramFound: programFound, WithdrawnClients: withdrawn, Problem: title, NextDetail: fix}
	if err := terminal.WriteDocument(runtime.Out, uninstallSchemaVersion, false, document); err != nil {
		return err
	}
	return terminal.Presented(problem)
}

// previewUninstall reports the clients whose AIGW settings uninstalling would
// remove, what that would change, and the program file and rollback copy that
// it would remove. Its apply command names the target, because another aigw
// on PATH would otherwise remove itself.
func previewUninstall(cmd *cobra.Command, runtime invocation.Context, target string, removed []string, configured, jsonMode bool) error {
	document := uninstallDocument{DryRun: true, Path: target, WithdrawnClients: []string{}, Removed: removed}
	document.ProgramFound = len(removed) > 0 && removed[0] == target
	if configured {
		before, err := runtime.Config.Load()
		if err != nil {
			return err
		}
		after := before.Clone()
		if err := runtime.Synchronizer().Withdraw(&after); err != nil {
			return err
		}
		if document.Preview, err = invocation.PreviewProjection(runtime, before, after); err != nil {
			return err
		}
		document.WithdrawnClients = before.EnabledClientIDs()
	}
	if !cmd.Flags().Changed("path") {
		if err := cmd.Flags().Set("path", target); err != nil {
			return err
		}
	}
	apply := invocation.ApplyCommand(cmd, nil)
	if jsonMode {
		document.NextAction, document.NextDetail = terminal.NextSteps(apply)
		return terminal.WriteDocument(runtime.Out, uninstallSchemaVersion, true, document)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Portable uninstall preview")
	rows := make([]terminal.Field, 0, len(removed)+1)
	for _, path := range removed {
		rows = append(rows, terminal.Field{Label: "Remove", Value: path})
	}
	if len(removed) == 0 {
		rows = append(rows, terminal.Field{State: terminal.Info, Label: "Program", Value: "No program file at " + target})
	}
	r.Rows(rows...)
	invocation.RenderPreview(r, document.Preview, apply)
	return nil
}
