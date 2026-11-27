package client

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// programOptions are the flags of enable and edit that declare a client's
// program: --program, the program's path, or --extensions, an editor's
// extensions directory that holds it, never both.
type programOptions struct {
	program, extensions string
}

// declares reports whether a flag declares the client's program.
func (options programOptions) declares() bool {
	return options.program != "" || options.extensions != ""
}

// hasExtension reports whether an editor extension ships the client's
// program, so that it can declare an extensions directory.
func hasExtension(spec domain.ClientSpec) bool { return spec.Extension.ID != "" }

// bindProgram adds the program flags to cmd.
func bindProgram(cmd *cobra.Command, options *programOptions, admission domain.Admission) {
	invocation.BindText(cmd, &options.program, "program", "The client's program `path`; it replaces --extensions")
	invocation.BindText(cmd, &options.extensions, "extensions",
		"An editor's extensions `directory`, for "+clientsWith(admission, hasExtension)+": the client uses the program of the newest installed version of its extension; it replaces --program")
}

// checkProgram checks the program flags before any configuration access: at
// most one of them, a non-empty path, and --extensions only for a client
// whose program an editor extension ships.
func checkProgram(cmd *cobra.Command, admission domain.Admission, spec domain.ClientSpec, options programOptions) error {
	path, changed := cmd.CommandPath(), cmd.Flags().Changed
	switch {
	case changed("program") && changed("extensions"):
		return fmt.Errorf("--program and --extensions are mutually exclusive; run `%s --help`", path)
	case changed("program") && strings.TrimSpace(options.program) == "":
		return fmt.Errorf("--program requires a non-empty path; run `%s --help`", path)
	case changed("extensions") && strings.TrimSpace(options.extensions) == "":
		return fmt.Errorf("--extensions requires a non-empty directory; run `%s --help`", path)
	case changed("extensions") && !hasExtension(spec):
		return fmt.Errorf("%s has no editor extension, so it names its program with --program; --extensions is for %s; run `%s --help`",
			spec.Label, clientsWith(admission, hasExtension), path)
	}
	return nil
}

// clientCommand is the command line `aigw client <verb> <client>` with flags,
// pairs of a flag and its value, or "" when a value would need quoting in the
// platform's shells.
func clientCommand(verb, client string, flags ...string) string {
	words := []string{"client", verb, client}
	for index := 1; index < len(flags); index += 2 {
		words = append(words, flags[index])
	}
	if platform.AIGWCommand(words...) == "" {
		return ""
	}
	return strings.Join(slices.Concat([]string{"aigw", "client", verb, client}, flags), " ")
}

// declaredProgram returns the declaration that the program flags of
// `aigw client <verb>` give, as absolute paths, or refuses it before anything
// is read or written: a --program that lies in an installed version of the
// client's extension, with the command that declares the same program by
// --extensions without its version, followed by after, the command's other
// words that it needs; a --program that is not a file that can run; and an
// --extensions directory that resolves to no program that can run, with the
// cause that a sync would give. Nothing is run.
func declaredProgram(runtime invocation.Context, spec domain.ClientSpec, options programOptions, verb string, after ...string) (programOptions, error) {
	switch {
	case options.program != "":
		program, _ := filepath.Abs(options.program)
		if directory, inside := adapter.ExtensionsDirectoryOf(spec, program); inside {
			fix := clientCommand(verb, spec.ID, slices.Concat([]string{"--extensions", directory}, after)...)
			if fix == "" {
				fix = "aigw client " + verb + " " + spec.ID + " --extensions <dir>"
			}
			return programOptions{}, terminal.ProblemError("--program "+program+" lies in one installed version of the "+runtime.Clients.Admission().ProductName(spec.ID)+" extension, which the next version replaces",
				"", terminal.UnchangedImpact, fix, nil)
		}
		if runnable, err := discovery.ExecutableAvailable(program); err != nil || !runnable {
			return programOptions{}, fmt.Errorf("--program %s is not a file that can run; run `aigw client list`, which shows where each client is installed", program)
		}
		return programOptions{program: program}, nil
	case options.extensions != "":
		extensions, _ := filepath.Abs(options.extensions)
		probe := domain.NewConfig(runtime.Clients.Admission())
		probe.DeclareProgram(spec.ID, "", extensions)
		if _, stop := adapter.ClientProgram(probe, spec.ID, discovery.Result{}, runtime.Host); stop != nil {
			return programOptions{}, terminal.ProblemError(stop.Error(), "", terminal.UnchangedImpact, stop.NextAction(), stop)
		}
		return programOptions{extensions: extensions}, nil
	}
	return options, nil
}
