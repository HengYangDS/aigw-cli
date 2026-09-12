package construction

import (
	"aigw-cli/tools/release/artifact"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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
			for _, name := range artifact.Archives(valid.Version) {
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

	t.Run("output parent collision", func(t *testing.T) {
		request := valid
		parent := filepath.Join(t.TempDir(), "not-a-directory")
		if err := os.WriteFile(parent, []byte("collision"), 0o600); err != nil {
			t.Fatal(err)
		}
		request.Output = filepath.Join(parent, "dist")
		if err := buildRelease(request, func(toolCall) error { return nil }); err == nil || !strings.Contains(err.Error(), "output parent") {
			t.Fatalf("output parent collision error = %v", err)
		}
	})

	t.Run("missing GoReleaser configuration", func(t *testing.T) {
		request := valid
		request.Root = t.TempDir()
		request.Output = filepath.Join(t.TempDir(), "dist")
		if err := buildRelease(request, func(toolCall) error { return nil }); err == nil || !strings.Contains(err.Error(), "GoReleaser config") {
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
		err := buildRelease(valid, func(call toolCall) error {
			if call.Name == "goreleaser" {
				return populatePortableStage(t, call, valid.Version, "portable_linux_amd64/aigw")
			}
			raw := strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")
			candidate := filepath.Join(filepath.Dir(filepath.Dir(raw)), "artifacts")
			if err := os.Remove(filepath.Join(candidate, artifact.Names(valid.Version)[0])); err != nil {
				return err
			}
			return os.WriteFile(raw, []byte(`{"spdxVersion":"SPDX-2.3"}`), 0o600)
		})
		if err == nil {
			t.Fatal("missing checksum input was accepted")
		}
	})

	t.Run("unexpected matrix entry", func(t *testing.T) {
		err := buildRelease(valid, func(call toolCall) error {
			if call.Name == "goreleaser" {
				return populatePortableStage(t, call, valid.Version, "portable_linux_amd64/aigw")
			}
			raw := strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")
			candidate := filepath.Join(filepath.Dir(filepath.Dir(raw)), "artifacts")
			if err := os.WriteFile(filepath.Join(candidate, "unexpected.bin"), []byte("unexpected"), 0o600); err != nil {
				return err
			}
			return os.WriteFile(raw, []byte(`{"spdxVersion":"SPDX-2.3"}`), 0o600)
		})
		if err == nil || !strings.Contains(err.Error(), "unexpected") {
			t.Fatalf("unexpected matrix error = %v", err)
		}
	})
}

func TestReleaseBuildPropagatesPostBuildValidationFailures(t *testing.T) {
	valid := buildRequest{
		Root: releaseRoot(t), Output: filepath.Join(t.TempDir(), "dist"), Version: "1.2.3", Epoch: "1784246400",
		GitLabOrigin: "https://gitlab.example", GitLabRepository: "group/aigw-cli",
		GitHubOrigin: "https://github.example", GitHubRepository: "org/aigw-cli",
		SigningKey: "key",
	}
	populate := func(call toolCall) error {
		if call.Name == "goreleaser" {
			return populatePortableStage(t, call, valid.Version, "portable_linux_amd64/aigw")
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
	if err := executeTool(command); err != nil {
		t.Fatal(err)
	}
	if err := executeTool(toolCall{Name: filepath.Join(root, "missing-command")}); err == nil {
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
	request, err := buildRequestFromEnvironment("dist")
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
	if _, err := buildRequestFromEnvironment("dist"); err == nil || !strings.Contains(err.Error(), "read VERSION") {
		t.Fatalf("missing VERSION error = %v", err)
	}
	missingChronology := t.TempDir()
	if err := os.WriteFile(filepath.Join(missingChronology, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(missingChronology); err != nil {
		t.Fatal(err)
	}
	if _, err := buildRequestFromEnvironment("dist"); err == nil || !strings.Contains(err.Error(), "open CHANGELOG") {
		t.Fatalf("missing release chronology error = %v", err)
	}
}

func TestPortableBinaryDiscovery(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this fixture provides a Unix executable")
	}
	stage := t.TempDir()
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
	if _, err := resolveReleaseEpoch(root, "1.2.3"); err == nil {
		t.Fatal("invalid release date was accepted")
	}
	if err := os.WriteFile(changelog, []byte(strings.Repeat("x", 70*1024)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveReleaseEpoch(root, "1.2.3"); err == nil || !strings.Contains(err.Error(), "token too long") {
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
		{"missing origin", "", "group/project", "origin and repository together"},
		{"missing repository", "https://gitlab.example.test", "", "origin and repository together"},
		{"http origin", "http://gitlab.example.test", "group/project", "HTTPS authority"},
		{"origin path", "https://gitlab.example.test/api", "group/project", "HTTPS authority"},
		{"empty hostname", "https://:443", "group/project", "HTTPS authority"},
		{"empty query", "https://gitlab.example.test?", "group/project", "HTTPS authority"},
		{"empty fragment", "https://gitlab.example.test#", "group/project", "HTTPS authority"},
		{"repeated root slash", "https://gitlab.example.test///", "group/project", "HTTPS authority"},
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
			err := buildRelease(request, func(toolCall) error { calls++; return unexpected })
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
