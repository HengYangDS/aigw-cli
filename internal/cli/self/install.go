package self

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/program"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// installSchemaVersion is the version of `aigw self install`'s JSON document,
// which continues from `aigw install`'s 1.
const installSchemaVersion = 2

// installDocument is `aigw self install`'s JSON document. PATH is unchanged,
// so `aigw` may name another program and the next step stays prose. A preview
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

// newInstallCommand constructs `aigw self install`, which copies the running
// portable program to its installation target.
func newInstallCommand(runtime invocation.Context) *cobra.Command {
	target := runtime.InstallTarget
	var jsonMode, dryRun bool
	command := &cobra.Command{
		Use:     "install",
		Short:   "Install this portable program",
		Example: "aigw self install --dry-run\naigw self install\naigw self install --path ~/bin/aigw --json",
		// The target is the command's argument: a missing one, a folder and
		// a program file that Homebrew owns are refused before anything runs.
		Args: cobra.MatchAll(cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
			return admitInstallTarget(cmd, runtime, target)
		}),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRun {
				return previewInstall(cmd, runtime, target, jsonMode)
			}
			if err := program.Install(runtime.Executable, target); err != nil {
				return installFailure(cmd, runtime, target, err)
			}
			if jsonMode {
				return terminal.WriteDocument(runtime.Out, installSchemaVersion, true, installDocument{Path: target, NextDetail: installedNext})
			}
			render := invocation.Renderer(runtime)
			render.ProductTitle("Portable installation")
			render.Success("Installed " + invocation.GivenPath(runtime, target))
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

// admitInstallTarget refuses an installation target before anything runs:
// none at all, a program file that Homebrew owns, which Homebrew updates, and
// a folder at the program's path or its rollback copy's. A target that it
// cannot inspect is the installation's to classify, as it writes there.
func admitInstallTarget(cmd *cobra.Command, runtime invocation.Context, target string) error {
	if strings.TrimSpace(target) == "" {
		return errors.New("portable installation target is unavailable; name the program file with --path")
	}
	if err := program.RequirePortableOwnership(target); errors.Is(err, program.ErrHomebrewManaged) {
		return homebrewManaged("brew upgrade --cask aigw", "No program file changed.", err)
	}
	err := program.RefuseDirectories(target)
	if directory, refused := errors.AsType[*program.TargetDirectoryError](err); refused {
		return terminal.ProblemError(err.Error(), "", "", programFileFix(cmd, runtime, directory.Path), err)
	}
	return nil
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
	// Every path in the preview keeps the operator's own spelling, as Path and
	// RollbackPath do, so that a script compares it with the --path it gave.
	rollback := ""
	if plan.Rollback != "" {
		rollback = program.RollbackPath(target)
	}
	// The write creates a missing folder, so the preview says that it will.
	creates := ""
	if folder := filepath.Dir(target); !folderExists(folder) {
		creates = givenFolder(target, folder, os.IsPathSeparator)
	}
	if jsonMode {
		document := installDocument{DryRun: true, Path: target, Version: runtime.Version, Rollback: rollback, Unchanged: plan.Unchanged, CreatesFolder: creates, NextDetail: next}
		return terminal.WriteDocument(runtime.Out, installSchemaVersion, true, document)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Portable installation preview")
	rows := []terminal.Field{
		{Label: "Program file", Value: invocation.GivenPath(runtime, target)},
		{Label: "Version", Value: runtime.Version},
	}
	switch {
	case plan.Unchanged:
		rows = append(rows, terminal.Field{State: terminal.OK, Label: "Installed", Value: "Already this program; installing again only makes it executable"})
	case rollback != "":
		rows = append(rows, terminal.Field{Label: "Rollback copy", Value: invocation.GivenPath(runtime, rollback)})
	}
	if creates != "" {
		rows = append(rows, terminal.Field{Label: "Creates folder", Value: invocation.GivenPath(runtime, creates)})
	}
	r.Rows(rows...)
	r.Success("Preview did not write the program, a rollback copy or a directory")
	r.Text(next)
	return nil
}

// givenFolder spells folder, the folder of target that filepath derives, as
// the operator spelled it in target, as RollbackPath keeps the operator's
// separators: target up to its last separator, which isSeparator tells, when
// that prefix is already clean and names folder with every separator alike. A
// prefix with "." or ".." segments or doubled separators names another folder
// once a link precedes "..", and a volume root such as C:\ or \\?\C:\ is
// only a root with its separator, while C: is the drive's current folder, so
// both keep the derived folder.
func givenFolder(target, folder string, isSeparator func(uint8) bool) string {
	slashed := func(value string) string {
		spelled := []byte(value)
		for index := range spelled {
			if isSeparator(spelled[index]) {
				spelled[index] = '/'
			}
		}
		return string(spelled)
	}
	for index := len(target) - 1; index > 0; index-- {
		if !isSeparator(target[index]) {
			continue
		}
		given := target[:index]
		if clean := slashed(given); path.Clean(clean) == clean && clean == slashed(folder) {
			return given
		}
		break
	}
	return folder
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
	if access, atFile := errors.AsType[*os.PathError](err); atFile && access.Path == filepath.Clean(runtime.Executable) {
		return err
	}
	return terminal.GivenPathFailure(err, platform.PathContext{Home: invocation.Home(runtime.Dependencies)}, false)
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
