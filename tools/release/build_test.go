package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

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
			for _, name := range portableArtifactNames("1.2.3") {
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
	if err := validatePortableArtifactMatrix(output, "1.2.3"); err != nil {
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
			if err := validateBuildRequest(request); err != nil {
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

func TestResolveReleaseEpochUsesCommitOrChangelogAuthority(t *testing.T) {
	root := t.TempDir()
	command := exec.Command("git", "init", "-q", root)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	for _, args := range [][]string{{"-C", root, "config", "user.name", "Fixture"}, {"-C", root, "config", "user.email", "fixture@example.test"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git config: %v: %s", err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-C", root, "add", "README.md"}, {"-C", root, "commit", "-q", "-m", "fixture"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git commit: %v: %s", err, output)
		}
	}
	t.Setenv("CI_COMMIT_TAG", "")
	if epoch, err := resolveReleaseEpoch(root, "0.1.0-dev"); err != nil || epoch == "" {
		t.Fatalf("commit epoch=%q error=%v", epoch, err)
	}
	if _, err := resolveReleaseEpoch(t.TempDir(), "0.1.0-dev"); err == nil || !strings.Contains(err.Error(), "read source commit epoch") {
		t.Fatalf("missing commit error = %v", err)
	}

	t.Setenv("CI_COMMIT_TAG", "v1.2.3")
	if _, err := resolveReleaseEpoch(t.TempDir(), "1.2.3"); err == nil || !strings.Contains(err.Error(), "open CHANGELOG") {
		t.Fatalf("missing changelog error = %v", err)
	}
	changelog := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(changelog, []byte("## [1.2.3] - 2026-08-09\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if epoch, err := resolveReleaseEpoch(root, "1.2.3"); err != nil || epoch != "1786233600" {
		t.Fatalf("release epoch=%q error=%v", epoch, err)
	}
	if _, err := resolveReleaseEpoch(root, "9.9.9"); err == nil || !strings.Contains(err.Error(), "heading not found") {
		t.Fatalf("missing heading error = %v", err)
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
			for _, name := range portableArtifactNames("1.2.3") {
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

func TestNormalizedSPDXIsDeterministicAndPortable(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "raw.json")
	raw := `{"spdxVersion":"SPDX-2.3","documentNamespace":"https://volatile.invalid/random","creationInfo":{"created":"2099-01-01T00:00:00Z"},"files":[{"fileName":"aigw","sourceInfo":"acquired from /aigw"}]}`
	if err := os.WriteFile(source, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	instant, err := parseEpoch("1784246400")
	if err != nil {
		t.Fatal(err)
	}
	left, right := filepath.Join(root, "left.json"), filepath.Join(root, "right.json")
	if err := normalizeSPDX(source, left, "1.2.3", instant); err != nil {
		t.Fatal(err)
	}
	if err := normalizeSPDX(source, right, "1.2.3", instant); err != nil {
		t.Fatal(err)
	}
	leftData, _ := os.ReadFile(left)
	rightData, _ := os.ReadFile(right)
	if !bytes.Equal(leftData, rightData) {
		t.Fatal("normalized SPDX differs across equivalent runs")
	}
	for _, forbidden := range []string{"/Users/", "/private/tmp/"} {
		if bytes.Contains(leftData, []byte(forbidden)) {
			t.Fatalf("normalized SPDX leaks host path %q", forbidden)
		}
	}
	hostLocal := filepath.Join(root, "host-local.json")
	if err := os.WriteFile(hostLocal, []byte(`{"sourceInfo":"/Users/alice/project"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := normalizeSPDX(hostLocal, filepath.Join(root, "rejected.json"), "1.2.3", instant); err == nil {
		t.Fatal("host-local SPDX path was accepted")
	}
}

func TestPortableArtifactMatrixRejectsNativePackagesAndCorruption(t *testing.T) {
	directory := t.TempDir()
	version := "1.2.3"
	for _, name := range portableArtifactNames(version) {
		if strings.HasSuffix(name, ".spdx.json") {
			if err := os.WriteFile(filepath.Join(directory, name), []byte(`{"spdxVersion":"SPDX-2.3"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if name != "checksums.txt" {
			if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := rewritePortableChecksums(directory, version); err != nil {
		t.Fatal(err)
	}
	if err := validatePortableArtifactMatrix(directory, version); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "aigw_1.2.3_linux_amd64.deb"), []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validatePortableArtifactMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("native package accepted: %v", err)
	}
	if err := os.Remove(filepath.Join(directory, "aigw_1.2.3_linux_amd64.deb")); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(directory, "aigw_1.2.3_linux_amd64.tar.gz")
	if err := os.WriteFile(archive, bytes.Repeat([]byte("x"), 3), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validatePortableArtifactMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("corruption accepted: %v", err)
	}
}

func TestReleaseBuildBoundaryFailures(t *testing.T) {
	valid := buildRequest{
		Root: releaseRoot(t), Output: filepath.Join(t.TempDir(), "dist"), Version: "1.2.3", Epoch: "1784246400",
		GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/aigw-cli",
		GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli",
	}

	t.Run("repository whitespace", func(t *testing.T) {
		request := valid
		request.GitHubRepository = "org/aigw cli"
		if err := validateBuildRequest(request); err == nil || !strings.Contains(err.Error(), "whitespace") {
			t.Fatalf("whitespace error = %v", err)
		}
	})

	t.Run("GoReleaser failure", func(t *testing.T) {
		want := errors.New("goreleaser failed")
		if err := buildRelease(valid, func(toolCall) error { return want }); !errors.Is(err, want) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("candidate directory collision", func(t *testing.T) {
		request := valid
		request.Output = filepath.Join(t.TempDir(), "dist")
		err := buildRelease(request, func(call toolCall) error {
			if call.Name != "goreleaser" {
				return nil
			}
			stage := goReleaserStage(t, call.Args)
			candidate := filepath.Join(filepath.Dir(stage), "artifacts")
			return os.WriteFile(candidate, []byte("collision"), 0o600)
		})
		if err == nil || !strings.Contains(err.Error(), "create release candidate") {
			t.Fatalf("candidate collision error = %v", err)
		}
	})

	t.Run("missing GoReleaser artifact", func(t *testing.T) {
		err := buildRelease(valid, func(call toolCall) error {
			if call.Name == "goreleaser" {
				return os.MkdirAll(goReleaserStage(t, call.Args), 0o700)
			}
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "read release artifact") {
			t.Fatalf("missing artifact error = %v", err)
		}
	})

	t.Run("missing portable binary", func(t *testing.T) {
		err := buildRelease(valid, func(call toolCall) error {
			if call.Name != "goreleaser" {
				return nil
			}
			stage := goReleaserStage(t, call.Args)
			if err := os.MkdirAll(stage, 0o700); err != nil {
				return err
			}
			for _, name := range portableArtifactNames(valid.Version) {
				if name == "checksums.txt" || strings.HasSuffix(name, ".spdx.json") {
					continue
				}
				if err := os.WriteFile(filepath.Join(stage, name), []byte(name), 0o600); err != nil {
					return err
				}
			}
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "no portable binary") {
			t.Fatalf("missing binary error = %v", err)
		}
	})
}

func TestReleaseBuildPropagatesPostBuildValidationFailures(t *testing.T) {
	valid := buildRequest{
		Root: releaseRoot(t), Output: filepath.Join(t.TempDir(), "dist"), Version: "1.2.3", Epoch: "1784246400",
		GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/aigw-cli",
		GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli",
	}
	populate := func(call toolCall) error {
		if call.Name == "goreleaser" {
			stage := goReleaserStage(t, call.Args)
			if err := os.MkdirAll(stage, 0o700); err != nil {
				return err
			}
			for _, name := range portableArtifactNames(valid.Version) {
				if name == "checksums.txt" || strings.HasSuffix(name, ".spdx.json") {
					continue
				}
				if err := os.WriteFile(filepath.Join(stage, name), []byte(name), 0o600); err != nil {
					return err
				}
			}
			binary := filepath.Join(stage, "portable_linux_amd64", "aigw")
			if err := os.MkdirAll(filepath.Dir(binary), 0o700); err != nil {
				return err
			}
			return os.WriteFile(binary, []byte("binary"), 0o700)
		}
		if call.Name == "syft" {
			path := strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")
			return os.WriteFile(path, []byte("{"), 0o600)
		}
		return nil
	}
	if err := buildRelease(valid, populate); err == nil || !strings.Contains(err.Error(), "decode Syft") {
		t.Fatalf("SPDX normalization error = %v", err)
	}
}

func TestReleaseBuildHelpersCoverAtomicReplacementAndCommands(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "new"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "old"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceDirectory(source, target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "new")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target + ".previous"); !os.IsNotExist(err) {
		t.Fatalf("backup remains: %v", err)
	}

	missing := filepath.Join(root, "missing")
	if err := replaceDirectory(missing, target); err == nil || !strings.Contains(err.Error(), "publish release output") {
		t.Fatalf("replace failure = %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "new")); err != nil {
		t.Fatalf("previous output was not restored: %v", err)
	}

	copyTarget := filepath.Join(root, "copied")
	if err := copyFile(filepath.Join(target, "new"), copyTarget); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(filepath.Join(root, "absent"), copyTarget); err == nil || !strings.Contains(err.Error(), "read release artifact") {
		t.Fatalf("copy read error = %v", err)
	}
	if err := copyFile(filepath.Join(target, "new"), root); err == nil || !strings.Contains(err.Error(), "write release artifact") {
		t.Fatalf("copy write error = %v", err)
	}

	command := toolCall{Name: "go", Directory: root, Args: []string{"version"}, Env: []string{"AIGW_TEST_VALUE=present"}}
	if err := execTool(command); err != nil {
		t.Fatal(err)
	}
	if err := execTool(toolCall{Name: filepath.Join(root, "missing-command")}); err == nil {
		t.Fatal("missing command succeeded")
	}
}

func TestArtifactComparisonAndChecksumReadFailures(t *testing.T) {
	if err := compareArtifactMatrices(filepath.Join(t.TempDir(), "missing"), t.TempDir(), "1.2.3"); err == nil {
		t.Fatal("missing left artifact matrix was accepted")
	}
	left := releaseFixture(t, "1.2.3")
	right := releaseFixture(t, "1.2.3")
	if err := os.WriteFile(filepath.Join(right, "aigw_1.2.3_linux_amd64.tar.gz"), []byte("different"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rewritePortableChecksums(right, "1.2.3"); err != nil {
		t.Fatal(err)
	}
	if err := compareArtifactMatrices(left, right, "1.2.3"); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("different artifact matrices error = %v", err)
	}
	if err := rewritePortableChecksums(t.TempDir(), "1.2.3"); err == nil {
		t.Fatal("missing checksum input was accepted")
	}
}

func TestArtifactMatrixReportsManifestAndComparisonReadFailures(t *testing.T) {
	version := "1.2.3"
	directory := releaseFixture(t, version)
	if err := os.Remove(filepath.Join(directory, "checksums.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, "checksums.txt"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := validatePortableArtifactMatrix(directory, version); err == nil {
		t.Fatal("unreadable checksum manifest accepted")
	}

	left := releaseFixture(t, version)
	right := releaseFixture(t, version)
	name := portableArtifactNames(version)[0]
	if err := os.Remove(filepath.Join(left, name)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(left, name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := compareArtifactMatrices(left, right, version); err == nil {
		t.Fatal("unreadable left artifact accepted")
	}
}

func TestArtifactComparisonValidatesRightSideBeforeReading(t *testing.T) {
	left := releaseFixture(t, "1.2.3")
	if err := compareArtifactMatrices(left, filepath.Join(t.TempDir(), "missing"), "1.2.3"); err == nil {
		t.Fatal("missing right artifact matrix was accepted")
	}
}

func TestReleaseBuildArgumentAndSPDXEdges(t *testing.T) {
	if _, err := parseBuildArguments(nil); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("argument error = %v", err)
	}
	if _, err := parseBuildArguments([]string{"1.2.3", "bad-epoch", "dist"}); err != nil {
		t.Fatalf("argument parsing should defer semantic validation: %v", err)
	}
	for name, value := range map[string]string{
		"AIGW_GITLAB_RELEASE_ORIGIN":     "https://gitlab.example",
		"AIGW_GITLAB_RELEASE_REPOSITORY": "group/aigw-cli",
		"AIGW_GITHUB_RELEASE_ORIGIN":     "https://github.example",
		"AIGW_GITHUB_RELEASE_REPOSITORY": "org/aigw-cli",
	} {
		t.Setenv(name, value)
	}
	request, err := parseBuildArguments([]string{"1.2.3", "1784246400", "dist"})
	if err != nil {
		t.Fatal(err)
	}
	if request.Version != "1.2.3" || request.Output != "dist" || request.Root == "" {
		t.Fatalf("request = %#v", request)
	}

	root := t.TempDir()
	instant := time.Unix(1784246400, 0).UTC()
	missingCreation := filepath.Join(root, "missing-creation.json")
	if err := os.WriteFile(missingCreation, []byte(`{"spdxVersion":"SPDX-2.3"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "normalized.json")
	if err := normalizeSPDX(missingCreation, target, "1.2.3", instant); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || !bytes.Contains(data, []byte(`"creationInfo"`)) {
		t.Fatalf("normalized SPDX = %s error=%v", data, err)
	}
	malformed := filepath.Join(root, "malformed.json")
	if err := os.WriteFile(malformed, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := normalizeSPDX(malformed, target, "1.2.3", instant); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("malformed SPDX error = %v", err)
	}
	if err := normalizeSPDX(filepath.Join(root, "absent.json"), target, "1.2.3", instant); err == nil || !strings.Contains(err.Error(), "read Syft") {
		t.Fatalf("missing SPDX error = %v", err)
	}

	if runtime.GOOS != "windows" {
		stage := filepath.Join(root, "stage")
		if err := os.MkdirAll(filepath.Join(stage, "portable_linux_amd64"), 0o700); err != nil {
			t.Fatal(err)
		}
		binary := filepath.Join(stage, "portable_linux_amd64", "aigw")
		if err := os.WriteFile(binary, []byte("binary"), 0o700); err != nil {
			t.Fatal(err)
		}
		if got, err := firstPortableBinary(stage); err != nil || got != binary {
			t.Fatalf("binary = %q error=%v", got, err)
		}
	}
}
