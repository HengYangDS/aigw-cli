// Package uninstall implements `aigw uninstall`, which withdraws AIGW's client
// projections and removes the portable program.
package uninstall

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/program"
	"aigw-cli/internal/terminal"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// schemaVersion is the version of `aigw uninstall`'s JSON document.
const schemaVersion = 1

// uninstallDocument is `aigw uninstall`'s JSON document: the program path,
// whether a program was there, and the clients whose projections were
// withdrawn. When the withdrawal was committed but the program could not be
// removed, problem names that, and the document's ok is false.
type uninstallDocument struct {
	DryRun           bool     `json:"dry_run,omitempty"`
	Path             string   `json:"path"`
	ProgramFound     bool     `json:"program_found"`
	WithdrawnClients []string `json:"withdrawn_clients"`
	// Removed is a preview's files that uninstalling would remove, and its
	// Preview what withdrawal would change.
	Removed []string `json:"removed,omitempty"`
	invocation.Preview
	Problem    string `json:"problem,omitempty"`
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// NewCommand constructs the command that removes only the installed portable executable and its owned backup.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var target string
	var jsonMode, dryRun bool
	command := &cobra.Command{
		Use:   "uninstall",
		Short: "Withdraw managed projections and remove a portable program",
		Long: "Withdraw AIGW-owned client projections, then remove the portable\n" +
			"executable and its rollback copy. This retains Account and Route data\n" +
			"and stored Tokens. Homebrew-managed copies must be removed with\n" +
			"Homebrew; this command does not manage them.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(target) == "" {
				target = runtime.Executable
			}
			if err := program.RequirePortableOwnership(target); err != nil {
				return err
			}
			// A directory at the program's path or at its rollback copy's
			// path is refused before any client is withdrawn.
			if err := program.RefuseDirectories(target); err != nil {
				return err
			}
			_, targetErr := os.Stat(target)
			if targetErr != nil && !errors.Is(targetErr, os.ErrNotExist) {
				return fmt.Errorf("inspect installed AIGW executable: %w", targetErr)
			}
			programFound := targetErr == nil
			_, statErr := os.Stat(runtime.Config.Path())
			if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
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
				title := "Portable program removal is incomplete"
				impact := "No AIGW configuration existed; no managed client withdrawal was needed. Tokens and versioned credential readers are retained."
				if statErr == nil {
					title = "Client withdrawal completed; program removal is incomplete"
					impact = "Managed client withdrawal is committed; configuration, Tokens and versioned credential readers are retained."
				}
				fix := "Inspect the selected target and retained files; after resolving file access, retry `uninstall --path <path>` from another verified AIGW executable."
				problem := invocation.Problem(runtime,
					title,
					"AIGW could not remove every selected portable program file.",
					impact, fix, err)
				if !jsonMode || statErr != nil {
					return problem
				}
				document := uninstallDocument{Path: target, ProgramFound: programFound, WithdrawnClients: withdrawn, Problem: title, NextDetail: fix}
				if err := terminal.WriteDocument(runtime.Out, schemaVersion, false, document); err != nil {
					return err
				}
				return terminal.Presented(problem)
			}
			if jsonMode {
				return terminal.WriteDocument(runtime.Out, schemaVersion, true,
					uninstallDocument{Path: target, ProgramFound: programFound, WithdrawnClients: withdrawn})
			}
			render := invocation.Renderer(runtime)
			render.ProductTitle("Portable uninstall")
			render.Success("Removed AIGW client projections, executable, and its single rollback copy")
			render.Text("Configuration and credential-store secrets were preserved.")
			render.Text("Versioned credential readers were retained for cached or other-installation callers.")
			return nil
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Show the client files and program files that uninstalling would change or remove without changing them")
	command.Flags().StringVar(&target, "path", "", "installed executable path; defaults to the running executable")
	command.Flags().BoolVar(&jsonMode, "json", false, "Write the removal as JSON")
	return command
}

// previewUninstall reports the clients whose projections uninstalling would
// withdraw, what withdrawal would change, and the program file and rollback
// copy that it would remove. Its apply command names the target, because
// another aigw on PATH would otherwise remove itself.
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
	if !cmd.Flags().Changed("path") && !cmd.Flags().Changed("target") {
		if err := cmd.Flags().Set("path", target); err != nil {
			return err
		}
	}
	apply := invocation.ApplyCommand(cmd, nil)
	if jsonMode {
		document.NextAction, document.NextDetail = terminal.NextSteps(apply)
		return terminal.WriteDocument(runtime.Out, schemaVersion, true, document)
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
