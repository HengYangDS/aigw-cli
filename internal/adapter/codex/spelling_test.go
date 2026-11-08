package codex

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/terminal"
)

// A configured target that names the default home in another spelling is the
// same file, so converging the targets keeps one of them and the plan does
// not refuse a duplicate. Windows writes / where Codex's default home has \.
func TestConvergedTargetsKeepOneSpellingOfAFile(t *testing.T) {
	home := t.TempDir()
	defaultHome := filepath.Join(home, ".codex", "config.toml")
	discovered := discovery.Result{Surfaces: []discovery.Surface{{ID: string(HomeDefault), ConfigPath: defaultHome, AutoManaged: true}}}
	spellings := []string{
		home + "//.codex/config.toml",
		home + "/./.codex/config.toml",
		home + "/.codex/../.codex/config.toml",
		filepath.ToSlash(defaultHome),
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		spellings = append(spellings, filepath.Join(home, ".CODEX", "Config.toml"))
	}
	for _, configured := range spellings {
		targets := convergedTargets(discovered, []string{configured})
		if len(targets) != 1 {
			t.Fatalf("%s converged into %q", configured, targets)
		}
		refs, err := targetRefs(discovered, targets, "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := normalizeCodexTargets(refs, true); err != nil {
			t.Fatalf("%s: %v", configured, err)
		}
	}
	other := filepath.Join(home, "other", "config.toml")
	if targets := convergedTargets(discovered, []string{other}); len(targets) != 2 {
		t.Fatalf("another file converged into %q", targets)
	}
}

// Two targets that name one file and reach the plan anyway are refused with
// a fix that names each file once, not a check that would refuse again.
func TestADuplicatedTargetNamesItsFix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	_, err := normalizeCodexTargets([]TargetRef{HomeTarget(path), HomeTarget(filepath.Dir(path) + "//config.toml")}, true)
	failed, atTarget := errors.AsType[*adapter.TargetError](err)
	problem := terminal.ProblemFor(err)
	if !atTarget || !strings.Contains(problem.Fix, "aigw client edit codex --target <file>") {
		t.Fatalf("the refusal = %#v, %+v", failed, problem)
	}
}
