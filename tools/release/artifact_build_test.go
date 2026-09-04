package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

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
	for _, name := range artifactNames(version) {
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
	if err := rewriteChecksums(directory, version); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactMatrix(directory, version); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "aigw_1.2.3_linux_amd64.deb"), []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("native package accepted: %v", err)
	}
	if err := os.Remove(filepath.Join(directory, "aigw_1.2.3_linux_amd64.deb")); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(directory, "aigw_1.2.3_linux_amd64.tar.gz")
	if err := os.WriteFile(archive, bytes.Repeat([]byte("x"), 3), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactMatrix(directory, version); err == nil || !strings.Contains(err.Error(), "checksum") {
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
			for _, name := range artifactNames(valid.Version) {
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

func populatePortableStage(t *testing.T, call toolCall, version string) error {
	t.Helper()
	stage := goReleaserStage(t, call.Args)
	if err := os.MkdirAll(stage, 0o700); err != nil {
		return err
	}
	for _, name := range artifactNames(version) {
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

func TestReleaseBuildPropagatesChecksumAndMatrixFailures(t *testing.T) {
	valid := buildRequest{Root: releaseRoot(t), Output: filepath.Join(t.TempDir(), "dist"), Version: "1.2.3", Epoch: "1784246400"}

	t.Run("checksum input disappears", func(t *testing.T) {
		err := buildRelease(valid, func(call toolCall) error {
			if call.Name == "goreleaser" {
				return populatePortableStage(t, call, valid.Version)
			}
			raw := strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json=")
			candidate := filepath.Join(filepath.Dir(filepath.Dir(raw)), "artifacts")
			if err := os.Remove(filepath.Join(candidate, artifactNames(valid.Version)[0])); err != nil {
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
				return populatePortableStage(t, call, valid.Version)
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
	}
	populate := func(call toolCall) error {
		if call.Name == "goreleaser" {
			stage := goReleaserStage(t, call.Args)
			if err := os.MkdirAll(stage, 0o700); err != nil {
				return err
			}
			for _, name := range artifactNames(valid.Version) {
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
	if err := rewriteChecksums(right, "1.2.3"); err != nil {
		t.Fatal(err)
	}
	if err := compareArtifactMatrices(left, right, "1.2.3"); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("different artifact matrices error = %v", err)
	}
	if err := rewriteChecksums(t.TempDir(), "1.2.3"); err == nil {
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
	if err := validateArtifactMatrix(directory, version); err == nil {
		t.Fatal("unreadable checksum manifest accepted")
	}

	left := releaseFixture(t, version)
	right := releaseFixture(t, version)
	name := artifactNames(version)[0]
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
	if _, err := parseBuildArguments([]string{"1.2.3", "1784246400", "dist"}); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("legacy explicit release inputs were accepted: %v", err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("## [1.2.3] - 2026-08-09\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	for name, value := range map[string]string{
		"AIGW_GITLAB_RELEASE_ORIGIN":     "https://gitlab.example",
		"AIGW_GITLAB_RELEASE_REPOSITORY": "group/aigw-cli",
		"AIGW_GITHUB_RELEASE_ORIGIN":     "https://github.example",
		"AIGW_GITHUB_RELEASE_REPOSITORY": "org/aigw-cli",
	} {
		t.Setenv(name, value)
	}
	request, err := parseBuildArguments([]string{"dist"})
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
	if _, err := parseBuildArguments([]string{"dist"}); err == nil || !strings.Contains(err.Error(), "read VERSION") {
		t.Fatalf("missing VERSION error = %v", err)
	}
	missingChronology := t.TempDir()
	if err := os.WriteFile(filepath.Join(missingChronology, "VERSION"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(missingChronology); err != nil {
		t.Fatal(err)
	}
	if _, err := parseBuildArguments([]string{"dist"}); err == nil || !strings.Contains(err.Error(), "open CHANGELOG") {
		t.Fatalf("missing release chronology error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	spdxRoot := t.TempDir()
	instant := time.Unix(1784246400, 0).UTC()
	missingCreation := filepath.Join(spdxRoot, "missing-creation.json")
	if err := os.WriteFile(missingCreation, []byte(`{"spdxVersion":"SPDX-2.3"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(spdxRoot, "normalized.json")
	if err := normalizeSPDX(missingCreation, target, "1.2.3", instant); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || !bytes.Contains(data, []byte(`"creationInfo"`)) {
		t.Fatalf("normalized SPDX = %s error=%v", data, err)
	}
	malformed := filepath.Join(spdxRoot, "malformed.json")
	if err := os.WriteFile(malformed, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := normalizeSPDX(malformed, target, "1.2.3", instant); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("malformed SPDX error = %v", err)
	}
	if err := normalizeSPDX(filepath.Join(spdxRoot, "absent.json"), target, "1.2.3", instant); err == nil || !strings.Contains(err.Error(), "read Syft") {
		t.Fatalf("missing SPDX error = %v", err)
	}

	if runtime.GOOS != "windows" {
		stage := filepath.Join(spdxRoot, "stage")
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
