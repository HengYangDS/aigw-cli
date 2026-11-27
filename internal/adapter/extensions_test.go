package adapter_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

var (
	appleSilicon = discovery.Host{GOOS: "darwin", GOARCH: "arm64"}
	linuxIntel   = discovery.Host{GOOS: "linux", GOARCH: "amd64"}
	linuxArm     = discovery.Host{GOOS: "linux", GOARCH: "arm64"}
)

// install puts a fake installed version of an extension into directory: its
// entry, with program inside it when program is not empty.
func install(t *testing.T, directory, entry, program string) string {
	t.Helper()
	root := filepath.Join(directory, entry)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if program == "" {
		return ""
	}
	path := filepath.Join(root, filepath.FromSlash(program))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("public program fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// extensionProgram resolves the program of a client that declares the
// extensions directory directory, on host.
func extensionProgram(t *testing.T, client, directory string, host discovery.Host) (adapter.Program, *adapter.StopError) {
	t.Helper()
	cfg := domain.NewConfig(clients.Admission())
	cfg.DeclareProgram(client, "", directory)
	return adapter.ClientProgram(cfg, client, discovery.Result{Executables: map[string]string{client: "/discovered/" + client}}, host)
}

// The program is the one inside the highest installed version for the host,
// at the path that the client's extension declares for that host, and never
// the one that discovery finds.
func TestAnExtensionsDirectoryResolvesToTheHighestVersionForTheHost(t *testing.T) {
	for name, test := range map[string]struct {
		client  string
		host    discovery.Host
		entries map[string]string
		want    string
	}{
		"Codex, two versions side by side on macOS": {
			client: "codex", host: appleSilicon, want: "openai.chatgpt-26.10.0-darwin-arm64",
			entries: map[string]string{
				"openai.chatgpt-26.5.1-darwin-arm64":  "bin/macos-aarch64/codex",
				"openai.chatgpt-26.10.0-darwin-arm64": "bin/macos-aarch64/codex",
			},
		},
		"Codex on Linux x64": {
			client: "codex", host: linuxIntel, want: "openai.chatgpt-26.10.0-linux-x64",
			entries: map[string]string{"openai.chatgpt-26.10.0-linux-x64": "bin/linux-x86_64/codex"},
		},
		"Codex on Linux arm64": {
			client: "codex", host: linuxArm, want: "openai.chatgpt-26.10.0-linux-arm64",
			entries: map[string]string{"openai.chatgpt-26.10.0-linux-arm64": "bin/linux-aarch64/codex"},
		},
		"Claude Code without a platform in its name": {
			client: "claude", host: appleSilicon, want: "anthropic.claude-code-2.1.0",
			entries: map[string]string{
				"anthropic.claude-code-2.0.9": "resources/native-binary/claude",
				"anthropic.claude-code-2.1.0": "resources/native-binary/claude",
			},
		},
		"versions for other platforms are ignored": {
			client: "codex", host: appleSilicon, want: "openai.chatgpt-26.5.1-darwin-arm64",
			entries: map[string]string{
				"openai.chatgpt-26.5.1-darwin-arm64": "bin/macos-aarch64/codex",
				"openai.chatgpt-27.0.0-darwin-x64":   "bin/macos-x86_64/codex",
				"openai.chatgpt-27.0.0-linux-x64":    "bin/linux-x86_64/codex",
			},
		},
		"the version built for the host wins a tie": {
			client: "codex", host: appleSilicon, want: "openai.chatgpt-26.10.0-darwin-arm64",
			entries: map[string]string{
				"openai.chatgpt-26.10.0":              "bin/macos-aarch64/codex",
				"openai.chatgpt-26.10.0-darwin-arm64": "bin/macos-aarch64/codex",
			},
		},
		"other extensions and other entries are ignored": {
			client: "codex", host: appleSilicon, want: "openai.chatgpt-26.5.1-darwin-arm64",
			entries: map[string]string{
				"openai.chatgpt-26.5.1-darwin-arm64": "bin/macos-aarch64/codex",
				"openai.chatgpt-helper-30.0.0":       "bin/macos-aarch64/codex",
				"openai.chatgpt-latest":              "bin/macos-aarch64/codex",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			var want string
			for entry, program := range test.entries {
				path := install(t, directory, entry, program)
				if entry == test.want {
					want = path
				}
			}
			// A file named like a version is no installed version.
			if err := os.WriteFile(filepath.Join(directory, "openai.chatgpt-99.0.0-darwin-arm64"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			got, stop := extensionProgram(t, test.client, directory, test.host)
			if stop != nil || got.Path != want || got.Discovered {
				t.Fatalf("ClientProgram() = %+v, %v; want %s", got, stop, want)
			}
		})
	}
}

// Version parts compare by their leading number, at any length, and then by
// the text after it: a release is above its prerelease, a prerelease of a
// higher number is above a lower release, a part with a number is above one
// without, other texts compare byte by byte, and when every common part is
// equal, the version with more parts is the higher.
func TestExtensionVersionsCompareByTheirParts(t *testing.T) {
	for _, test := range []struct{ lower, higher string }{
		{"1.9.0", "1.10.0"},
		{"2.0.9", "10.0.0"},
		{"1.2.3-beta", "1.2.3"},
		{"1.2.9", "1.2.10-beta"},
		{"1.2.3-alpha", "1.2.3-beta"},
		{"1.beta", "1.0"},
		{"1.2", "1.2.0"},
		{"1.2.3", "1.2.99999999999999999999999"},
		{"1.02", "1.3"},
	} {
		t.Run(test.lower+" < "+test.higher, func(t *testing.T) {
			directory := t.TempDir()
			install(t, directory, "anthropic.claude-code-"+test.lower, "resources/native-binary/claude")
			install(t, directory, "anthropic.claude-code-"+test.higher, "resources/native-binary/claude")
			if got, stop := extensionProgram(t, "claude", directory, appleSilicon); stop != nil || got.Version != test.higher {
				t.Fatalf("ClientProgram() = %+v, %v; want version %s", got, stop, test.higher)
			}
		})
	}
}

// A version that the directory's .obsolete file lists is left out; without
// the file, none is.
func TestAnObsoleteVersionIsLeftOut(t *testing.T) {
	directory := t.TempDir()
	older := install(t, directory, "openai.chatgpt-26.5.1-darwin-arm64", "bin/macos-aarch64/codex")
	install(t, directory, "openai.chatgpt-26.10.0-darwin-arm64", "bin/macos-aarch64/codex")
	if err := os.WriteFile(filepath.Join(directory, ".obsolete"), []byte(`{"openai.chatgpt-26.10.0-darwin-arm64":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, stop := extensionProgram(t, "codex", directory, appleSilicon); stop != nil || got.Path != older || got.Version != "26.5.1" {
		t.Fatalf("ClientProgram() = %+v, %v; want %s", got, stop, older)
	}
}

// Each case in which the directory resolves to no program stops the client
// with its cause and the one fix, and never falls back to a lower version or
// to the program that discovery finds.
func TestAnExtensionsDirectoryWithoutAProgramStopsTheClient(t *testing.T) {
	for name, test := range map[string]struct {
		client  string
		prepare func(t *testing.T, directory string)
		problem string
		version string
		cause   bool
	}{
		"no installed version": {
			client: "codex", prepare: func(*testing.T, string) {},
			problem: "The extensions directory holds no installed Codex extension (openai.chatgpt) for this host",
		},
		"only versions for other platforms": {
			client: "codex",
			prepare: func(t *testing.T, directory string) {
				install(t, directory, "openai.chatgpt-26.10.0-linux-x64", "bin/linux-x86_64/codex")
			},
			problem: "The extensions directory holds no installed Codex extension (openai.chatgpt) for this host",
		},
		"a directory that does not exist": {
			client: "claude",
			prepare: func(t *testing.T, directory string) {
				if err := os.Remove(directory); err != nil {
					t.Fatal(err)
				}
			},
			problem: "Claude Code's extensions directory does not exist",
		},
		"the highest version without its program": {
			client: "codex", version: "26.10.0",
			prepare: func(t *testing.T, directory string) {
				install(t, directory, "openai.chatgpt-26.5.1-darwin-arm64", "bin/macos-aarch64/codex")
				install(t, directory, "openai.chatgpt-26.10.0-darwin-arm64", "")
			},
			problem: "Version 26.10.0 of the Codex extension holds no program that can run at bin/macos-aarch64/codex",
		},
		"a .obsolete file that is no JSON": {
			client: "codex", cause: true,
			prepare: func(t *testing.T, directory string) {
				install(t, directory, "openai.chatgpt-26.10.0-darwin-arm64", "bin/macos-aarch64/codex")
				if err := os.WriteFile(filepath.Join(directory, ".obsolete"), []byte("{"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			problem: "Codex's extensions directory has a .obsolete file that AIGW cannot read",
		},
		"a .obsolete file that is no JSON object": {
			client: "codex", cause: true,
			prepare: func(t *testing.T, directory string) {
				if err := os.WriteFile(filepath.Join(directory, ".obsolete"), []byte("null"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			problem: "Codex's extensions directory has a .obsolete file that AIGW cannot read",
		},
		"a .obsolete file that cannot be read": {
			client: "codex", cause: true,
			prepare: func(t *testing.T, directory string) {
				if err := os.Mkdir(filepath.Join(directory, ".obsolete"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			problem: "Codex's extensions directory has a .obsolete file that AIGW cannot read",
		},
		"a client without an editor extension": {
			client: "hermes", prepare: func(*testing.T, string) {},
			problem: "Hermes has no editor extension, so it cannot declare an extensions directory",
		},
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			test.prepare(t, directory)
			got, stop := extensionProgram(t, test.client, directory, appleSilicon)
			spec, _ := clients.Admission().Spec(test.client)
			if stop == nil || got.Path != "" || got.Version != test.version || stop.Error() != test.problem || stop.Client() != test.client || stop.NextAction() != adapter.ProgramRepair(spec) {
				t.Fatalf("ClientProgram() = %+v, %v; want the stop %q", got, stop, test.problem)
			}
			if cause := errors.Unwrap(stop); test.cause && (cause == nil || !strings.Contains(cause.Error(), filepath.Join(directory, ".obsolete"))) {
				t.Fatalf("the stop's cause %v does not name the .obsolete file", cause)
			}
		})
	}
}

// Discovery passes over a program inside an installed version of the client's
// extension, which AIGW never adopts, and finds the next one on the search
// path, or none when the extension's is the only one.
func TestDiscoveryPassesOverAProgramInsideAnExtension(t *testing.T) {
	codex, _ := clients.Admission().Spec("codex")
	inside := install(t, t.TempDir(), codex.Extension.ID+"-26.10.0", "bin/codex")
	outside := install(t, t.TempDir(), "bin", "codex")
	for name, test := range map[string]struct {
		path []string
		want string
	}{
		"only the extension's program":   {[]string{filepath.Dir(inside)}, ""},
		"the extension's program first":  {[]string{filepath.Dir(inside), filepath.Dir(outside)}, outside},
		"another program before the one": {[]string{filepath.Dir(outside), filepath.Dir(inside)}, outside},
	} {
		t.Run(name, func(t *testing.T) {
			source := discovery.System{GOOS: "darwin", Home: t.TempDir(), Path: strings.Join(test.path, string(os.PathListSeparator))}
			if got := adapter.NewDiscoverer(clients.Registry(), source).Discover("codex").Executable("codex"); got != test.want {
				t.Fatalf("Discover(codex) = %q, want %q", got, test.want)
			}
		})
	}
}
