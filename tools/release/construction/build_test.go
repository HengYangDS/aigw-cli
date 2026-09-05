package construction

import (
	"aigw-cli/tools/release/artifact"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

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
	for _, name := range []string{"go.mod", "LICENSE", "README.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(root, "dist")
	var calls []toolCall
	runner := func(call toolCall) error {
		calls = append(calls, call)
		if call.Name == "goreleaser" {
			stage := goReleaserStage(t, call.Args)
			if err := os.MkdirAll(stage, 0o700); err != nil {
				return err
			}
			for _, name := range artifact.Names("1.2.3") {
				if name == "checksums.txt" || strings.HasSuffix(name, ".spdx.json") {
					continue
				}
				if err := os.WriteFile(filepath.Join(stage, name), []byte(name), 0o600); err != nil {
					return err
				}
			}
			binary := filepath.Join(stage, "portable_darwin_arm64_v8.0", "aigw")
			if err := os.MkdirAll(filepath.Dir(binary), 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(binary, []byte("binary"), 0o700); err != nil {
				return err
			}
		}
		if call.Name == "syft" {
			path := strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")
			return os.WriteFile(path, []byte(`{"spdxVersion":"SPDX-2.3","creationInfo":{}}`), 0o600)
		}
		return nil
	}
	request := buildRequest{
		Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400",
		GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/aigw-cli",
		GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli",
	}
	if err := buildRelease(request, runner); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0].Name != "goreleaser" || calls[1].Name != "syft" {
		t.Fatalf("calls = %#v", calls)
	}
	for _, expected := range []string{"AIGW_VERSION=1.2.3", "AIGW_RELEASE_EPOCH=1784246400", "AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example", "AIGW_GITHUB_RELEASE_REPOSITORY=org/aigw-cli"} {
		if !slices.Contains(calls[0].Env, expected) {
			t.Fatalf("GoReleaser environment missing %q: %v", expected, calls[0].Env)
		}
	}
	if err := artifact.ValidateMatrix(output, "1.2.3"); err != nil {
		t.Fatal(err)
	}
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
	for _, line := range strings.Split(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "dist: "); ok {
			return strings.Trim(value, "\"")
		}
	}
	t.Fatalf("GoReleaser dist missing from %s", config)
	return ""
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
		"local":  {Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400"},
		"gitlab": {Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400", GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/aigw-cli"},
		"github": {Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400", GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateRequest(request); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReleaseBuildRejectsInvalidOrPartialInputsBeforeLaunchingTools(t *testing.T) {
	valid := buildRequest{Root: t.TempDir(), Output: t.TempDir(), Version: "1.2.3", Epoch: "1784246400", GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/aigw-cli", GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli"}
	for name, mutate := range map[string]func(*buildRequest){
		"version":         func(r *buildRequest) { r.Version = "../escape" },
		"epoch":           func(r *buildRequest) { r.Epoch = "not-an-epoch" },
		"partial GitLab":  func(r *buildRequest) { r.GitLabRepository = "" },
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

func TestValidateSourcesAcceptsLocalOrOneCompleteForge(t *testing.T) {
	for _, name := range []string{"AIGW_GITLAB_RELEASE_ORIGIN", "AIGW_GITLAB_RELEASE_REPOSITORY", "AIGW_GITHUB_RELEASE_ORIGIN", "AIGW_GITHUB_RELEASE_REPOSITORY"} {
		t.Setenv(name, "")
	}
	if err := ValidateSources(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://gitlab.example.test")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "group/project")
	if err := ValidateSources(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "")
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", "https://github.example.test")
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "owner/project")
	if err := ValidateSources(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "")
	if err := ValidateSources(); err == nil || !strings.Contains(err.Error(), "GitHub release source is incomplete") {
		t.Fatalf("partial source error = %v", err)
	}
}

func TestValidateSourcesRejectsInvalidAuthoritiesAndRepositories(t *testing.T) {
	for _, name := range []string{"AIGW_GITLAB_RELEASE_ORIGIN", "AIGW_GITLAB_RELEASE_REPOSITORY", "AIGW_GITHUB_RELEASE_ORIGIN", "AIGW_GITHUB_RELEASE_REPOSITORY"} {
		t.Setenv(name, "")
	}
	cases := []struct {
		name, origin, repository, want string
	}{
		{"http origin", "http://gitlab.example.test", "group/project", "HTTPS authority"},
		{"origin path", "https://gitlab.example.test/api", "group/project", "HTTPS authority"},
		{"repository edge slash", "https://gitlab.example.test", "/group/project", "namespace/project path"},
		{"repository query", "https://gitlab.example.test", "group/project?x", "namespace/project path"},
		{"repository empty segment", "https://gitlab.example.test", "group//project", "namespace/project path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", tc.origin)
			t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", tc.repository)
			if err := ValidateSources(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
	t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", "")
	t.Setenv("AIGW_GITHUB_RELEASE_ORIGIN", "https://github.example.test")
	t.Setenv("AIGW_GITHUB_RELEASE_REPOSITORY", "group/subgroup/project")
	if err := ValidateSources(); err == nil || !strings.Contains(err.Error(), "owner/repository") {
		t.Fatalf("nested GitHub repository error = %v", err)
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

func TestReadProductVersionRejectsMissingAndMalformedCarrier(t *testing.T) {
	missing := t.TempDir()
	if _, err := readProductVersion(missing); err == nil || !strings.Contains(err.Error(), "read VERSION") {
		t.Fatalf("missing VERSION error = %v", err)
	}
	t.Setenv("CI_COMMIT_TAG", "v1.2.3")
	if err := buildCI(missing, t.TempDir(), t.TempDir(), nil, nil, nil); err == nil || !strings.Contains(err.Error(), "read VERSION") {
		t.Fatalf("missing CI VERSION error = %v", err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("not-semver\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readProductVersion(root); err == nil || !strings.Contains(err.Error(), "invalid release version") {
		t.Fatalf("malformed VERSION error = %v", err)
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
	err := buildRelease(buildRequest{Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400", GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/aigw-cli", GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli"}, func(call toolCall) error {
		if call.Name == "goreleaser" {
			stage := goReleaserStage(t, call.Args)
			if mkdirErr := os.MkdirAll(stage, 0o700); mkdirErr != nil {
				return mkdirErr
			}
			for _, name := range artifact.Names("1.2.3") {
				if name == "checksums.txt" || strings.HasSuffix(name, ".spdx.json") {
					continue
				}
				if writeErr := os.WriteFile(filepath.Join(stage, name), []byte(name), 0o600); writeErr != nil {
					return writeErr
				}
			}
			binary := filepath.Join(stage, "portable_darwin_arm64_v8.0", "aigw")
			if mkdirErr := os.MkdirAll(filepath.Dir(binary), 0o700); mkdirErr != nil {
				return mkdirErr
			}
			return os.WriteFile(binary, []byte("binary"), 0o700)
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
