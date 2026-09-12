package construction

import (
	"aigw-cli/tools/release/artifact"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

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

func TestGoReleaserArchiveMetadataIsHostIndependent(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", ".config", "release", "goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var configuration struct {
		Archives []struct {
			BuildsInfo struct {
				Owner string `yaml:"owner"`
				Group string `yaml:"group"`
			} `yaml:"builds_info"`
			Files []struct {
				Source string `yaml:"src"`
				Info   struct {
					Owner string `yaml:"owner"`
					Group string `yaml:"group"`
				} `yaml:"info"`
			} `yaml:"files"`
		} `yaml:"archives"`
	}
	if err := yaml.Unmarshal(data, &configuration); err != nil {
		t.Fatal(err)
	}
	if len(configuration.Archives) != 1 {
		t.Fatalf("archives = %d, want 1", len(configuration.Archives))
	}
	archive := configuration.Archives[0]
	if archive.BuildsInfo.Owner != "root" || archive.BuildsInfo.Group != "root" {
		t.Fatalf("build archive identity = %q:%q, want root:root", archive.BuildsInfo.Owner, archive.BuildsInfo.Group)
	}
	for _, file := range archive.Files {
		if file.Info.Owner != "root" || file.Info.Group != "root" {
			t.Fatalf("archive identity for %s = %q:%q, want root:root", file.Source, file.Info.Owner, file.Info.Group)
		}
	}
}

func TestReleaseBuildInvokesPortableToolchainWithExplicitInputs(t *testing.T) {
	root := releaseRoot(t)
	for name, content := range map[string]string{
		"go.mod": "fixture", "go.sum": "fixture", "package-lock.json": "fixture",
		"mise.lock": "fixture", "LICENSE": "fixture", "README.md": "fixture",
		"mise.toml": "[tools]\ngo = \"1.27.1\"\nosv-scanner = \"2.5.1\"\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	key := signingKey(t)
	output := filepath.Join(root, "dist")
	var calls []toolCall
	var names []string
	runner := func(call toolCall) error {
		calls = append(calls, call)
		names = append(names, call.Name)
		if call.Name == "git" && slices.Contains(call.Args, "status") {
			return nil
		}
		if call.Name == "goreleaser" {
			return populatePortableStage(t, call, "1.2.3", "portable_darwin_arm64_v8.0/aigw")
		}
		if call.Name == "syft" {
			path := strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")
			return os.WriteFile(path, []byte(`{"spdxVersion":"SPDX-2.3","creationInfo":{}}`), 0o600)
		}
		if call.Name == "osv-scanner" {
			path := call.Args[slices.Index(call.Args, "--output-file")+1]
			encoded, err := dependencyReportFixture(root)
			if err != nil {
				return err
			}
			return os.WriteFile(path, encoded, 0o600)
		}
		if call.Name == "git" {
			value := strings.Repeat("a", 40) + "\n"
			if slices.Contains(call.Args, "HEAD^{tree}") {
				value = strings.Repeat("b", 40) + "\n"
			}
			_, err := call.Stdout.Write([]byte(value))
			return err
		}
		if call.Name == "ssh-keygen" {
			command := exec.Command(call.Name, call.Args...)
			command.Dir = call.Directory
			return command.Run()
		}
		return nil
	}
	request := buildRequest{
		Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400",
		GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/aigw-cli",
		GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli",
		SigningKey: key,
	}
	if err := buildRelease(request, runner); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(names, []string{"git", "goreleaser", "syft", "osv-scanner", "git", "git", "ssh-keygen"}) {
		t.Fatalf("calls = %#v", calls)
	}
	osv := calls[slices.IndexFunc(calls, func(call toolCall) bool { return call.Name == "osv-scanner" })]
	for _, expected := range []string{"scan", "source", "--lockfile", filepath.Join(root, "go.mod"), filepath.Join(root, "package-lock.json"), "--no-call-analysis=go", "--all-packages", "--licenses="} {
		if !slices.Contains(osv.Args, expected) {
			t.Fatalf("OSV arguments missing %q: %v", expected, osv.Args)
		}
	}
	for _, expected := range []string{"AIGW_VERSION=1.2.3", "AIGW_RELEASE_EPOCH=1784246400", "AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example", "AIGW_GITHUB_RELEASE_REPOSITORY=org/aigw-cli"} {
		goReleaser := calls[slices.IndexFunc(calls, func(call toolCall) bool { return call.Name == "goreleaser" })]
		if !slices.Contains(goReleaser.Env, expected) {
			t.Fatalf("GoReleaser environment missing %q: %v", expected, goReleaser.Env)
		}
	}
	if err := artifact.ValidateMatrix(output, "1.2.3"); err != nil {
		t.Fatal(err)
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
		if call.Name != "git" || call.Directory != "repository" || !reflect.DeepEqual(call.Args, []string{"status", "--porcelain=v1", "--untracked-files=all"}) {
			t.Fatalf("clean-source call = %#v", call)
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
	config := filepath.Join(root, ".config", "release")
	if err := os.MkdirAll(config, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "goreleaser.yaml"), []byte("version: 2\n"), 0o600); err != nil {
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

func TestRenderGoReleaserConfigRejectsMissingSource(t *testing.T) {
	if _, err := renderGoReleaserConfig(t.TempDir(), t.TempDir(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "read GoReleaser config") {
		t.Fatalf("missing config error = %v", err)
	}
}

func TestRenderGoReleaserConfigRejectsUnwritableDestination(t *testing.T) {
	root := releaseRoot(t)
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := renderGoReleaserConfig(root, blocked, t.TempDir()); err == nil || !strings.Contains(err.Error(), "write GoReleaser config") {
		t.Fatalf("unwritable config error = %v", err)
	}
}

func TestReleaseBuildAcceptsLocalOrSingleForgeContext(t *testing.T) {
	for name, request := range map[string]buildRequest{
		"local":  {Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400", SigningKey: "key"},
		"gitlab": {Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400", GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/subgroup/aigw-cli", SigningKey: "key"},
		"github": {Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400", GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli", SigningKey: "key"},
		"ipv6":   {Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400", GitLabOrigin: "https://[fd00::8]:8443/", GitLabRepository: "group_name/project.name-1", SigningKey: "key"},
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
			err := buildRelease(request, func(toolCall) error { launched = true; return nil })
			if err == nil || launched {
				t.Fatalf("error=%v launched=%t", err, launched)
			}
		})
	}
}

func TestBuildCIResolvesTagVersionAndReproducibleEpoch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CI_COMMIT_TAG", "v1.2.3")
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://gitlab.example")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "group/aigw-cli")
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", "https://github.example")
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "org/aigw-cli")
	var epochs []string
	build := func(request buildRequest) error {
		epochs = append(epochs, request.Epoch)
		if request.Version != "1.2.3" {
			t.Fatalf("version=%q", request.Version)
		}
		if err := os.MkdirAll(request.Output, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(request.Output, "artifact"), []byte("same"), 0o600)
	}
	epoch := func(root, version string) (string, error) {
		if root == "" || version != "1.2.3" {
			t.Fatalf("epoch root=%q version=%q", root, version)
		}
		return "1784246400", nil
	}
	if err := buildCI(root, filepath.Join(t.TempDir(), "build"), filepath.Join(t.TempDir(), "dist"), build, epoch, func(left, right, version string) error {
		if version != "1.2.3" {
			t.Fatalf("compare version=%q", version)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(epochs, []string{"1784246400", "1784246400"}) {
		t.Fatalf("epochs=%v", epochs)
	}
}

func TestBuildCIRejectsMissingVersionCarrier(t *testing.T) {
	missing := t.TempDir()
	t.Setenv("CI_COMMIT_TAG", "v1.2.3")
	if err := buildCI(missing, t.TempDir(), t.TempDir(), nil, nil, nil); err == nil || !strings.Contains(err.Error(), "read VERSION") {
		t.Fatalf("missing CI VERSION error = %v", err)
	}
}

func TestBuildIdentityUsesSemanticVersionGrammar(t *testing.T) {
	for _, tc := range []struct {
		version string
		valid   bool
	}{
		{"1.2.3-rc.1+build.7", true},
		{"01.2.3", false},
		{"1.2.3-rc.01", false},
	} {
		t.Run(tc.version, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte(tc.version+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			err := validateRequest(buildRequest{Version: tc.version, Epoch: "0"})
			if (err == nil) != tc.valid {
				t.Errorf("build identity error=%v, valid=%t", err, tc.valid)
			}
			t.Setenv("CI_COMMIT_TAG", "v"+tc.version)
			epochReached := errors.New("identity admitted")
			err = buildCI(root, "", "", nil, func(string, string) (string, error) {
				return "", epochReached
			}, nil)
			if errors.Is(err, epochReached) != tc.valid || (!tc.valid && (err == nil || !strings.Contains(err.Error(), "invalid CI release version"))) {
				t.Fatalf("CI identity error=%v, valid=%t", err, tc.valid)
			}
		})
	}
}

func TestBuildCIRejectsTagThatDisagreesWithVersionCarrier(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CI_COMMIT_TAG", "v1.2.3")

	err := buildCI(root, filepath.Join(t.TempDir(), "build"), filepath.Join(t.TempDir(), "dist"), nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "VERSION") {
		t.Fatalf("error = %v, want VERSION mismatch", err)
	}
}

func TestBuildCIFailsClosedAcrossUntaggedAndDependencyFailures(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(root, "workspace")
	output := filepath.Join(root, "dist")

	t.Setenv("CI_COMMIT_TAG", "")
	if err := buildCI(root, workspace, output, nil, nil, nil); err == nil || !strings.Contains(err.Error(), "requires CI_COMMIT_TAG") {
		t.Fatalf("missing identity error = %v", err)
	}
	t.Setenv("CI_COMMIT_TAG", "v1.2.3")
	want := errors.New("epoch failed")
	if err := buildCI(root, workspace, output, nil, func(string, string) (string, error) { return "", want }, nil); !errors.Is(err, want) {
		t.Fatalf("epoch error = %v", err)
	}

	epoch := func(string, string) (string, error) { return "1784246400", nil }
	calls := 0
	if err := buildCI(root, workspace, output, func(buildRequest) error {
		calls++
		return want
	}, epoch, nil); !errors.Is(err, want) || calls != 1 {
		t.Fatalf("first build error=%v calls=%d", err, calls)
	}
	calls = 0
	if err := buildCI(root, workspace, output, func(request buildRequest) error {
		calls++
		if calls == 2 {
			return want
		}
		return os.MkdirAll(request.Output, 0o755)
	}, epoch, nil); !errors.Is(err, want) || calls != 2 {
		t.Fatalf("second build error=%v calls=%d", err, calls)
	}
	if err := buildCI(root, workspace, output, func(request buildRequest) error {
		return os.MkdirAll(request.Output, 0o755)
	}, epoch, func(string, string, string) error { return want }); !errors.Is(err, want) {
		t.Fatalf("comparison error = %v", err)
	}
}

func TestResolveReleaseEpochUsesChangelogAuthorityInEveryEnvironment(t *testing.T) {
	root := t.TempDir()
	changelog := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(changelog, []byte("## [1.2.3] - 2026-08-09\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if epoch, err := resolveReleaseEpoch(root, "1.2.3"); err != nil || epoch != "1786233600" {
		t.Fatalf("local release epoch=%q error=%v", epoch, err)
	}

	t.Setenv("CI_COMMIT_TAG", "v1.2.3")
	if epoch, err := resolveReleaseEpoch(root, "1.2.3"); err != nil || epoch != "1786233600" {
		t.Fatalf("tagged release epoch=%q error=%v", epoch, err)
	}
	if _, err := resolveReleaseEpoch(root, "9.9.9"); err == nil || !strings.Contains(err.Error(), "heading not found") {
		t.Fatalf("missing heading error = %v", err)
	}
	if _, err := resolveReleaseEpoch(t.TempDir(), "1.2.3"); err == nil || !strings.Contains(err.Error(), "open CHANGELOG") {
		t.Fatalf("missing changelog error=%v", err)
	}
}

func TestCandidateBuildEpochUsesSourceCommitWithoutInventingARelease(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("## [Unreleased]\n\n## [1.2.3] - 2026-08-09\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "--quiet", root},
		{"-C", root, "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "-c", "user.name=Build Test", "-c", "user.email=build@example.test", "commit", "--allow-empty", "--quiet", "-m", "test source"},
	} {
		command := exec.Command("git", args...)
		command.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-09-07T00:00:00Z", "GIT_COMMITTER_DATE=2026-09-07T00:00:00Z")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("prepare source commit: %v: %s", err, output)
		}
	}
	t.Setenv("CI_COMMIT_TAG", "")
	t.Setenv("GITHUB_REF_TYPE", "")
	if epoch, err := resolveReleaseEpoch(root, "1.2.4"); err != nil || epoch != "1788739200" {
		t.Fatalf("candidate source epoch=%q error=%v", epoch, err)
	}
	t.Setenv("CI_COMMIT_TAG", "v1.2.4")
	if _, err := resolveReleaseEpoch(root, "1.2.4"); err == nil {
		t.Fatal("tagged release accepted without its release chronicle")
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
	err := buildRelease(buildRequest{Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400", GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/aigw-cli", GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli", SigningKey: "key"}, func(call toolCall) error {
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
