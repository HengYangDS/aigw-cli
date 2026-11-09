// Package install implements `aigw install`, which copies the running portable
// program to its installation target.
package install

import (
	"errors"
	"os"
	"path/filepath"
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
	DryRun    bool   `json:"dry_run,omitempty"`
	Path      string `json:"path"`
	Version   string `json:"version,omitempty"`
	Rollback  string `json:"rollback,omitempty"`
	Unchanged bool   `json:"unchanged,omitempty"`
	// CreatesFolder is the program file's folder, which installing creates,
	// when it does not exist yet.
	CreatesFolder string `json:"creates_folder,omitempty"`
	NextDetail    string `json:"next_detail,omitempty"`
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(target) == "" {
				return errors.New("portable installation target is unavailable")
			}
			if dryRun {
				return previewInstall(cmd, runtime, target, jsonMode)
			}
			if err := program.Install(runtime.Executable, target); err != nil {
				return installFailure(cmd, runtime, target, err)
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
	command.Flags().BoolVar(&jsonMode, "json", false, "Write the installed path as JSON")
	return command
}

// previewInstall reports the program file that installing would write, as
// given, the version, and the rollback copy of a program that it would
// replace. Only this program installs itself, while `aigw` may name another,
// so the next step stays prose, as after an installation.
func previewInstall(cmd *cobra.Command, runtime invocation.Context, target string, jsonMode bool) error {
	plan, err := program.PlanInstall(runtime.Executable, target)
	if err != nil {
		return installFailure(cmd, runtime, target, err)
	}
	next := invocation.ApplyWithThisProgram
	if plan.Unchanged {
		next = installedNext
	}
	rollback := ""
	if plan.Rollback != "" {
		rollback = program.RollbackPath(target)
	}
	// The write creates a missing folder, so the preview says that it will.
	creates := ""
	if folder := filepath.Dir(target); !folderExists(folder) {
		creates = folder
	}
	if jsonMode {
		document := installDocument{DryRun: true, Path: target, Version: runtime.Version, Rollback: rollback, Unchanged: plan.Unchanged, CreatesFolder: creates, NextDetail: next}
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
	if creates != "" {
		rows = append(rows, terminal.Field{Label: "Creates folder", Value: creates})
	}
	r.Rows(rows...)
	r.Success("Preview did not write the program, a rollback copy or a directory")
	r.Text(next)
	return nil
}

// folderExists reports whether path names a folder.
func folderExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// installFailure gives a failed installation a fix that acts on its cause: a
// local check inspects neither the program file nor its folder, and fails in a
// home without a configuration. A running program that cannot read itself has
// no target that would help, so that failure stays as it is.
func installFailure(cmd *cobra.Command, runtime invocation.Context, target string, err error) error {
	if directory, refused := errors.AsType[*program.TargetDirectoryError](err); refused {
		return terminal.ProblemError(err.Error(), "", "", programFileFix(cmd, runtime, directory.Path), err)
	}
	if link, atLink := errors.AsType[*program.TargetLinkError](err); atLink {
		return terminal.ProblemError("AIGW cannot install there: a link leads nowhere where the program's folder belongs", "", "Nothing was written.", link.NextAction(), err)
	}
	if access, atFile := errors.AsType[*os.PathError](err); atFile && access.Path == filepath.Clean(runtime.Executable) {
		return err
	}
	return terminal.FileAccessFailure(err, target, invocation.Home(runtime.Dependencies))
}

// programFileFix is the fix of a --path that names a directory: this command
// line with the program file inside that directory, or a sentence when no
// command line can say so or that file is a directory too. It stays prose, as
// after an installation, because `aigw` on PATH may be another program.
func programFileFix(cmd *cobra.Command, runtime invocation.Context, directory string) string {
	file := filepath.Join(directory, filepath.Base(runtime.InstallTarget))
	if info, err := os.Stat(file); err == nil && info.IsDir() {
		return "Run the same command with --path naming a program file; " + file + " is a directory too."
	}
	if line := invocation.Rerun(cmd, nil, []string{"path"}, "--path", file); line != "" && invocation.Command(file) != "" {
		return "Run `" + line + "` with this same program; `aigw` on PATH may be another program."
	}
	return "Run the same command with --path naming the program file " + file + "."
}
