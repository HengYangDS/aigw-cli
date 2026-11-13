package projection

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

// TestGitLabMacOSMisePreflightBindsTheVerifiedExecutable runs the generated
// preflight against a stand-in executable. Comparing its text alone passed a
// step that printed its findings without stopping, and one that left the word
// `mise` bound to a login profile's shell function.
func TestGitLabMacOSMisePreflightBindsTheVerifiedExecutable(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		NativeDarwin gitLabJob `yaml:"native-darwin"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	identity := pinnedMacOSRunnerMise(t, root)
	step := pipeline.NativeDarwin.Script[0]
	if strings.Count(step, identity.Directory) != 2 || strings.Count(step, identity.Executable) != 1 {
		t.Fatalf("the preflight must name its directory twice and its digest once:\n%s", step)
	}
	admitted := identity.Version + " macos-arm64 (2026-11-11)"
	// A login profile that activates Mise defines this function; it calls the
	// host's shared executable whatever PATH says.
	const profile = "mise() { printf '%s\\n' 'shared Mise'; }\n"
	const use = "\nmise --version\n"
	bound := "Runner-owned Mise verified: " + admitted + "\n" + admitted + "\n"
	for _, test := range []struct {
		name, prefix, prints string
		present, tampered    bool
		exits                int
		want                 string
	}{
		{name: "verified", prints: admitted, present: true, want: bound},
		{name: "verified over a profile function", prefix: profile, prints: admitted, present: true, want: bound},
		// A function that the step cannot remove still owns the word, so the
		// step must stop instead of reporting a verified Mise.
		{name: "a function that cannot be removed", prefix: profile + "readonly -f mise\n", prints: admitted, present: true, want: "The word mise does not run the runner-owned Mise.\n"},
		{name: "missing", want: "Runner-owned Mise is missing.\n"},
		{name: "another digest", prints: admitted, present: true, tampered: true, want: "Runner-owned Mise digest differs from the admitted release.\n"},
		{name: "another version", prints: identity.Version + "0 macos-arm64 (2026-11-11)", present: true, want: "Runner-owned Mise version differs from the admitted release.\n"},
		{name: "does not run", present: true, exits: 3, want: "Runner-owned Mise does not run.\n"},
	} {
		for _, shell := range []string{"/bin/bash", "/bin/sh"} {
			t.Run(test.name+" in "+shell, func(t *testing.T) {
				directory := t.TempDir()
				digest := strings.Repeat("0", 64)
				if test.present {
					content := []byte("#!/bin/sh\nprintf '%s\\n' '" + test.prints + "'\n")
					if test.exits != 0 {
						content = fmt.Appendf(nil, "#!/bin/sh\nexit %d\n", test.exits)
					}
					if err := os.WriteFile(filepath.Join(directory, "mise"), content, 0o700); err != nil {
						t.Fatal(err)
					}
					if !test.tampered {
						digest = fmt.Sprintf("%x", sha256.Sum256(content))
					}
				}
				script := strings.ReplaceAll(strings.ReplaceAll(step, identity.Directory, directory), identity.Executable, digest)
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				command := exec.CommandContext(ctx, shell, "-c", test.prefix+script+use)
				command.Env = []string{"PATH=/usr/bin:/bin"}
				output, err := command.CombinedOutput()
				verified := strings.HasPrefix(test.name, "verified")
				if (err == nil) != verified || string(output) != test.want {
					t.Fatalf("preflight error = %v, output:\n%s\nwant success = %t, output:\n%s", err, output, verified, test.want)
				}
			})
		}
	}
}
