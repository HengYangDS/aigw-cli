package construction

import (
	"aigw-cli/tools/release/artifact"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

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
		SigningKey: signingKey(t),
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
			if call.Name == "osv-scanner" {
				return writeJSON(call.Args[len(call.Args)-1], dependencyReportFixture(valid.Root))
			}
			if call.Name != "goreleaser" {
				return nil
			}
			stage := goReleaserStage(t, call.Args)
			candidate := filepath.Join(filepath.Dir(stage), "artifacts")
			return os.WriteFile(candidate, []byte("collision"), 0o600)
		})
		if err == nil || !strings.Contains(err.Error(), "build portable release artifacts") {
			t.Fatalf("candidate collision error = %v", err)
		}
	})

	t.Run("missing GoReleaser artifact", func(t *testing.T) {
		err := buildRelease(t.Context(), valid, func(call toolCall) error {
			if call.Name == "osv-scanner" {
				return writeJSON(call.Args[len(call.Args)-1], dependencyReportFixture(valid.Root))
			}
			if call.Name == "goreleaser" {
				return os.MkdirAll(goReleaserStage(t, call.Args), 0o700)
			}
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "read release artifact") {
			t.Fatalf("missing artifact error = %v", err)
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
		policy := filepath.Join(request.Root, ".config", "checks", "dependencies", "policy.toml")
		if err := os.MkdirAll(filepath.Dir(policy), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(policy, []byte("PackageOverrides = []\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		request.Output = filepath.Join(t.TempDir(), "dist")
		if err := buildRelease(t.Context(), request, func(call toolCall) error {
			if call.Name == "osv-scanner" {
				return writeJSON(call.Args[len(call.Args)-1], dependencyReportFixture(request.Root))
			}
			return nil
		}); err == nil || !strings.Contains(err.Error(), "GoReleaser config") {
			t.Fatalf("missing configuration error = %v", err)
		}
	})
}

func populatePortableStage(t *testing.T, call toolCall, version, executable string) error {
	t.Helper()
	stage := goReleaserStage(t, call.Args)
	casks := filepath.Join(stage, "homebrew", "Casks")
	if err := os.MkdirAll(casks, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(casks, "aigw.rb"), []byte("generated cask"), 0o600); err != nil {
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

func TestReleaseOutputRejectsInvalidFirstPublicationPaths(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	target := filepath.Join(root, "unpublished")
	if err := replaceDirectory(missing, target); err == nil || !strings.Contains(err.Error(), "publish release output") {
		t.Fatalf("first publication accepted a missing source: %v", err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("failed publication left a target: %v", err)
	}
	if err := replaceDirectory(missing, filepath.Join(root, "missing-parent", "release")); err == nil || !strings.Contains(err.Error(), "inspect release output parent") {
		t.Fatalf("missing release parent was not rejected before publication: %v", err)
	}
	blocker := filepath.Join(root, "operator-owned")
	if err := os.WriteFile(blocker, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceDirectory(missing, filepath.Join(blocker, "unpublished")); err == nil || !strings.Contains(err.Error(), "inspect release output") {
		t.Fatalf("non-directory release parent was accepted: %v", err)
	}
	if content, err := os.ReadFile(blocker); err != nil || string(content) != "unchanged" {
		t.Fatalf("invalid release target changed an operator-owned file: %q, %v", content, err)
	}
}

func TestReleaseBuildEnvironment(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("# Changelog\n\nThis project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/).\n\n## Unreleased\n\n## 1.2.3 - 2026-08-09\n\n### Fixed\n\n- Fix.\n"), 0o600); err != nil {
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
	request.ReleasePublicKey, err = releasePublicKey(signingKey(t))
	if err != nil {
		t.Fatal(err)
	}
	request.SigningKey = filepath.Join(root, "missing.pub")
	environment, err := goReleaserEnvironment(request)
	if err != nil || !slices.Contains(environment, "AIGW_RELEASE_PUBLIC_KEY="+request.ReleasePublicKey) {
		t.Fatalf("frozen signer was reread or changed: %v, %v", environment, err)
	}
	request.ReleasePublicKey = ""
	if _, err := goReleaserEnvironment(request); err == nil || !strings.Contains(err.Error(), "read release signing public key") {
		t.Fatalf("unreadable signer entered release environment: %v", err)
	}
	request.Epoch = "invalid"
	if _, err := goReleaserEnvironment(request); err == nil {
		t.Fatal("invalid epoch entered release environment")
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

func TestReleaseEpochRejectsInvalidDateAndOversizedChangelogLine(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CI_COMMIT_TAG", "v1.2.3")
	changelog := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(changelog, []byte("## 1.2.3 - 2026-99-99\n"), 0o600); err != nil {
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
				Version: "1.2.3", Epoch: "0", SigningKey: signingKey(t),
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

func releaseFixtureRunner(t *testing.T, root string, calls *[]toolCall, fault *string) toolRunner {
	t.Helper()
	return func(call toolCall) error {
		*calls = append(*calls, call)
		if call.Name == "git" && slices.Contains(call.Args, "status") {
			return nil
		}
		if call.Name == "goreleaser" {
			return populatePortableStage(t, call, "1.2.3", "portable_darwin_arm64_v8.0/aigw")
		}
		if call.Name == "syft" {
			build := (*calls)[slices.IndexFunc(*calls, func(call toolCall) bool { return call.Name == "goreleaser" })]
			if call.Args[1] != "dir:"+goReleaserStage(t, build.Args) ||
				!slices.Contains(call.Args, "go-module-binary-cataloger,file") {
				t.Fatalf("SBOM must catalog the complete native binary matrix: %v", call.Args)
			}
			path := strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")
			if err := os.WriteFile(path, spdxFixture(t, filepath.Dir(path), "1.2.3"), 0o600); err != nil {
				return err
			}
			candidate := filepath.Join(filepath.Dir(filepath.Dir(path)), "artifacts")
			switch *fault {
			case "missing artifact":
				return os.Remove(filepath.Join(candidate, artifact.Names("1.2.3")[0]))
			case "unexpected artifact":
				return os.WriteFile(filepath.Join(candidate, "unexpected.bin"), []byte("unexpected"), 0o600)
			}
			return nil
		}
		if call.Name == "osv-scanner" {
			path := call.Args[slices.Index(call.Args, "--output-file")+1]
			return writeJSON(path, dependencyReportFixture(root))
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
}

func TestReleaseBindsEmbeddedSignerToActualChecksumSignature(t *testing.T) {
	root := releaseRoot(t)
	for name, content := range map[string]string{
		"go.mod": "module fixture\n", "go.sum": "fixture", "package-lock.json": "fixture", "mise.lock": "fixture",
		"mise.toml": "[tools]\ngo = \"1.27.1\"\nosv-scanner = \"2.5.1\"\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	key, other := signingKey(t), signingKey(t)
	public, err := os.ReadFile(other + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "dist")
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}
	accepted := filepath.Join(output, "accepted")
	if err := os.WriteFile(accepted, []byte("previous release"), 0o600); err != nil {
		t.Fatal(err)
	}
	var calls []toolCall
	fault := ""
	runner := releaseFixtureRunner(t, root, &calls, &fault)
	err = buildRelease(t.Context(), buildRequest{Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400", SigningKey: key}, func(call toolCall) error {
		if call.Name == "ssh-keygen" {
			// Model signing-input drift after the producer freezes its public key.
			private, err := os.ReadFile(other)
			if err != nil {
				return err
			}
			if err := os.WriteFile(key, private, 0o600); err != nil {
				return err
			}
			if err := os.WriteFile(key+".pub", public, 0o600); err != nil {
				return err
			}
		}
		return runner(call)
	})
	if err == nil || !strings.Contains(err.Error(), "embedded signer") {
		t.Fatalf("mismatched signer was admitted: %v", err)
	}
	if data, err := os.ReadFile(accepted); err != nil || string(data) != "previous release" {
		t.Fatalf("mismatched signer changed accepted output: %q, %v", data, err)
	}
}

func TestReleaseSignerAdmissionPreservesAcceptedOutput(t *testing.T) {
	for _, test := range []struct {
		name, key, contents, diagnostic string
	}{
		{"absent signer", "", "", "requires AIGW_RELEASE_SIGNING_KEY"},
		{"unreadable signer", "missing.pub", "", "read release signing public key"},
		{"invalid signer", "invalid.pub", "invalid", "valid SSH public key"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := releaseRoot(t)
			key := test.key
			if key != "" {
				key = filepath.Join(root, key)
			}
			if test.contents != "" {
				if err := os.WriteFile(key, []byte(test.contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			output := filepath.Join(root, "dist")
			if err := os.Mkdir(output, 0o700); err != nil {
				t.Fatal(err)
			}
			accepted := filepath.Join(output, "accepted")
			if err := os.WriteFile(accepted, []byte("previous release"), 0o600); err != nil {
				t.Fatal(err)
			}
			var calls []string
			err := buildRelease(t.Context(), buildRequest{Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400", SigningKey: key}, func(call toolCall) error {
				calls = append(calls, call.Name)
				return nil
			})
			var expected []string
			if key != "" {
				expected = []string{"git"}
			}
			if err == nil || !strings.Contains(err.Error(), test.diagnostic) || !slices.Equal(calls, expected) {
				t.Fatalf("signer refusal = %v, calls %v; want %s, %v", err, calls, test.diagnostic, expected)
			}
			if data, err := os.ReadFile(accepted); err != nil || string(data) != "previous release" {
				t.Fatalf("signer refusal changed accepted output: %q, %v", data, err)
			}
			workspaces, err := filepath.Glob(filepath.Join(root, ".aigw-release-*"))
			if err != nil || len(workspaces) != 0 {
				t.Fatalf("signer refusal left workspace residue: %v, %v", workspaces, err)
			}
		})
	}
}
