// Package install implements `aigw install`, which copies the running portable
// program to its installation target.
package install

import (
	"errors"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/program"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// schemaVersion is the version of `aigw install`'s JSON document.
const schemaVersion = 1

// installDocument is `aigw install`'s JSON document. PATH is unchanged, so
// `aigw` may name another program and the next step stays prose. A preview
// names the version that it would install, the rollback copy that it would
// keep, and whether the target already holds this program.
type installDocument struct {
	DryRun     bool   `json:"dry_run,omitempty"`
	Path       string `json:"path"`
	Version    string `json:"version,omitempty"`
	Rollback   string `json:"rollback,omitempty"`
	Unchanged  bool   `json:"unchanged,omitempty"`
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// installedNext is the next step after an installation: PATH is unchanged.
const installedNext = "PATH is unchanged. Use the installed path directly, or add its directory to PATH before running `aigw setup`."

// NewCommand constructs the portable executable installation command.
func NewCommand(runtime invocation.Context) *cobra.Command {
	target := runtime.InstallTarget
	var jsonMode, dryRun bool
	command := &cobra.Command{
		Use:   "install",
		Short: "Install this portable AIGW executable",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if strings.TrimSpace(target) == "" {
				return errors.New("portable installation target is unavailable")
			}
			if dryRun {
				return previewInstall(runtime, target, jsonMode)
			}
			if err := program.Install(runtime.Executable, target); err != nil {
				return err
			}
			if jsonMode {
				return terminal.WriteDocument(runtime.Out, schemaVersion, true, installDocument{Path: target, NextDetail: installedNext})
			}
			render := invocation.Renderer(runtime)
			render.ProductTitle("Portable installation")
			render.Success("Installed " + target)
			render.Text("PATH is unchanged. Use the installed path directly, or add its directory to PATH before running the command below.")
			render.Next("aigw setup")
			return nil
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Show the program file and rollback copy that installing would write without writing them")
	command.Flags().StringVar(&target, "path", target, "destination executable path")
	command.Flags().StringVar(&target, "target", target, "destination executable path; --path is the same value")
	command.MarkFlagsMutuallyExclusive("path", "target")
	_ = command.Flags().SetAnnotation("target", invocation.ReplacedBy, []string{"path"})
	command.Flags().BoolVar(&jsonMode, "json", false, "Write the installed path as JSON")
	return command
}

// previewInstall reports the program file that installing would write, as
// given, the version, and the rollback copy of a program that it would
// replace. Only this program installs itself, while `aigw` may name another,
// so the next step stays prose, as after an installation.
func previewInstall(runtime invocation.Context, target string, jsonMode bool) error {
	plan, err := program.PlanInstall(runtime.Executable, target)
	if err != nil {
		return err
	}
	next := invocation.ApplyWithThisProgram
	if plan.Unchanged {
		next = installedNext
	}
	rollback := ""
	if plan.Rollback != "" {
		rollback = program.RollbackPath(target)
	}
	if jsonMode {
		document := installDocument{DryRun: true, Path: target, Version: runtime.Version, Rollback: rollback, Unchanged: plan.Unchanged, NextDetail: next}
		return terminal.WriteDocument(runtime.Out, schemaVersion, true, document)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Portable installation preview")
	rows := []terminal.Field{
		{Label: "Program file", Value: target},
		{Label: "Version", Value: runtime.Version},
	}
	switch {
	case plan.Unchanged:
		rows = append(rows, terminal.Field{State: terminal.OK, Label: "Installed", Value: "Already this program; installing again only makes it executable"})
	case rollback != "":
		rows = append(rows, terminal.Field{Label: "Rollback copy", Value: rollback})
	}
	r.Rows(rows...)
	r.Success("Preview did not write the program, a rollback copy or a directory")
	r.Text(next)
	return nil
}
