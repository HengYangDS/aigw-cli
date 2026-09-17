package construction

import (
	"aigw-cli/tools/release/artifact"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseToolOwnsOutputPipesAndExplicitContext(t *testing.T) {
	if os.Getenv("AIGW_TEST_RELEASE_TOOL") == "child" {
		for _, output := range []*os.File{os.Stdout, os.Stderr} {
			info, err := output.Stat()
			if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
				os.Exit(24)
			}
		}
		input, err := io.ReadAll(os.Stdin)
		if err != nil || len(input) != 0 {
			os.Exit(25)
		}
		data, err := os.ReadFile("marker")
		if err != nil {
			os.Exit(26)
		}
		_, _ = fmt.Fprintf(os.Stdout, "%s:%s", data, os.Getenv("AIGW_TEST_RELEASE_VALUE"))
		os.Exit(0)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "marker"), []byte("owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	output, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = output.Close() })
	err = executeTool(t.Context())(toolCall{
		Name: executable, Directory: root,
		Args:   []string{"-test.run=^TestReleaseToolOwnsOutputPipesAndExplicitContext$"},
		Env:    []string{"AIGW_TEST_RELEASE_TOOL=child", "AIGW_TEST_RELEASE_VALUE=explicit"},
		Stdout: output,
	})
	data, readErr := os.ReadFile(output.Name())
	if err != nil || readErr != nil || string(data) != "owned:explicit" {
		t.Fatalf("release tool output=%q, execution=%v, read=%v", data, err, readErr)
	}
}

func TestReleaseToolCancellationStopsBeforeExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	err = executeTool(ctx)(toolCall{Name: executable, Args: []string{"-test.run=^TestReleaseToolCancellationStopsBeforeExecution$"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("release tool lost cancellation: %v", err)
	}
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

func TestInternalReleaseNeedsNoPublisherCredentials(t *testing.T) {
	root := releaseRoot(t)
	for _, name := range []string{"AIGW_MACOS_SIGNING_P12", "AIGW_MACOS_SIGNING_PASSWORD_FILE", "AIGW_MACOS_SIGNING_REQUIREMENTS"} {
		t.Setenv(name, "")
	}
	want := errors.New("source inspection reached")
	calls := 0
	err := buildRelease(t.Context(), buildRequest{Root: root, Output: filepath.Join(root, "dist"), Version: "1.2.3", Epoch: "0", SigningKey: "synthetic"}, func(call toolCall) error {
		calls++
		if call.Name != "git" {
			t.Fatalf("first boundary = %s, want source inspection", call.Name)
		}
		return want
	})
	if !errors.Is(err, want) || calls != 1 {
		t.Fatalf("internal release blocked before source inspection: err=%v calls=%d", err, calls)
	}
}

func TestReleaseBuildBoundaryFailures(t *testing.T) {
	valid := buildRequest{
		Root: releaseRoot(t), Output: filepath.Join(t.TempDir(), "dist"), Version: "1.2.3", Epoch: "1784246400",
		GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/aigw-cli",
		GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli",
		SigningKey: "key",
	}

	t.Run("repository whitespace", func(t *testing.T) {
		request := valid
		request.GitHubRepository = "org/aigw cli"
		if err := validateRequest(request); err == nil || !strings.Contains(err.Error(), "namespace/project path") {
			t.Fatalf("whitespace error = %v", err)
		}
	})

	t.Run("GoReleaser failure", func(t *testing.T) {
		want := errors.New("goreleaser failed")
		if err := buildRelease(t.Context(), valid, func(toolCall) error { return want }); !errors.Is(err, want) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("candidate directory collision", func(t *testing.T) {
		request := valid
		request.Output = filepath.Join(t.TempDir(), "dist")
		err := buildRelease(t.Context(), request, func(call toolCall) error {
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
		err := buildRelease(t.Context(), valid, func(call toolCall) error {
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
		err := buildRelease(t.Context(), valid, func(call toolCall) error {
			if call.Name == "syft" {
				path := strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")
				return os.WriteFile(path, []byte(`{"spdxVersion":"SPDX-2.3"}`), 0o600)
			}
			if call.Name != "goreleaser" {
				return nil
			}
			stage := goReleaserStage(t, call.Args)
			if err := os.MkdirAll(stage, 0o700); err != nil {
				return err
			}
			for _, name := range artifact.Archives(valid.Version) {
				if err := os.WriteFile(filepath.Join(stage, name), []byte(name), 0o600); err != nil {
					return err
				}
			}
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "binary matrix") {
			t.Fatalf("missing binary error = %v", err)
		}
	})

	t.Run("output parent collision", func(t *testing.T) {
		request := valid
		parent := filepath.Join(t.TempDir(), "not-a-directory")
		if err := os.WriteFile(parent, []byte("collision"), 0o600); err != nil {
			t.Fatal(err)
		}
		request.Output = filepath.Join(parent, "dist")
		if err := buildRelease(t.Context(), request, func(toolCall) error { return nil }); err == nil || !strings.Contains(err.Error(), "output parent") {
			t.Fatalf("output parent collision error = %v", err)
		}
	})

	t.Run("missing GoReleaser configuration", func(t *testing.T) {
		request := valid
		request.Root = t.TempDir()
		request.Output = filepath.Join(t.TempDir(), "dist")
		if err := buildRelease(t.Context(), request, func(toolCall) error { return nil }); err == nil || !strings.Contains(err.Error(), "GoReleaser config") {
			t.Fatalf("missing configuration error = %v", err)
		}
	})
}

func populatePortableStage(t *testing.T, call toolCall, version, executable string) error {
	t.Helper()
	stage := goReleaserStage(t, call.Args)
	if err := os.MkdirAll(stage, 0o700); err != nil {
		return err
	}
	for _, name := range artifact.Archives(version) {
		if err := os.WriteFile(filepath.Join(stage, name), []byte(name), 0o600); err != nil {
			return err
		}
	}
	binary := filepath.Join(stage, filepath.FromSlash(executable))
	if err := os.MkdirAll(filepath.Dir(binary), 0o700); err != nil {
		return err
	}
	return os.WriteFile(binary, []byte("binary"), 0o700)
}

func TestReleaseBuildPropagatesChecksumAndMatrixFailures(t *testing.T) {
	valid := buildRequest{Root: releaseRoot(t), Output: filepath.Join(t.TempDir(), "dist"), Version: "1.2.3", Epoch: "1784246400", SigningKey: "key"}

	t.Run("checksum input disappears", func(t *testing.T) {
		err := buildRelease(t.Context(), valid, func(call toolCall) error {
			if call.Name == "goreleaser" {
				return populatePortableStage(t, call, valid.Version, "portable_linux_amd64/aigw")
			}
			raw := strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")
			candidate := filepath.Join(filepath.Dir(filepath.Dir(raw)), "artifacts")
			if err := os.Remove(filepath.Join(candidate, artifact.Names(valid.Version)[0])); err != nil {
				return err
			}
			return os.WriteFile(raw, spdxFixture(t, filepath.Dir(raw), "1.2.3"), 0o600)
		})
		if err == nil {
			t.Fatal("missing checksum input was accepted")
		}
	})

	t.Run("unexpected matrix entry", func(t *testing.T) {
		err := buildRelease(t.Context(), valid, func(call toolCall) error {
			if call.Name == "goreleaser" {
				return populatePortableStage(t, call, valid.Version, "portable_linux_amd64/aigw")
			}
			raw := strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")
			candidate := filepath.Join(filepath.Dir(filepath.Dir(raw)), "artifacts")
			if err := os.WriteFile(filepath.Join(candidate, "unexpected.bin"), []byte("unexpected"), 0o600); err != nil {
				return err
			}
			return os.WriteFile(raw, spdxFixture(t, filepath.Dir(raw), "1.2.3"), 0o600)
		})
		if err == nil || !strings.Contains(err.Error(), "unexpected") {
			t.Fatalf("unexpected matrix error = %v", err)
		}
	})
}

func TestReleaseBuildPropagatesPostBuildValidationFailures(t *testing.T) {
	for _, boundary := range []string{"decode Syft", "selected lockfiles"} {
		t.Run(boundary, func(t *testing.T) {
			root := releaseRoot(t)
			output := filepath.Join(root, "dist")
			if err := os.Mkdir(output, 0o700); err != nil {
				t.Fatal(err)
			}
			accepted := filepath.Join(output, "accepted")
			if err := os.WriteFile(accepted, []byte("previous release"), 0o600); err != nil {
				t.Fatal(err)
			}
			valid := buildRequest{Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400", SigningKey: "unused"}
			err := buildRelease(t.Context(), valid, func(call toolCall) error {
				switch call.Name {
				case "git":
					return nil
				case "goreleaser":
					return populatePortableStage(t, call, valid.Version, "portable_linux_amd64/aigw")
				case "syft":
					data := spdxFixture(t, filepath.Dir(strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")), valid.Version)
					if boundary == "decode Syft" {
						data = []byte("{")
					}
					return os.WriteFile(strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json="), data, 0o600)
				case "osv-scanner":
					return os.WriteFile(call.Args[len(call.Args)-1], []byte(`{"results":[]}`), 0o600)
				default:
					t.Fatalf("invalid evidence reached later release tool %s", call.Name)
					return nil
				}
			})
			if err == nil || !strings.Contains(err.Error(), boundary) {
				t.Fatalf("release validation error = %v, want %s", err, boundary)
			}
			if data, err := os.ReadFile(accepted); err != nil || string(data) != "previous release" {
				t.Fatalf("invalid evidence changed accepted output: %q, %v", data, err)
			}
			entries, err := os.ReadDir(output)
			if err != nil || len(entries) != 1 {
				t.Fatalf("partial evidence published: %v, %v", entries, err)
			}
			workspaces, err := filepath.Glob(filepath.Join(root, ".aigw-release-*"))
			if err != nil || len(workspaces) != 0 {
				t.Fatalf("failed release left workspace residue: %v, %v", workspaces, err)
			}
		})
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
	foreign := target + ".previous"
	if err := os.WriteFile(foreign, []byte("operator-owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceDirectory(source, target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "new")); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(foreign); err != nil || string(content) != "operator-owned" {
		t.Fatalf("replacement changed an unowned sibling: %q, %v", content, err)
	}

	missing := filepath.Join(root, "missing")
	if err := replaceDirectory(missing, target); err == nil || !strings.Contains(err.Error(), "publish release output") {
		t.Fatalf("replace failure = %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "new")); err != nil {
		t.Fatalf("previous output was not restored: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("replacement left temporary output: %v, %v", entries, err)
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
	if err := executeTool(t.Context())(command); err != nil {
		t.Fatal(err)
	}
	if err := executeTool(t.Context())(toolCall{Name: filepath.Join(root, "missing-command")}); err == nil {
		t.Fatal("missing command succeeded")
	}
}

func TestReleaseBuildEnvironment(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("## [1.2.3] - 2026-08-09\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	for name, value := range map[string]string{
		"AIGW_GITLAB_RELEASE_ORIGIN":     "https://gitlab.example",
		"AIGW_GITLAB_RELEASE_REPOSITORY": "group/aigw-cli",
		"AIGW_GITHUB_RELEASE_ORIGIN":     "https://github.example",
		"AIGW_GITHUB_RELEASE_REPOSITORY": "org/aigw-cli",
	} {
		t.Setenv(name, value)
	}
	request, err := buildRequestFromEnvironment(t.Context(), "dist")
	if err != nil {
		t.Fatal(err)
	}
	requestRoot, requestRootErr := os.Stat(request.Root)
	wantRoot, wantRootErr := os.Stat(root)
	if request.Version != "1.2.3" || request.Epoch != "1786233600" || request.Output != "dist" || requestRootErr != nil || wantRootErr != nil || !os.SameFile(requestRoot, wantRoot) {
		t.Fatalf("request = %#v", request)
	}
	missingVersion := t.TempDir()
	if err := os.Chdir(missingVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := buildRequestFromEnvironment(t.Context(), "dist"); err == nil || !strings.Contains(err.Error(), "read VERSION") {
		t.Fatalf("missing VERSION error = %v", err)
	}
	missingChronology := t.TempDir()
	if err := os.WriteFile(filepath.Join(missingChronology, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(missingChronology); err != nil {
		t.Fatal(err)
	}
	if _, err := buildRequestFromEnvironment(t.Context(), "dist"); err == nil || !strings.Contains(err.Error(), "open CHANGELOG") {
		t.Fatalf("missing release chronology error = %v", err)
	}
}

func TestBuildCIRejectsMalformedTagShapes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"1.2.3", "vnot-semver"} {
		t.Run(tag, func(t *testing.T) {
			t.Setenv("CI_COMMIT_TAG", tag)
			if err := buildCI(root, t.TempDir(), t.TempDir(), nil, nil, nil); err == nil || !strings.Contains(err.Error(), "invalid CI") {
				t.Fatalf("tag %q error = %v", tag, err)
			}
		})
	}
}

func TestReleaseEpochRejectsInvalidDateAndOversizedChangelogLine(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CI_COMMIT_TAG", "v1.2.3")
	changelog := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(changelog, []byte("## [1.2.3] - 2026-99-99\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveReleaseEpoch(t.Context(), root, "1.2.3"); err == nil {
		t.Fatal("invalid release date was accepted")
	}
	if err := os.WriteFile(changelog, []byte(strings.Repeat("x", 70*1024)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveReleaseEpoch(t.Context(), root, "1.2.3"); err == nil || !strings.Contains(err.Error(), "token too long") {
		t.Fatalf("oversized changelog error = %v", err)
	}
}

func TestValidateSourcesRejectsInvalidAuthoritiesAndRepositories(t *testing.T) {
	for _, name := range []string{"AIGW_GITLAB_RELEASE_ORIGIN", "AIGW_GITLAB_RELEASE_REPOSITORY", "AIGW_GITHUB_RELEASE_ORIGIN", "AIGW_GITHUB_RELEASE_REPOSITORY"} {
		t.Setenv(name, "")
	}
	cases := []struct {
		name, origin, repository, want string
	}{
		{"missing origin", "", "group/project", "release source is incomplete"},
		{"missing repository", "https://gitlab.example.test", "", "release source is incomplete"},
		{"public http origin", "http://gitlab.example.com", "group/project", "must use HTTPS"},
		{"origin path", "https://gitlab.example.test/api", "group/project", "HTTP(S) origin"},
		{"empty hostname", "https://:443", "group/project", "HTTP(S) origin"},
		{"empty query", "https://gitlab.example.test?", "group/project", "HTTP(S) origin"},
		{"empty fragment", "https://gitlab.example.test#", "group/project", "HTTP(S) origin"},
		{"repeated root slash", "https://gitlab.example.test///", "group/project", "HTTP(S) origin"},
		{"repository edge slash", "https://gitlab.example.test", "/group/project", "namespace/project path"},
		{"repository query", "https://gitlab.example.test", "group/project?x", "namespace/project path"},
		{"repository empty segment", "https://gitlab.example.test", "group//project", "namespace/project path"},
		{"repository missing namespace", "https://gitlab.example.test", "project", "namespace/project path"},
		{"repository escaped segment", "https://gitlab.example.test", "group/%2e%2e", "namespace/project path"},
		{"repository escaped separator", "https://gitlab.example.test", "group/project%2Fother", "namespace/project path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", tc.origin)
			t.Setenv("AIGW_GITLAB_RELEASE_REPOSITORY", tc.repository)
			if err := ValidateSources(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("source check error = %v, want %q", err, tc.want)
			}
			request := buildRequest{
				Version: "1.2.3", Epoch: "0", SigningKey: "key",
				GitLabOrigin: tc.origin, GitLabRepository: tc.repository,
			}
			calls := 0
			unexpected := errors.New("construction reached external tool execution")
			err := buildRelease(t.Context(), request, func(toolCall) error { calls++; return unexpected })
			if err == nil || !strings.Contains(err.Error(), tc.want) || calls != 0 {
				t.Fatalf("build error=%v tool calls=%d, want %q before execution", err, calls, tc.want)
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
	if err := validateRequest(buildRequest{
		Version: "1.2.3", Epoch: "0", GitHubOrigin: "https://github.example.test", GitHubRepository: "group/subgroup/project",
	}); err == nil || !strings.Contains(err.Error(), "owner/repository") {
		t.Fatalf("nested GitHub build repository error = %v", err)
	}
}
