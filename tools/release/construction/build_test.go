package construction

import (
	"aigw-cli/internal/program/artifact"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

func TestGoReleaserStagePreservesPaths(t *testing.T) {
	for _, stage := range []string{`/tmp/native release/goreleaser`, `C:\Users\runner\AppData\Local\Temp\native release\goreleaser`} {
		t.Run(stage, func(t *testing.T) {
			config, err := renderGoReleaserConfig(releaseRoot(t), t.TempDir(), stage)
			if err != nil {
				t.Fatal(err)
			}
			if got := goReleaserStage(t, []string{"--config", config}); got != stage {
				t.Fatalf("GoReleaser stage = %q, want %q", got, stage)
			}
		})
	}
}

func TestReleaseBuildInvokesPortableToolchainWithExplicitInputs(t *testing.T) {
	root := releaseRoot(t)
	for name, content := range map[string]string{
		"go.mod": "module aigw-cli\n", "go.sum": "fixture", "package-lock.json": "fixture",
		"LICENSE": "fixture", "README.md": "fixture",
		".config/mise/config.toml": "[tools]\ngo = \"1.27.1\"\nosv-scanner = \"2.5.1\"\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	key := signingKey(t)
	output := filepath.Join(root, "dist")
	var calls []toolCall
	fault := ""
	runner := releaseFixtureRunner(t, root, &calls, &fault)
	request := buildRequest{
		Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400",
		GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/aigw-cli",
		GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli",
		SigningKey: key,
	}
	if err := buildRelease(t.Context(), request, runner); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, call := range calls {
		if call.Name != "git" {
			names = append(names, call.Name)
		}
	}
	if !slices.Equal(names, []string{"osv-scanner", "goreleaser", "syft", "ssh-keygen"}) {
		t.Fatalf("calls = %#v", calls)
	}
	osv := calls[slices.IndexFunc(calls, func(call toolCall) bool { return call.Name == "osv-scanner" })]
	for _, expected := range []string{"scan", "source", "--config", filepath.Join(filepath.Dir(osv.Args[len(osv.Args)-1]), "policy.toml"), "--lockfile", filepath.Join(root, "go.mod"), filepath.Join(root, "package-lock.json"), "--no-call-analysis=go", "--all-packages", "--all-vulns", "--licenses="} {
		if !slices.Contains(osv.Args, expected) {
			t.Fatalf("OSV arguments missing %q: %v", expected, osv.Args)
		}
	}
	for _, expected := range []string{"AIGW_VERSION=1.2.3", "GORELEASER_CURRENT_TAG=v1.2.3", "AIGW_RELEASE_EPOCH=1784246400", "AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example", "AIGW_GITHUB_RELEASE_REPOSITORY=org/aigw-cli"} {
		goReleaser := calls[slices.IndexFunc(calls, func(call toolCall) bool { return call.Name == "goreleaser" })]
		if !slices.Contains(goReleaser.Env, expected) {
			t.Fatalf("GoReleaser environment missing %q: %v", expected, goReleaser.Env)
		}
	}
	if err := artifact.ValidateMatrix(output, "1.2.3"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(output, "homebrew", "Casks", "aigw.rb")); err != nil || string(data) != "generated cask" {
		t.Fatalf("generated package projection was discarded: %q, %v", data, err)
	}
}

func TestReleaseSourceMustBeClean(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		var call toolCall
		if err := ensureCleanSource("repository", func(got toolCall) error {
			call = got
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if call.Name != "git" || call.Directory != "repository" || !reflect.DeepEqual(call.Args, []string{"--no-optional-locks", "status", "--porcelain=v1", "--untracked-files=all"}) {
			t.Fatalf("clean-source call = %#v", call)
		}
	})

	t.Run("inspection preserves the source index", func(t *testing.T) {
		root := t.TempDir()
		signedNativeInputFixture(t, root, "0.3.1", signingKey(t))
		t.Setenv("GIT_OPTIONAL_LOCKS", "1")
		index := filepath.Join(root, ".git", "index")
		before, err := os.ReadFile(index)
		if err != nil {
			t.Fatal(err)
		}
		stamp := time.Unix(1, 0)
		if err := os.Chtimes(filepath.Join(root, "VERSION"), stamp, stamp); err != nil {
			t.Fatal(err)
		}
		if err := ensureCleanSource(root, executeTool(t.Context())); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(index)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("source inspection rewrote the Git index")
		}
	})

	t.Run("dirty", func(t *testing.T) {
		err := ensureCleanSource("repository", func(call toolCall) error {
			_, writeErr := call.Stdout.Write([]byte(" M tools/release/construction/build.go\n"))
			return writeErr
		})
		if err == nil || !strings.Contains(err.Error(), "requires committed source") {
			t.Fatalf("dirty source error = %v", err)
		}
	})

	t.Run("inspection failure", func(t *testing.T) {
		want := errors.New("git status failed")
		if err := ensureCleanSource("repository", func(toolCall) error { return want }); !errors.Is(err, want) {
			t.Fatalf("inspection error = %v", err)
		}
	})
}

func goReleaserStage(t *testing.T, args []string) string {
	t.Helper()
	var config string
	for index, argument := range args {
		if argument == "--config" && index+1 < len(args) {
			config = args[index+1]
			break
		}
	}
	if config == "" {
		t.Fatalf("GoReleaser config missing from %v", args)
	}
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	var configuration struct {
		Dist string `yaml:"dist"`
	}
	if err := yaml.Unmarshal(data, &configuration); err != nil {
		t.Fatal(err)
	}
	if configuration.Dist == "" {
		t.Fatalf("GoReleaser dist missing from %s", config)
	}
	return configuration.Dist
}

func releaseRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".config", "mise"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeScannerLock(t, root)
	config := filepath.Join(root, ".config", "release")
	if err := os.MkdirAll(filepath.Join(config, "goreleaser"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(config, "syft"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "goreleaser", ".goreleaser.yaml"), []byte("version: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	policy, err := os.ReadFile(filepath.Join("..", "..", "..", ".config", "release", "syft", ".syft.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "syft", ".syft.yaml"), policy, 0o600); err != nil {
		t.Fatal(err)
	}
	dependencyPolicy := filepath.Join(root, ".config", "checks", "dependencies", "policy.toml")
	if err := os.MkdirAll(filepath.Dir(dependencyPolicy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dependencyPolicy, []byte("PackageOverrides = []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func signingKey(t *testing.T) string {
	t.Helper()
	key := filepath.Join(t.TempDir(), "release-signing-key")
	if output, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("generate signing key: %v: %s", err, output)
	}
	return key
}

func TestReleaseBuildAcceptsLocalOrSingleForgeContext(t *testing.T) {
	for name, request := range map[string]buildRequest{
		"local":          {Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400", SigningKey: "key"},
		"gitlab":         {Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400", GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/subgroup/aigw-cli", SigningKey: "key"},
		"github":         {Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400", GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli", SigningKey: "key"},
		"ipv6":           {Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400", GitLabOrigin: "https://[fd00::8]:8443/", GitLabRepository: "group_name/project.name-1", SigningKey: "key"},
		"private GitLab": {Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400", GitLabOrigin: "http://10.0.0.8:8080", GitLabRepository: "group/project", SigningKey: "key"},
		"private GitHub": {Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400", GitHubOrigin: "http://[fd00::8]:8080", GitHubRepository: "org/project", SigningKey: "key"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateRequest(request); err != nil {
				t.Fatal(err)
			}
			t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", request.GitLabOrigin)
			t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", request.GitLabRepository)
			t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", request.GitHubOrigin)
			t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", request.GitHubRepository)
			if err := ValidateSources(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReleaseBuildRejectsInvalidOrPartialInputsBeforeLaunchingTools(t *testing.T) {
	valid := buildRequest{Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400", GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/aigw-cli", GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli", SigningKey: "key"}
	for name, mutate := range map[string]func(*buildRequest){
		"version":         func(r *buildRequest) { r.Version = "../escape" },
		"epoch":           func(r *buildRequest) { r.Epoch = "not-an-epoch" },
		"partial GitLab":  func(r *buildRequest) { r.GitLabRepository = "" },
		"partial GitHub":  func(r *buildRequest) { r.GitHubOrigin = "" },
		"insecure origin": func(r *buildRequest) { r.GitHubOrigin = "http://github.example" },
	} {
		t.Run(name, func(t *testing.T) {
			request := valid
			mutate(&request)
			launched := false
			err := buildRelease(t.Context(), request, func(toolCall) error { launched = true; return nil })
			if err == nil || launched {
				t.Fatalf("error=%v launched=%t", err, launched)
			}
		})
	}
}

func TestReleaseBuildPropagatesToolFailureAndNeverPublishesPartialMatrix(t *testing.T) {
	root := releaseRoot(t)
	output := filepath.Join(root, "dist")
	if err := os.MkdirAll(output, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "accepted"), []byte("previous release"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := errors.New("tool failed")
	err := buildRelease(t.Context(), buildRequest{Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400", GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/aigw-cli", GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli", SigningKey: signingKey(t)}, func(call toolCall) error {
		if call.Name == "goreleaser" {
			return populatePortableStage(t, call, "1.2.3", "portable_darwin_arm64_v8.0/aigw")
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
	entries, readErr := os.ReadDir(output)
	if readErr != nil || len(entries) != 1 || entries[0].Name() != "accepted" {
		t.Fatalf("previous output was not preserved atomically: entries=%v error=%v", entries, readErr)
	}
}

func TestReleaseBuildPropagatesChecksumAndMatrixFailures(t *testing.T) {
	root := releaseRoot(t)
	for name, content := range map[string]string{"go.mod": "module aigw-cli\n", "go.sum": "fixture", "package-lock.json": "fixture"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".config", "mise", "config.toml"), []byte("[tools]\ngo = \"1.27.1\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var calls []toolCall
	fault := ""
	runner := releaseFixtureRunner(t, root, &calls, &fault)
	output := filepath.Join(root, "dist")
	request := buildRequest{Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400", SigningKey: signingKey(t)}
	if err := buildRelease(t.Context(), request, runner); err != nil {
		t.Fatal(err)
	}
	accepted, err := os.ReadFile(filepath.Join(output, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"missing artifact", "unexpected artifact"} {
		t.Run(failure, func(t *testing.T) {
			fault, calls = failure, nil
			err := buildRelease(t.Context(), request, runner)
			if err == nil || (failure == "missing artifact" && !errors.Is(err, os.ErrNotExist)) || (failure == "unexpected artifact" && !strings.Contains(err.Error(), "unexpected")) {
				t.Fatalf("invalid artifact matrix was not rejected: %v", err)
			}
			if current, err := os.ReadFile(filepath.Join(output, "checksums.txt")); err != nil || string(current) != string(accepted) {
				t.Fatalf("invalid artifact matrix changed accepted output: %q, %v", current, err)
			}
			if err := artifact.ValidateMatrix(output, request.Version); err != nil {
				t.Fatal(err)
			}
			workspaces, err := filepath.Glob(filepath.Join(root, ".aigw-release-*"))
			if err != nil || len(workspaces) != 0 {
				t.Fatalf("invalid artifact matrix left workspace residue: %v, %v", workspaces, err)
			}
		})
	}
}
