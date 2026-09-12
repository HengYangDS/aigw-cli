package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRepositoryQualityScriptsRequireLocalDependencies(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repositoryRoot(t), "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	for _, name := range []string{"spec:check"} {
		t.Run(name, func(t *testing.T) {
			output, err := systemOutputRunner(command{Name: "node", Args: []string{"--run", name}})
			if err == nil || !bytes.Contains(output, []byte("MODULE_NOT_FOUND")) || !bytes.Contains(output, []byte("node_modules")) {
				t.Fatalf("missing local dependency: error=%v output=%s", err, output)
			}
		})
	}
}

func TestTOMLChecksExecuteInRequestedRepository(t *testing.T) {
	repository := repositoryRoot(t)
	caller := t.TempDir()
	root := filepath.Join(caller, "checkout with spaces")
	valid := `z = 2
a = 1
choices = [
  "z",
  "a",
]
record = {z = 2, a = 1}
`
	files := map[string]string{
		"mise.lock":                 valid,
		"tools/domain/config.toml":  valid,
		"internal/domain/test.toml": valid,
		"build/generated.toml":      "value = [\n",
	}
	for _, path := range []string{".gitignore", ".config/checks/toml/policy.toml"} {
		files[path] = string(readFile(t, filepath.Join(repository, filepath.FromSlash(path))))
	}
	for path, content := range files {
		destination := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	t.Chdir(caller)
	for _, test := range []struct {
		name, root, path, content string
		valid                     bool
	}{
		{"absolute root", root, "mise.lock", valid, true},
		{"relative root", filepath.Base(root), "mise.lock", valid, true},
		{"source fixture syntax", root, "internal/domain/test.toml", "value = [\n", false},
		{"tool policy format", root, "tools/domain/config.toml", "value=1\n", false},
		{"lock syntax", root, "mise.lock", "value = [\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			destination := filepath.Join(root, filepath.FromSlash(test.path))
			if err := os.WriteFile(destination, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.WriteFile(destination, []byte(valid), 0o600); err != nil {
					t.Error(err)
				}
			})
			var output []byte
			var inputs []string
			err := run([]string{"check-toml", test.root}, &bytes.Buffer{}, func(call command) error {
				if call.Dir != root || !slices.Equal(call.Env, []string{"RUST_LOG=warn"}) {
					t.Fatalf("TOML subprocess contract: %#v", call)
				}
				inputs = append(inputs, call.Args[slices.Index(call.Args, "--")+1:]...)
				var err error
				output, err = systemOutputRunner(call)
				return err
			})
			for _, path := range inputs {
				if !filepath.IsLocal(path) {
					t.Fatalf("TOML input must be relative to its checkout: %q", path)
				}
			}
			if test.valid {
				if err != nil || len(output) != 0 {
					t.Fatalf("valid TOML checkout: %v\n%s", err, output)
				}
				return
			}
			if err == nil || !strings.Contains(filepath.ToSlash(string(output)), test.path) {
				t.Fatalf("TOML carrier %q was not diagnosed: %v\n%s", test.path, err, output)
			}
			if source := readFile(t, filepath.Join(root, filepath.FromSlash(test.path))); string(source) != test.content {
				t.Fatalf("check changed source: %q", source)
			}
		})
	}
}

func TestFormattingCoversCurrentCarriersAndPreservesOwnedExclusions(t *testing.T) {
	repository := repositoryRoot(t)
	parent := t.TempDir()
	root := filepath.Join(parent, "checkout with spaces")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"package.json", ".gitignore", ".editorconfig", ".prettierignore"} {
		content, err := os.ReadFile(filepath.Join(repository, path))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	files := map[string]string{
		"document.md":                          "# Current\n\nRead the current instructions.\n",
		"docs/archive/current.md":              "# Current archive operation\n",
		".config/archive/metadata.json":        "{\n  \"version\": 1\n}\n",
		".config/fixture.yaml":                 "name: fixture\n",
		"openspec/changes/archive/old/spec.md": "#   Historic source\n",
		"build/tracked.json":                   "{\n  \"version\": 1\n}\n",
		"literal[1].json":                      "{\n  \"version\": 1\n}\n",
		"unsupported.txt":                      "not a prettier file",
		"build/generated.json":                 "{\"owned\":true}\n",
	}
	for path, content := range files {
		destination := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(parent, ".prettierrc.json"), []byte("{\"tabWidth\": 7}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", root, "add", "-f", "build/tracked.json").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
	}
	t.Setenv("GIT_INDEX_FILE", filepath.Join(repository, ".git", "foreign-index"))
	for _, test := range []struct {
		path    string
		content string
		valid   bool
	}{
		{"document.md", files["document.md"], true},
		{"document.md", "#   Current\n", false},
		{"docs/archive/current.md", "#   Current archive operation\n", false},
		{".config/archive/metadata.json", "{\"version\":1}\n", false},
		{".config/fixture.yaml", "name:    fixture\n", false},
		{"build/tracked.json", "{\"version\":1}\n", false},
		{"literal[1].json", "{\"version\":1}\n", false},
	} {
		t.Run(fmt.Sprintf("%s/valid=%t", test.path, test.valid), func(t *testing.T) {
			destination := filepath.Join(root, filepath.FromSlash(test.path))
			if err := os.WriteFile(destination, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.WriteFile(destination, []byte(files[test.path]), 0o600); err != nil {
					t.Error(err)
				}
			})
			var output []byte
			err := run([]string{"check-format", root}, &bytes.Buffer{}, func(call command) error {
				if len(call.Args) != 1 {
					t.Fatalf("format inventory escaped onto command line: %d", len(call.Args))
				}
				if strings.Contains(call.Input, filepath.ToSlash(root)) {
					t.Fatal("format inventory must resolve relative to the requested checkout")
				}
				call.Args[0] = filepath.Join(repository, "tools", "ci", "format.mjs")
				var err error
				output, err = systemOutputRunner(call)
				return err
			})
			if test.valid {
				if err != nil {
					t.Fatalf("formatted checkout: %v\n%s", err, output)
				}
			} else if err == nil || !bytes.Contains(output, []byte(test.path)) {
				t.Fatalf("current carrier %q was not diagnosed: %v\n%s", test.path, err, output)
			}
			if actual := readFile(t, destination); string(actual) != test.content {
				t.Fatalf("format check changed %s", test.path)
			}
		})
	}
}

func TestFormattingRequiresNativeInputsAndSupportedSource(t *testing.T) {
	repository := repositoryRoot(t)
	checker := filepath.Join(repository, "tools", "ci", "format.mjs")
	for _, scenario := range []struct{ name, input, diagnostic string }{
		{"empty inventory", "[]", "no authored files supported by Prettier"},
		{"unsupported source", `["source.txt"]`, "no authored files supported by Prettier"},
		{"missing ignore", `["source.txt"]`, "ENOENT"},
		{"missing dependency", `["source.txt"]`, "ERR_MODULE_NOT_FOUND"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "source.txt")
			content := []byte("unsupported authored source\n")
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatal(err)
			}
			if scenario.name != "missing ignore" {
				if err := os.WriteFile(filepath.Join(root, ".prettierignore"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			entry := checker
			if scenario.name == "missing dependency" {
				entry = filepath.Join(root, "tools", "ci", "format.mjs")
				if err := os.MkdirAll(filepath.Dir(entry), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(entry, readFile(t, checker), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command("node", entry)
			command.Dir, command.Stdin = root, strings.NewReader(scenario.input)
			output, err := command.CombinedOutput()
			if err == nil || !bytes.Contains(output, []byte(scenario.diagnostic)) {
				t.Fatalf("%s: %v\n%s", scenario.name, err, output)
			}
			if !bytes.Equal(readFile(t, path), content) {
				t.Fatal("check changed source")
			}
		})
	}
}

func TestNativeCheckAdaptersConsumeCompletePipeInput(t *testing.T) {
	repository := repositoryRoot(t)
	root := t.TempDir()
	for _, path := range []string{".prettierignore", ".config/checks/markdown/policy.yaml"} {
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, readFile(t, filepath.Join(repository, path)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "document.md")
	content := []byte("# Document\n\n```mermaid\nflowchart LR\n  A --> B\n```\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	input := string(encoded[:1]) + strings.Repeat(" \n", 1<<19) + string(encoded[1:])
	for _, adapter := range []string{"format.mjs", "markdown/lint.mjs", "markdown/diagrams.mjs"} {
		t.Run(adapter, func(t *testing.T) {
			command := exec.Command("node", filepath.Join(repository, "tools", "ci", filepath.FromSlash(adapter)))
			command.Dir, command.Stdin = root, strings.NewReader(input)
			output, err := command.CombinedOutput()
			if err != nil || !bytes.Contains(output, []byte("checked 1")) {
				t.Fatalf("complete streamed inventory: %v\n%s", err, output)
			}
			if !bytes.Equal(readFile(t, path), content) {
				t.Fatal("check changed source")
			}
		})
	}
}
