//go:build darwin

package construction

import (
	"aigw-cli/internal/upgrade/artifact"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestNativeReleaseArchivesContainDeterministicCertificateSignatures(t *testing.T) {
	request, requirement := privateCertificateRelease(t)
	var previous []byte
	for range 2 {
		if previous != nil {
			// Cross a filesystem timestamp boundary so wall-clock metadata fails.
			time.Sleep(1100 * time.Millisecond)
		}
		stage, err := buildArchives(request, t.TempDir(), executeTool)
		if err != nil {
			t.Fatal(err)
		}
		checksums, err := os.ReadFile(filepath.Join(stage, "checksums.txt"))
		if err != nil || previous != nil && !bytes.Equal(previous, checksums) {
			t.Fatalf("certificate-signed archive matrix is not reproducible: %v\nfirst:\n%s\nsecond:\n%s", err, previous, checksums)
		}
		previous = checksums
		for _, platform := range []string{"darwin", "linux", "windows"} {
			for _, arch := range []string{"amd64", "arm64"} {
				target := artifact.Target{OS: platform, Arch: arch}
				program, err := target.ReadProgram(filepath.Join(stage, target.ArchiveName(request.Version)), filepath.Join(stage, "checksums.txt"), request.Version)
				if err != nil {
					t.Fatal(err)
				}
				if platform == "darwin" {
					path := filepath.Join(t.TempDir(), "aigw")
					if err := os.WriteFile(path, program, 0o700); err != nil {
						t.Fatal(err)
					}
					privateReleaseCommand(t, "", "/usr/bin/codesign", "--verify", "--strict", "--test-requirement", "="+requirement, path)
				}
			}
		}
	}
	for _, platform := range []string{"linux", "windows"} {
		t.Run(platform+" without macOS credentials", func(t *testing.T) {
			request.TargetOS = platform
			for _, name := range []string{"AIGW_MACOS_SIGNING_P12", "AIGW_MACOS_SIGNING_PASSWORD_FILE", "AIGW_MACOS_SIGNING_REQUIREMENTS"} {
				t.Setenv(name, "")
			}
			stage, err := buildArchives(request, t.TempDir(), executeTool)
			if err != nil {
				t.Fatal(err)
			}
			checksums, err := os.ReadFile(filepath.Join(stage, "checksums.txt"))
			if err != nil || len(strings.Fields(string(checksums))) != 4 {
				t.Fatalf("native %s build must contain exactly two architecture archives: %v", platform, err)
			}
			for _, arch := range []string{"amd64", "arm64"} {
				target := artifact.Target{OS: platform, Arch: arch}
				if _, err := target.ReadProgram(filepath.Join(stage, target.ArchiveName(request.Version)), filepath.Join(stage, "checksums.txt"), request.Version); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func privateCertificateRelease(t *testing.T) (buildRequest, string) {
	t.Helper()
	root := releaseRoot(t)
	config, err := os.ReadFile(filepath.Join("..", "..", "..", ".config", "release", "goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{
		".config/release/goreleaser.yaml": config,
		"go.mod":                          []byte("module aigw-cli\n\ngo 1.25\n"),
		"cmd/aigw/main.go":                []byte("package main\n\nfunc main() {}\n"),
		"README.md":                       []byte("# Signing fixture\n"), "LICENSE": []byte("fixture\n"),
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	private := t.TempDir()
	p12, password, certificate := filepath.Join(private, "identity.p12"), filepath.Join(private, "password"), filepath.Join(private, "identity")
	if err := os.WriteFile(password, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	privateReleaseCommand(t, "", "rcodesign", "-C", "/dev/null", "generate-self-signed-certificate", "--person-name", "aigw-release-fixture", "--validity-days", "1", "--pem-filename", certificate, "--p12-file", p12, "--p12-password", "synthetic")
	fingerprint := string(privateReleaseCommand(t, "", "/usr/bin/openssl", "x509", "-in", certificate+".crt", "-noout", "-fingerprint", "-sha1"))
	_, fingerprint, found := strings.Cut(fingerprint, "=")
	if !found {
		t.Fatal("synthetic certificate fingerprint is absent")
	}
	fingerprint = strings.ReplaceAll(strings.TrimSpace(fingerprint), ":", "")
	requirement := `certificate leaf = H"` + fingerprint + `" and identifier "aigw"`
	requirements := filepath.Join(private, "requirement.bin")
	privateReleaseCommand(t, "", "/usr/bin/csreq", "-r", "="+requirement, "-b", requirements)
	t.Setenv("AIGW_MACOS_SIGNING_P12", p12)
	t.Setenv("AIGW_MACOS_SIGNING_PASSWORD_FILE", password)
	t.Setenv("AIGW_MACOS_SIGNING_REQUIREMENTS", requirements)
	privateReleaseCommand(t, root, "git", "init", "--quiet")
	privateReleaseCommand(t, root, "git", "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--allow-empty", "--quiet", "-m", "test fixture")
	request := buildRequest{Root: root, Version: "1.2.3", Epoch: strconv.FormatInt(time.Now().Unix(), 10)}
	return request, requirement
}

func privateReleaseCommand(t *testing.T, directory, executable string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("private release fixture %s: %v\n%s", executable, err, output)
	}
	return output
}

func TestReleaseBuildPreservesToolAndWorkspaceCleanupFailures(t *testing.T) {
	root := releaseRoot(t)
	output := filepath.Join(root, "dist")
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}
	accepted := filepath.Join(output, "accepted")
	if err := os.WriteFile(accepted, []byte("previous release"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := errors.New("interrupted release tool")
	var workspace string
	err := buildRelease(buildRequest{Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400", SigningKey: "fixture-key"}, func(call toolCall) error {
		if call.Name != "goreleaser" {
			return nil
		}
		workspace = filepath.Dir(goReleaserStage(t, call.Args))
		if err := unix.Chflags(workspace, unix.UF_IMMUTABLE); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := unix.Chflags(workspace, 0); err != nil {
				t.Errorf("restore fixture permissions: %v", err)
			}
		})
		return want
	})
	if !errors.Is(err, want) || !errors.Is(err, unix.EPERM) || !strings.Contains(err.Error(), workspace) {
		t.Fatalf("release discarded tool or cleanup cause and location: %v", err)
	}
	if content, err := os.ReadFile(accepted); err != nil || string(content) != "previous release" {
		t.Fatalf("failed build changed accepted release: %q, %v", content, err)
	}
}

func TestReleaseOutputReportsPublicationBeforeCleanupFailure(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "candidate"), filepath.Join(root, "dist")
	for directory, content := range map[string]string{source: "new release", target: "old release"} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "artifact"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	previous, err := os.Open(filepath.Join(target, "artifact"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(unix.Fchflags(int(previous.Fd()), 0), previous.Close()); err != nil {
			t.Errorf("restore retained fixture permissions: %v", err)
		}
	})
	if err := unix.Fchflags(int(previous.Fd()), unix.UF_IMMUTABLE); err != nil {
		t.Fatal(err)
	}
	err = replaceDirectory(source, target)
	if !errors.Is(err, unix.EPERM) || !strings.Contains(err.Error(), "release output published") {
		t.Fatalf("completed publication was misreported: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(target, "artifact")); err != nil || string(content) != "new release" {
		t.Fatalf("published artifact = %q, %v", content, err)
	}
	retained, err := filepath.Glob(filepath.Join(root, ".aigw-release-backup-*", "previous", "artifact"))
	if err != nil || len(retained) != 1 {
		t.Fatalf("expected one exact retained predecessor: %v, %v", retained, err)
	}
	if content, err := os.ReadFile(retained[0]); err != nil || string(content) != "old release" {
		t.Fatalf("retained predecessor = %q, %v", content, err)
	}
}
