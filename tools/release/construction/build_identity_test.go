package construction

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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
	output := filepath.Join(t.TempDir(), "nested", "release", "dist")
	var epochs []string
	build := func(request buildRequest) error {
		if info, err := os.Stat(filepath.Dir(output)); err != nil || !info.IsDir() {
			t.Fatalf("output parent was not prepared before building: %v", err)
		}
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
	if err := buildCI(root, filepath.Join(t.TempDir(), "build"), output, build, epoch, func(left, right, version string) error {
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
	if data, err := os.ReadFile(filepath.Join(output, "artifact")); err != nil || string(data) != "same" {
		t.Fatalf("published artifact=%q: %v", data, err)
	}
}

func TestBuildCIRejectsInvalidIdentity(t *testing.T) {
	for _, test := range []struct{ name, tag, version, want string }{
		{"missing carrier", "v1.2.3", "", "read VERSION"},
		{"mismatched carrier", "v1.2.3", "1.2.4", "VERSION"},
		{"unprefixed tag", "1.2.3", "1.2.3", "invalid CI"},
		{"malformed tag", "vnot-semver", "1.2.3", "invalid CI"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.version != "" {
				if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte(test.version+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("CI_COMMIT_TAG", test.tag)
			if err := buildCI(root, t.TempDir(), t.TempDir(), nil, nil, nil); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("invalid CI identity error = %v, want %q", err, test.want)
			}
		})
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
	occupied := filepath.Join(root, "occupied")
	if err := os.WriteFile(occupied, []byte("preserved"), 0o600); err != nil {
		t.Fatal(err)
	}
	failingBuild := func(buildRequest) error {
		calls++
		return want
	}
	if err := buildCI(root, workspace, filepath.Join(occupied, "dist"), failingBuild, epoch, nil); err == nil || !strings.Contains(err.Error(), "prepare release output parent") || calls != 0 {
		t.Fatalf("output admission error=%v build calls=%d", err, calls)
	}
	if data, err := os.ReadFile(occupied); err != nil || string(data) != "preserved" {
		t.Fatalf("output admission changed the existing file: %q, %v", data, err)
	}
	if err := buildCI(root, workspace, output, failingBuild, epoch, nil); !errors.Is(err, want) || calls != 1 {
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
	if err := os.WriteFile(changelog, []byte("# Changelog\n\nThis project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/).\n\n## [Unreleased]\n\n## [1.2.3] - 2026-08-09\n\n### Fixed\n\n- Fix.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if epoch, err := resolveReleaseEpoch(t.Context(), root, "1.2.3"); err != nil || epoch != "1786233600" {
		t.Fatalf("local release epoch=%q error=%v", epoch, err)
	}

	t.Setenv("CI_COMMIT_TAG", "v1.2.3")
	if epoch, err := resolveReleaseEpoch(t.Context(), root, "1.2.3"); err != nil || epoch != "1786233600" {
		t.Fatalf("tagged release epoch=%q error=%v", epoch, err)
	}
	if _, err := resolveReleaseEpoch(t.Context(), root, "9.9.9"); err == nil || !strings.Contains(err.Error(), "heading not found") {
		t.Fatalf("missing heading error = %v", err)
	}
	if _, err := resolveReleaseEpoch(t.Context(), t.TempDir(), "1.2.3"); err == nil || !strings.Contains(err.Error(), "open CHANGELOG") {
		t.Fatalf("missing changelog error=%v", err)
	}
}

func TestCandidateBuildEpochUsesSourceCommitWithoutInventingARelease(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("# Changelog\n\nThis project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/).\n\n## [Unreleased]\n\n## [1.2.3] - 2026-08-09\n\n### Fixed\n\n- Fix.\n"), 0o600); err != nil {
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
	if epoch, err := resolveReleaseEpoch(t.Context(), root, "1.2.4"); err != nil || epoch != "1788739200" {
		t.Fatalf("candidate source epoch=%q error=%v", epoch, err)
	}
	t.Setenv("CI_COMMIT_TAG", "v1.2.4")
	if _, err := resolveReleaseEpoch(t.Context(), root, "1.2.4"); err == nil {
		t.Fatal("tagged release accepted without its release chronicle")
	}
}
