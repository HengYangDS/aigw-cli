package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestSourceRunsThePortableGateSequence(t *testing.T) {
	t.Setenv("AIGW_COMMIT_BASE", "")
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
	want := [][]string{
		{"golangci-lint", "config", "verify", "--config", ".config/checks/go/policy.yml"},
		{"goreleaser", "check", ".config/release/goreleaser.yaml"},
		{"cue", "fmt", "--check", "--files", ".config/ci"},
		{"go", "run", "./tools/ci", "project", "--check"},
		{"node", "--run", "dependencies:audit"},
		{"go", "run", "./tools/ci", "openspec"},
		{"editorconfig-checker", "-disable-indentation", "-disable-indent-size"},
		{"go", "run", "./tools/ci", "check-format", "."},
		{"go", "run", "./tools/ci", "check-markdown", "."},
		{"go", "run", "./tools/ci", "check-mermaid", "."},
		{"go", "run", "./tools/ci", "links", "."},
		{"go", "run", "./tools/ci", "check-toml", "."},
		{"go", "mod", "tidy", "-diff"},
		{"go", "mod", "verify"},
		{"osv-scanner", "scan", "source", "--lockfile", "go.mod", "--lockfile", "package-lock.json", "--format", "table", "--verbosity", "warn", "."},
		{"go", "run", "./tools/ci", "check-secrets", "."},
		{"go", "run", "./tools/release", "validate-toolchain", "go.mod"},
		{"go", "run", "./tools/release", "validate-release-sources"},
		{"go", "run", "./tools/architecture", "--root", "."},
		{"go", "run", "./tools/ci", "check-source-size", "."},
		{"go", "run", "./tools/ci", "check-go", "."},
		{"go", "test", "-tags=client_acceptance", "./tools/release", "-run", "^TestNativeClient(Inputs|StreamEnvelope|FilePreservation)$"},
		{"go", "run", "./tools/repository", "--root", ".", "protected-lifecycle"},
		{"actionlint"},
		{"go", "run", "./tools/coverage", "--race"},
	}
	var got [][]string
	runner := func(call command) error {
		got = append(got, append([]string{call.Name}, call.Args...))
		return nil
	}
	if err := run([]string{"source"}, &bytes.Buffer{}, runner); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestQualityCommandsResolveInDeclaredToolchain(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	output, err := exec.Command("cue", "export", ".config/ci/pipeline.cue", ".ethos/workspace.toml", "--expression", "qualityToolchain", "--out", "json").Output()
	if err != nil {
		t.Fatal(err)
	}
	var environment map[string]string
	if err := json.Unmarshal(output, &environment); err != nil {
		t.Fatal(err)
	}
	if environment["MISE_ENABLE_TOOLS"] == "" {
		t.Fatal("quality toolchain must declare its native tools")
	}
	for key, value := range environment {
		t.Setenv(key, value)
	}
	observed := make(map[string]bool)
	for _, call := range qualityCommands {
		if observed[call.Name] {
			continue
		}
		observed[call.Name] = true
		t.Run(call.Name, func(t *testing.T) {
			if output, err := exec.Command("mise", "which", call.Name).CombinedOutput(); err != nil {
				t.Fatalf("quality command is unavailable in its declared toolchain: %v\n%s", err, output)
			}
		})
	}
}

func TestQualityUsesNativeConfigurationSchemas(t *testing.T) {
	repository := repositoryRoot(t)
	for _, test := range []struct {
		tool, policy, state string
	}{
		{"golangci-lint", ".config/checks/go/policy.yml", "valid"},
		{"golangci-lint", ".config/checks/go/policy.yml", "unknown field"},
		{"golangci-lint", ".config/checks/go/policy.yml", "missing"},
		{"goreleaser", ".config/release/goreleaser.yaml", "valid"},
		{"goreleaser", ".config/release/goreleaser.yaml", "unknown field"},
		{"goreleaser", ".config/release/goreleaser.yaml", "missing"},
	} {
		t.Run(test.tool+"/"+test.state, func(t *testing.T) {
			content := readFile(t, filepath.Join(repository, filepath.FromSlash(test.policy)))
			if test.state == "unknown field" {
				content = append(content, []byte("\naigw_unknown_configuration_field: true\n")...)
			}
			root := filepath.Join(t.TempDir(), "checkout with spaces")
			policy := filepath.Join(root, filepath.FromSlash(test.policy))
			if err := os.MkdirAll(filepath.Dir(policy), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)
			t.Setenv("AIGW_COMMIT_BASE", "")
			t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "")
			t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
			if test.state != "missing" {
				if err := os.WriteFile(policy, content, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			observed := false
			var diagnostics []byte
			err := run([]string{"quality"}, &bytes.Buffer{}, func(call command) error {
				if call.Name != test.tool {
					return nil
				}
				observed = true
				var err error
				diagnostics, err = systemOutputRunner(call)
				return err
			})
			if !observed || (err == nil) != (test.state == "valid") {
				t.Fatalf("native schema validation: observed=%t state=%s error=%v\n%s", observed, test.state, err, diagnostics)
			}
			if test.state == "missing" {
				if _, err := os.Stat(policy); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("schema validation created missing input: %v", err)
				}
				return
			}
			if actual := readFile(t, policy); !bytes.Equal(actual, content) {
				t.Fatal("schema validation changed its input")
			}
		})
	}
}

func TestRepositoryInventoryUsesTheRequestedCheckoutIndex(t *testing.T) {
	root, foreign := t.TempDir(), t.TempDir()
	for _, directory := range []string{root, foreign} {
		if output, err := exec.Command("git", "-C", directory, "init", "--quiet").CombinedOutput(); err != nil {
			t.Fatalf("git init: %v\n%s", err, output)
		}
		if err := os.WriteFile(filepath.Join(directory, ".gitignore"), []byte("*.md\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "README.md"), []byte("# Project\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.Command("git", "-C", directory, "add", "-f", "README.md").CombinedOutput(); err != nil {
			t.Fatalf("git add: %v\n%s", err, output)
		}
	}
	owned := filepath.Join(root, "owned.md")
	if err := os.WriteFile(owned, []byte("# Owned source\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", root, "add", "-f", "owned.md").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
	}
	for _, binding := range []struct{ name, value string }{
		{"GIT_INDEX_FILE", filepath.Join(foreign, ".git", "index")},
		{"GIT_COMMON_DIR", filepath.Join(foreign, ".git")},
	} {
		t.Run(binding.name, func(t *testing.T) {
			t.Setenv(binding.name, binding.value)
			files, err := currentRepositoryFiles(root, "Markdown", "*.md")
			if err != nil || !slices.Equal(files, []string{filepath.Join(root, "README.md"), owned}) {
				t.Fatalf("requested checkout inventory = %v, %v", files, err)
			}
		})
	}
}

func TestSourceExtendsQualityWithCompleteCoverage(t *testing.T) {
	t.Setenv("AIGW_COMMIT_BASE", "")
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
	quality, err := configuredQualityCommands()
	if err != nil {
		t.Fatal(err)
	}
	source, err := configuredSourceCommands()
	if err != nil {
		t.Fatal(err)
	}
	if len(source) != len(quality)+1 || !reflect.DeepEqual(source[:len(quality)], quality) {
		t.Fatalf("source gate does not extend quality exactly\nquality: %#v\nsource:  %#v", quality, source)
	}
	if call := source[len(quality)]; call.Name != "go" || !slices.Equal(call.Args, []string{"run", "./tools/coverage", "--race"}) {
		t.Fatalf("source gate lacks complete coverage: %#v", source)
	}

	var got [][]string
	if err := run([]string{"quality"}, &bytes.Buffer{}, func(call command) error {
		got = append(got, append([]string{call.Name}, call.Args...))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if expected := commandArguments(quality); !reflect.DeepEqual(got, expected) {
		t.Fatalf("commands = %#v, want %#v", got, expected)
	}
}

func commandArguments(commands []command) [][]string {
	arguments := make([][]string, 0, len(commands))
	for _, call := range commands {
		arguments = append(arguments, append([]string{call.Name}, call.Args...))
	}
	return arguments
}

func TestSourceStopsAtTheFirstFailedGate(t *testing.T) {
	t.Setenv("AIGW_COMMIT_BASE", "")
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
	want := errors.New("failed")
	calls := 0
	err := run([]string{"source"}, &bytes.Buffer{}, func(command) error {
		calls++
		if calls == 3 {
			return want
		}
		return nil
	})
	if !errors.Is(err, want) || calls != 3 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}

func TestSourceIncludesProductProvenanceWhenConfigured(t *testing.T) {
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "maintainer@example.com")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "trust/allowed-signers")
	t.Setenv("AIGW_COMMIT_BASE", "accepted")
	var got []command
	if err := run([]string{"source"}, &bytes.Buffer{}, func(call command) error {
		got = append(got, call)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := command{Name: "go", Args: []string{"run", "./tools/forge", "commits", "--base", "accepted", "--email", "maintainer@example.com", "--allowed-signers", "trust/allowed-signers"}}
	if !slices.ContainsFunc(got, func(call command) bool { return reflect.DeepEqual(call, want) }) {
		t.Fatalf("missing provenance command: %#v", got)
	}
}

func TestSourceConfigurationRejectsIncompleteProductProvenance(t *testing.T) {
	for _, missing := range []string{"base", "email", "signers"} {
		t.Run(missing, func(t *testing.T) {
			t.Setenv("AIGW_COMMIT_BASE", "accepted")
			t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "maintainer@example.com")
			t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "trust/allowed-signers")
			switch missing {
			case "base":
				t.Setenv("AIGW_COMMIT_BASE", "")
			case "email":
				t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "")
			case "signers":
				t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
			}
			if _, err := configuredSourceCommands(); err == nil || !strings.Contains(err.Error(), "requires commit base") {
				t.Fatalf("missing %s error = %v", missing, err)
			}
		})
	}
}

func TestSourceReportsInvalidArgumentsAndConfiguredSourceFailure(t *testing.T) {
	if err := run([]string{"source", "extra"}, &bytes.Buffer{}, func(command) error { return nil }); err == nil {
		t.Fatal("source accepted an extra argument")
	}
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "maintainer@example.com")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
	if err := run([]string{"source"}, &bytes.Buffer{}, func(command) error { return nil }); err == nil || !strings.Contains(err.Error(), "allowed signers") {
		t.Fatalf("configured source error = %v", err)
	}
}

func TestRunRejectsInvalidCommandShapes(t *testing.T) {
	for _, args := range [][]string{
		{"project", "extra"},
		{"quality", "extra"},
		{"source", "extra"},
		{"openspec", "extra"},
		{"links"},
		{"links", ".", "extra"},
		{"check-go"},
		{"check-go", ".", "extra"},
		{"check-toml"},
		{"check-toml", ".", "extra"},
		{"check-format"},
		{"check-format", ".", "extra"},
		{"check-markdown"},
		{"check-markdown", ".", "extra"},
		{"check-markdown-policy"},
		{"check-markdown-policy", ".", "extra"},
		{"check-secrets"},
		{"check-secrets", ".", "extra"},
		{"native", "--platform", "linux", "extra"},
		{"trust-input"},
		{"trust-input", "--output", "out", "--github-env", "env", "extra"},
	} {
		if err := run(args, &bytes.Buffer{}, func(command) error { return nil }); err == nil {
			t.Fatalf("accepted %#v", args)
		}
	}
}

func TestTrustInputRejectsMissingOrUnwritableDestinations(t *testing.T) {
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS", "")
	root := t.TempDir()
	output := filepath.Join(root, "allowed-signers")
	environment := filepath.Join(root, "github-env")
	if err := writeTrustInput(output, environment, "AIGW_RELEASE_ALLOWED_SIGNERS"); err == nil || !strings.Contains(err.Error(), "is required") {
		t.Fatalf("missing trust error = %v", err)
	}

	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS", "release ssh-ed25519 fixture")
	if err := writeTrustInput(filepath.Join(root, "missing", "allowed-signers"), environment, "AIGW_RELEASE_ALLOWED_SIGNERS"); err == nil || !strings.Contains(err.Error(), "write trust input") {
		t.Fatalf("write trust error = %v", err)
	}
	if err := writeTrustInput(output, filepath.Join(root, "missing", "github-env"), "AIGW_RELEASE_ALLOWED_SIGNERS"); err == nil || !strings.Contains(err.Error(), "open GitHub environment") {
		t.Fatalf("environment open error = %v", err)
	}
}

func TestTrustInputWritesPrivateFileAndGitHubEnvironment(t *testing.T) {
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS", "release ssh-ed25519 fixture")
	root := t.TempDir()
	output := filepath.Join(root, "allowed-signers")
	environment := filepath.Join(root, "github-env")
	if err := run([]string{"trust-input", "--output", output, "--github-env", environment}, &bytes.Buffer{}, func(command) error { return nil }); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "release ssh-ed25519 fixture\n" {
		t.Fatalf("trust input=%q error=%v", data, err)
	}
	info, err := os.Stat(output)
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("trust mode=%v error=%v", info, err)
	}
	envData, err := os.ReadFile(environment)
	if err != nil || string(envData) != "AIGW_RELEASE_ALLOWED_SIGNERS_FILE="+output+"\n" {
		t.Fatalf("GitHub environment=%q error=%v", envData, err)
	}
}

func TestArtifactTrustInputHasItsOwnExplicitAuthority(t *testing.T) {
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS", "source-only ssh-ed25519 fixture")
	t.Setenv("AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS", "release ssh-ed25519 artifact")
	root := t.TempDir()
	output, environment := filepath.Join(root, "artifact-signers"), filepath.Join(root, "github-env")
	if err := run([]string{"trust-input", "--artifact", "--output", output, "--github-env", environment}, &bytes.Buffer{}, func(command) error { return nil }); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "release ssh-ed25519 artifact\n" {
		t.Fatalf("artifact trust=%q error=%v", data, err)
	}
	data, err = os.ReadFile(environment)
	if err != nil || string(data) != "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE="+output+"\n" {
		t.Fatalf("artifact environment=%q error=%v", data, err)
	}
	t.Setenv("AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS", "")
	missing := filepath.Join(root, "missing-artifact-signers")
	err = run([]string{"trust-input", "--artifact", "--output", missing, "--github-env", environment}, &bytes.Buffer{}, func(command) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS is required") {
		t.Fatalf("absent artifact authority must not adopt source trust: %v", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("absent authority wrote a trust file: %v", err)
	}
}

func TestProjectCommandRendersAndChecksTheTrackedProjections(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, ".config", "ci", "pipeline.cue")
	if err := os.MkdirAll(filepath.Dir(model), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `package ci
gitlab: {stages: ["verify"]}
githubVerify: {name: "Verify"}
githubRelease: {name: "Release"}
`
	if err := os.WriteFile(model, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(root, ".ethos", "workspace.toml")
	if err := os.MkdirAll(filepath.Dir(workspace), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workspace, []byte("[branch_roles]\naccepted_branch = 'dev'\nrelease_branch = 'main'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"project", "--root", root}, &bytes.Buffer{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"project", "--check", "--root", root}, &bytes.Buffer{}, nil); err != nil {
		t.Fatalf("fresh projection check: %v", err)
	}
	for _, relative := range []string{".gitlab-ci.yml", ".github/workflows/verify.yml", ".github/workflows/release.yml"} {
		path := filepath.Join(root, relative)
		if data, err := os.ReadFile(path); err != nil || len(data) == 0 {
			t.Fatalf("projection %s: data=%q error=%v", relative, data, err)
		}
	}
}

func TestProjectCommandReportsRenderingFailure(t *testing.T) {
	if err := run([]string{"project", "--root", t.TempDir()}, &bytes.Buffer{}, nil); err == nil || !strings.Contains(err.Error(), "render .gitlab-ci.yml") {
		t.Fatalf("project command model error = %v", err)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRepositoryRootWorksInSourceArchive(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "tools", "ci")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(directory)
	if got := repositoryRoot(t); got != root {
		t.Fatalf("repository root = %q, want %q", got, root)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
