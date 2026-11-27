package domain

import (
	"path/filepath"
	"strings"
	"testing"
)

// extensionsDirectory is an editor's extensions directory in the domain's
// fixtures; nothing reads it.
var extensionsDirectory = filepath.Join(string(filepath.Separator), "home", "user", ".vscode", "extensions")

// A program inside an installed version of the client's extension, for any
// platform, lies in that extensions directory; another program lies in none,
// and so does every program of a client without an extension.
func TestAProgramInsideAnInstalledVersionNamesItsExtensionsDirectory(t *testing.T) {
	admission := testAdmission()
	codex, _ := admission.Spec(ClientCodex)
	hermes, _ := admission.Spec(ClientHermes)
	for _, test := range []struct {
		spec   ClientSpec
		path   string
		want   string
		inside bool
	}{
		{codex, filepath.Join(extensionsDirectory, codex.Extension.ID+"-26.10.0-darwin-arm64", "bin", "macos-aarch64", "codex"), extensionsDirectory, true},
		{codex, filepath.Join(extensionsDirectory, codex.Extension.ID+"-26.10.0-linux-x64", "bin", "linux-x86_64", "codex"), extensionsDirectory, true},
		{codex, filepath.Join(extensionsDirectory, codex.Extension.ID+"-26.10.0", "codex"), extensionsDirectory, true},
		{codex, filepath.Join(string(filepath.Separator), "usr", "local", "bin", "codex"), "", false},
		{codex, filepath.Join(extensionsDirectory, codex.Extension.ID+"-latest", "codex"), "", false},
		{codex, filepath.Join(extensionsDirectory, codex.Extension.ID+"-helper-30.0.0", "codex"), "", false},
		{codex, "", "", false},
		{hermes, filepath.Join(extensionsDirectory, codex.Extension.ID+"-26.10.0-darwin-arm64", "hermes"), "", false},
	} {
		if got, inside := test.spec.ExtensionsDirectoryOf(test.path); got != test.want || inside != test.inside {
			t.Errorf("ExtensionsDirectoryOf(%s, %s) = %q, %v; want %q, %v", test.spec.ID, test.path, got, inside, test.want, test.inside)
		}
	}
}

// Loading refuses a program inside an installed version of the client's
// extension, naming the client, the field and the extensions declaration that
// replaces it, and refuses extensions for a client that has no editor
// extension, naming the client and the field. A program elsewhere, and the
// extensions of a client that has an extension, load.
func TestLoadingRefusesAVersionedProgramAndExtensionsWithoutAnExtension(t *testing.T) {
	codex, _ := testAdmission().Spec(ClientCodex)
	versioned := filepath.Join(extensionsDirectory, codex.Extension.ID+"-26.10.0-darwin-arm64", "bin", "macos-aarch64", "codex")
	declare := func(client, program, extensions string) Config {
		cfg := validConfig()
		binding := cfg.Clients[client]
		binding.Program, binding.Extensions = program, extensions
		cfg.Clients[client] = binding
		return cfg
	}
	for name, test := range map[string]struct {
		cfg  Config
		want []string
	}{
		"a program inside a version of the extension": {
			declare(ClientCodex, versioned, ""),
			[]string{`client "codex" program lies in one installed version of its editor extension`, `extensions = "` + extensionsDirectory + `"`},
		},
		"extensions for Hermes": {
			declare(ClientHermes, "", extensionsDirectory),
			[]string{`client "hermes" extensions cannot be declared`, "Hermes has no editor extension"},
		},
		"extensions for Claude Desktop": {
			declare(ClientClaudeDesktop, "", extensionsDirectory),
			[]string{`client "claude-desktop" extensions cannot be declared`, "Claude Desktop has no editor extension"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := test.cfg.Validate()
			for _, want := range test.want {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("Validate() = %v, want %q", err, want)
				}
			}
		})
	}
	for name, cfg := range map[string]Config{
		"a program outside the extension":    declare(ClientCodex, filepath.Join(string(filepath.Separator), "usr", "local", "bin", "codex"), ""),
		"Codex's extensions directory":       declare(ClientCodex, "", extensionsDirectory),
		"Claude Code's extensions directory": declare(ClientClaude, "", extensionsDirectory),
	} {
		if err := cfg.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v", name, err)
		}
	}
}
