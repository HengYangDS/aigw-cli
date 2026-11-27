package terminal

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/store"
)

// A refusal of the configuration file names why it is refused and, as its
// fix, the edit of the file: a command that would repair it loads the file
// first and stops the same way, so neither the evidence nor the fix names a
// command. The program's case shows the extensions directory to declare, the
// one path that its edit writes.
func TestARefusalOfTheFileIsFixedByItsEdit(t *testing.T) {
	for name, refused := range map[string]interface {
		error
		domain.FileRefusal
	}{
		"a program inside a version of an editor extension": &domain.ProgramDeclarationError{
			Client: "codex", Reason: "program lies in one installed version of its editor extension, which the next version replaces",
			Change: "replace `program` with `extensions = \"/home/user/.vscode/extensions\"`",
		},
		"extensions for a client without an extension": &domain.ProgramDeclarationError{
			Client: "hermes", Reason: "extensions cannot be declared, since Hermes has no editor extension",
			Change: "replace `extensions` with `program`, the path of its program",
		},
		"a forwarding that cannot carry the selection": &domain.ForwardingUpstreamError{
			Client: "codex", Provider: "team", Protocol: domain.ProtocolAnthropic,
		},
	} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", name, jsonMode), func(t *testing.T) {
				var out bytes.Buffer
				renderer := NewRenderer(&out, false)
				failure := &store.LoadError{Phase: store.LoadPhaseValidate, Err: domain.NewEntryError("client `"+name+"`", refused)}
				RenderError(renderer, LocalFailure("9.8.7", platform.PathContext{}, fmt.Errorf("load: %w", failure)), jsonMode)
				rendered := out.String()
				if renderer.Err() != nil {
					t.Fatal(renderer.Err())
				}
				for _, forbidden := range []string{"aigw client edit", "aigw use", "--direct"} {
					if strings.Contains(rendered, forbidden) {
						t.Fatalf("the diagnostic names %q, which cannot run while the file does not load: %q", forbidden, rendered)
					}
				}
				if jsonMode {
					return
				}
				for _, required := range []string{
					"stopped at the configuration validate stage: " + refused.Refusal(),
					"Edit the configuration file: " + refused.Edit(),
					"aigw config path",
				} {
					if !strings.Contains(rendered, required) {
						t.Fatalf("the diagnostic omits %q: %q", required, rendered)
					}
				}
			})
		}
	}
}
