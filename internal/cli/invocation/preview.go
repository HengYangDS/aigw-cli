package invocation

import (
	"regexp"
	"runtime"
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/app"
	"aigw-cli/internal/app/synchronization"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// previewSuccess ends every configuration preview's human output.
const previewSuccess = "Preview did not write configuration, credentials or client files"

// applyInProse continues a preview whose command line would need quoting.
const applyInProse = "Run the same command without --dry-run to apply this plan"

// ReplacedBy annotates a flag with the spelling that replaces it, which a
// preview's apply command uses.
const ReplacedBy = "aigw_replaced_by"

// Preview is what applying a configuration change would do beyond the
// configuration itself: the client files that it would change and the
// credential reader that it would install. A command's preview document
// embeds it.
type Preview struct {
	Targets              []adapter.ProjectionPlan      `json:"targets,omitempty"`
	CredentialEntrypoint *app.CredentialEntrypointPlan `json:"credential_entrypoint,omitempty"`
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

// WithEntrypoint completes a projection with the credential reader that a
// commit would prepare for its clients, without checking any Token.
func WithEntrypoint(runtime Context, after domain.Config, projection synchronization.Projection) (Preview, error) {
	preview := Preview{Targets: projection.Targets}
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

// ApplyCommand returns the continuation of a preview: the same command line
// without --dry-run and --json, with each replaced flag in its replacement's
// spelling, or prose when a word would need quoting, because no one quoting
// runs in every shell of a platform.
func ApplyCommand(cmd *cobra.Command, args []string) string {
	words := append(strings.Fields(cmd.CommandPath()), args...)
	isPlain := true
	for _, arg := range args {
		isPlain = isPlain && plain(arg)
	}
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if flag.Name == "dry-run" || flag.Name == "json" {
			return
		}
		name := flag.Name
		if replacement := flag.Annotations[ReplacedBy]; len(replacement) == 1 {
			name = replacement[0]
		}
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
	if !isPlain {
		return applyInProse
	}
	return strings.Join(words, " ")
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
	r.Section("Projection targets")
	rows := []terminal.Field{{State: terminal.OK, Label: "Projection", Value: "No client configuration needs changing"}}
	if len(preview.Targets) > 0 {
		rows = make([]terminal.Field, 0, len(preview.Targets))
		for _, plan := range preview.Targets {
			rows = append(rows, terminal.Field{Label: plan.Target, Value: plan.Action})
		}
	}
	r.Rows(rows...)
	r.Success(previewSuccess)
	r.Next(apply)
}
