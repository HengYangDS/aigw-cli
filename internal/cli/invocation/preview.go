package invocation

import (
	"regexp"
	"runtime"
	"slices"
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/app"
	"aigw-cli/internal/app/synchronization"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// previewSuccess ends every configuration preview's human output.
const previewSuccess = "Preview did not write configuration, credentials or client files"

// applyInProse continues a preview whose command line would need quoting.
const applyInProse = "Run the same command without --dry-run to apply this plan"

// Preview is what applying a configuration change would do beyond the
// configuration itself: the client files that it would change and the
// credential reader that it would install. A command's preview document
// embeds it and names each file as change documents do; human output names
// each file by its full path.
type Preview struct {
	Files                []ClientFile                  `json:"client_files,omitempty"`
	CredentialEntrypoint *app.CredentialEntrypointPlan `json:"credential_entrypoint,omitempty"`
	plans                []adapter.ProjectionPlan
	followed             []synchronization.Followed
}

// Plans returns the projection plans of the client files that the preview
// names, for a command that renders its preview itself.
func (preview Preview) Plans() []adapter.ProjectionPlan { return preview.plans }

// Home is the home directory from which documents name client files: the
// observed host's, from which the client adapters locate them, else the
// environment's.
func Home(deps app.Dependencies) string {
	if deps.Host.Home != "" {
		return deps.Host.Home
	}
	return terminal.HomeDirectory(deps.Env)
}

// GivenPath spells a path that a success line names: inside home as ~/… with
// "/" on every platform, as a shell prompt does; any other path, including
// home itself, as configured.
func GivenPath(runtime Context, path string) string {
	if named, _ := platform.HomeTarget(path, Home(runtime.Dependencies)); named != "" {
		return named
	}
	return path
}

// RenderCredentialReader names the credential reader that a preview or a
// change installs, under its own section; it writes nothing without one.
func RenderCredentialReader(r *terminal.Renderer, runtime Context, reader *app.CredentialEntrypointPlan) {
	if reader == nil {
		return
	}
	r.Section("Credential reader")
	r.Text("Install " + GivenPath(runtime, reader.Path))
}

// ClientFile is one client file that a change writes or would write, as a
// command's change document names it: the client, the file relative to the
// home directory, absent outside it, and the snake_case action.
type ClientFile struct {
	Client string `json:"client"`
	File   string `json:"file,omitempty"`
	Action string `json:"action"`
	// Followed is the model that the client's own file selects and that the
	// change records as the client's selection, and Returns the command that
	// selects the client's earlier model again.
	Followed string `json:"followed_selection,omitempty"`
	Returns  string `json:"returns_with,omitempty"`
}

// ClientFiles names the client files of a preview as change documents do.
func ClientFiles(runtime Context, preview Preview) []ClientFile {
	return clientFiles(runtime, preview.plans, preview.followed)
}

// clientFiles names the client files of plans as documents do, relative to
// Home, each with the model that the change follows for its client.
func clientFiles(runtime Context, plans []adapter.ProjectionPlan, followed []synchronization.Followed) []ClientFile {
	home := Home(runtime.Dependencies)
	files := make([]ClientFile, 0, len(plans))
	for _, plan := range plans {
		file, _ := platform.HomeTarget(plan.Target, home)
		row := ClientFile{Client: plan.Client, File: file, Action: terminal.SnakeValue(plan.Action)}
		if index := slices.IndexFunc(followed, func(choice synchronization.Followed) bool { return choice.Client == plan.Client }); index >= 0 {
			row.Followed, row.Returns = followed[index].Model.String(), followed[index].Returns
		}
		files = append(files, row)
	}
	return files
}

// FollowedSelection says that a change follows the model that a client's own
// file selects, and names the command that returns to the earlier model.
func FollowedSelection(choice synchronization.Followed) string {
	return followedSentence(choice.Model.String(), choice.Product, choice.Returns)
}

// followedSentence is the one wording of a followed choice.
func followedSentence(model, product, returns string) string {
	return "Follows " + model + ", chosen in " + product + "; `" + returns + "` returns to the earlier model"
}

// followedDetails is the sentence for client under its first file's row, and
// nothing under its further files or for a client that was not followed.
func followedDetails(followed []synchronization.Followed, client string, said map[string]bool) []string {
	index := slices.IndexFunc(followed, func(choice synchronization.Followed) bool { return choice.Client == client })
	if index < 0 || said[client] {
		return nil
	}
	said[client] = true
	return []string{FollowedSelection(followed[index])}
}

// RenderClientFiles names the client files that a change writes, each by its
// client's label and the file, under one section; it writes nothing without
// any. A client whose own choice the change follows gets the sentence that
// says so under its first file.
func RenderClientFiles(r *terminal.Renderer, runtime Context, files []ClientFile) {
	if len(files) == 0 {
		return
	}
	r.Section("Client files")
	rows := make([]terminal.Field, 0, len(files))
	said := map[string]bool{}
	for _, file := range files {
		product := runtime.Clients.Admission().Label(file.Client)
		label := product
		if file.File != "" {
			label += " " + file.File
		}
		row := terminal.Field{Label: label, Value: file.Action}
		if file.Followed != "" && !said[file.Client] {
			said[file.Client] = true
			row.Details = []string{followedSentence(file.Followed, product, file.Returns)}
		}
		rows = append(rows, row)
	}
	r.Rows(rows...)
}

// PreviewChange previews a configuration transition as Commit would apply it.
func PreviewChange(runtime Context, before, after domain.Config) (Preview, error) {
	projection, err := runtime.Synchronizer().Preview(before, after)
	if err != nil {
		return Preview{}, err
	}
	return WithEntrypoint(runtime, after, projection)
}

// PreviewProjection previews a configuration transition as CommitProjection
// would apply it for the named clients, or for every client.
func PreviewProjection(runtime Context, before, after domain.Config, clientIDs ...string) (Preview, error) {
	projection, err := runtime.Synchronizer().PreviewProjection(before, after, clientIDs...)
	if err != nil {
		return Preview{}, err
	}
	return WithEntrypoint(runtime, after, projection)
}

// Planned is a projection's client files alone, without the credential
// reader: what a commit, which prepares the reader itself, writes.
func Planned(runtime Context, projection synchronization.Projection) Preview {
	preview := Preview{plans: projection.Targets, followed: projection.Followed}
	if len(projection.Targets) > 0 {
		preview.Files = clientFiles(runtime, projection.Targets, projection.Followed)
	}
	return preview
}

// WithEntrypoint completes a projection with the credential reader that a
// commit would prepare for its clients, without checking any Token.
func WithEntrypoint(runtime Context, after domain.Config, projection synchronization.Projection) (Preview, error) {
	preview := Planned(runtime, projection)
	if len(projection.Clients) == 0 {
		return preview, nil
	}
	entrypoint, err := app.PlanCredentialEntrypoint(runtime.Synchronizer(), after, projection.Clients...)
	preview.CredentialEntrypoint = entrypoint
	return preview, err
}

// plainWord matches a word that the platform's shells pass through
// unchanged, so a command line of such words runs as written: it starts with
// a letter, digit, dot or slash. Windows shells also pass a backslash
// through, which a path holds there, and expand a percent sign.
var plainWord = func() *regexp.Regexp {
	first, rest := `A-Za-z0-9._/`, `A-Za-z0-9._/:@+=~`
	if runtime.GOOS == "windows" {
		first, rest = first+`\\`, rest+`\\`
	} else {
		rest += `%`
	}
	return regexp.MustCompile(`^[` + first + `][` + rest + `-]*$`)
}()

// shellTilde finds a tilde that a shell would expand after an equals sign or
// a colon.
var shellTilde = regexp.MustCompile(`[=:]~`)

func plain(word string) bool {
	return plainWord.MatchString(word) && !shellTilde.MatchString(word)
}

// Command returns the aigw command line of words, or "" when a word would
// need quoting in the platform's shells.
func Command(words ...string) string {
	if !slices.ContainsFunc(words, func(word string) bool { return !plain(word) }) {
		return strings.Join(append([]string{"aigw"}, words...), " ")
	}
	return ""
}

// ApplyCommand returns the continuation of a preview: the same command line
// without --dry-run and --json, or prose when a word would need quoting,
// because no one quoting runs in every shell of a platform.
func ApplyCommand(cmd *cobra.Command, args []string) string {
	line, isPlain := commandLine(cmd, args, "dry-run", "json")
	if !isPlain {
		return applyInProse
	}
	return line
}

// Rerun returns cmd's command line as it was given, without --json and the
// replaced flags, and with the added words after it, for a fix that sets
// flags of the command. It returns "" when a given word would need quoting.
func Rerun(cmd *cobra.Command, args, replaced []string, added ...string) string {
	line, isPlain := commandLine(cmd, args, append([]string{"json"}, replaced...)...)
	if !isPlain {
		return ""
	}
	return strings.Join(append([]string{line}, added...), " ")
}

// commandLine rebuilds cmd's command line from its arguments and the flags
// that it was given, except omitted ones, and reports whether every word
// runs as written in the platform's shells.
func commandLine(cmd *cobra.Command, args []string, omitted ...string) (string, bool) {
	words := append(strings.Fields(cmd.CommandPath()), args...)
	isPlain := true
	for _, arg := range args {
		isPlain = isPlain && plain(arg)
	}
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if slices.Contains(omitted, flag.Name) {
			return
		}
		name := flag.Name
		if flag.Value.Type() == "bool" {
			if flag.Value.String() == "true" {
				words = append(words, "--"+name)
			} else {
				words = append(words, "--"+name+"=false")
			}
			return
		}
		values := []string{flag.Value.String()}
		if slice, ok := flag.Value.(pflag.SliceValue); ok {
			values = slice.GetSlice()
		}
		for _, value := range values {
			words = append(words, "--"+name, value)
			isPlain = isPlain && plain(value)
		}
	})
	return strings.Join(words, " "), isPlain
}

// ApplyWithThisProgram continues a preview of a command that changes the
// running program's own file. `aigw` on PATH may name another program, so the
// step is prose.
const ApplyWithThisProgram = "Run the same command with this same program, without --dry-run, to apply this plan; `aigw` on PATH may be another program."

// RenderPreview ends a configuration preview's human output: the credential
// reader that applying it would install, the client files that it would
// change, the confirmation that nothing was written, and how to apply it.
func RenderPreview(r *terminal.Renderer, preview Preview, apply string) {
	if preview.CredentialEntrypoint != nil {
		r.Section("Credential entrypoint")
		r.Text("Install " + preview.CredentialEntrypoint.Path)
	}
	r.Section("Client files")
	if len(preview.plans) == 0 {
		r.Text("No client file needs changing")
	} else {
		rows := make([]terminal.Field, 0, len(preview.plans))
		said := map[string]bool{}
		for _, plan := range preview.plans {
			rows = append(rows, terminal.Field{Label: plan.Target, Value: plan.Action, Details: followedDetails(preview.followed, plan.Client, said)})
		}
		r.Rows(rows...)
	}
	r.Success(previewSuccess)
	r.Next(apply)
}
