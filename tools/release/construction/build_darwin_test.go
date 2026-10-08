//go:build darwin

package construction

import (
	"aigw-cli/internal/process"
	"aigw-cli/internal/upgrade/artifact"
	"bytes"
	"context"
	"crypto/sha256"
	"debug/macho"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestNativeReleaseUsesCacheSignificantDeploymentTarget(t *testing.T) {
	request := privateInternalRelease(t)
	request.TargetOS = runtime.GOOS
	t.Setenv("GOOS", runtime.GOOS)
	t.Setenv("GOARCH", runtime.GOARCH)
	t.Setenv("TARGET", "")
	t.Setenv("CGO_ENABLED", "1")
	t.Setenv("CC", "clang")
	t.Setenv("GOCACHE", t.TempDir())
	t.Setenv("MACOSX_DEPLOYMENT_TARGET", "14.0")
	environment, err := goReleaserEnvironment(request)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	var diagnostic bytes.Buffer
	run := func(call toolCall) error {
		stdout := call.Stdout
		if stdout == nil {
			stdout = &bytes.Buffer{}
		}
		started := time.Now()
		err := (process.Runner{}).RunStream(ctx, process.Plan{
			Executable: call.Name, Directory: call.Directory,
			Args: call.Args, Env: append(os.Environ(), call.Env...),
		}, stdout, &diagnostic)
		t.Logf("native cache fixture %s: %s", call.Name, time.Since(started))
		return err
	}
	// GOOS/GOARCH separate Go cache entries. Rebuild the exact warmed native
	// target here; the deterministic archive journey retains the full matrix.
	warmupPath := filepath.Join(t.TempDir(), "newer-host")
	if err := run(toolCall{Name: "go", Directory: request.Root, Args: []string{
		"build", "-trimpath", "-buildvcs=false", "-o", warmupPath, "./cmd/aigw",
	}}); err != nil {
		t.Fatalf("warm native release compiler cache: %v\n%s", err, diagnostic.Bytes())
	}
	warmupHeader, err := macho.Open(warmupPath)
	if err != nil {
		t.Fatal(err)
	}
	const buildVersionCommand, macOSPlatform, warmupMinOS = 0x32, 1, 14 << 16
	newerTarget := false
	for _, load := range warmupHeader.Loads {
		raw := load.Raw()
		if len(raw) >= 24 && warmupHeader.ByteOrder.Uint32(raw[:4]) == buildVersionCommand {
			newerTarget = warmupHeader.ByteOrder.Uint32(raw[8:12]) == macOSPlatform &&
				warmupHeader.ByteOrder.Uint32(raw[12:16]) == warmupMinOS
		}
	}
	if err := warmupHeader.Close(); err != nil {
		t.Fatal(err)
	}
	if !newerTarget {
		t.Fatal("native cache warmup did not establish the newer deployment target")
	}
	programPath := filepath.Join(t.TempDir(), "native-release")
	err = run(toolCall{
		Name: "goreleaser", Directory: request.Root,
		Args: []string{"build", "--snapshot", "--clean", "--id", "macos", "--single-target", "--output", programPath,
			"--config", filepath.Join(request.Root, ".config", "release", "goreleaser.yaml")},
		Env: environment,
	})
	if err != nil || bytes.Contains(diagnostic.Bytes(), []byte("warning:")) {
		t.Fatalf("native release must compile and link for its declared floor after a newer-target cache warmup: %v\n%s", err, diagnostic.Bytes())
	}
	program, err := os.ReadFile(programPath)
	if err != nil {
		t.Fatal(err)
	}
	header, err := macho.NewFile(bytes.NewReader(program))
	if err != nil {
		t.Fatal(err)
	}
	wantCPU := map[string]macho.Cpu{"amd64": macho.CpuAmd64, "arm64": macho.CpuArm64}[runtime.GOARCH]
	if header.Cpu != wantCPU {
		t.Fatalf("native cache target CPU = %s, expected %s", header.Cpu, wantCPU)
	}
	requireNativeReleaseSignature(t, program)
}

func TestNativeReleaseArchivesHaveDeterministicLocalSignatures(t *testing.T) {
	request := privateInternalRelease(t)
	var previous []byte
	for range 2 {
		if previous != nil {
			// Cross a filesystem timestamp boundary so wall-clock metadata fails.
			time.Sleep(1100 * time.Millisecond)
		}
		stage, err := buildArchives(request, t.TempDir(), executeTool(t.Context()))
		if err != nil {
			t.Fatal(err)
		}
		checksums, err := os.ReadFile(filepath.Join(stage, "checksums.txt"))
		if err != nil || previous != nil && !bytes.Equal(previous, checksums) {
			t.Fatalf("locally signed archive matrix is not reproducible: %v\nfirst:\n%s\nsecond:\n%s", err, previous, checksums)
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
					requireNativeReleaseSignature(t, program)
				}
			}
		}
	}
	for _, platform := range []string{"darwin", "linux", "windows"} {
		t.Run(platform+" without macOS credentials", func(t *testing.T) {
			request.TargetOS = platform
			for _, name := range []string{"AIGW_MACOS_SIGNING_P12", "AIGW_MACOS_SIGNING_PASSWORD_FILE", "AIGW_MACOS_SIGNING_REQUIREMENTS"} {
				t.Setenv(name, "")
			}
			stage, err := buildArchives(request, t.TempDir(), executeTool(t.Context()))
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

func requireNativeReleaseSignature(t *testing.T, program []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(path, program, 0o700); err != nil {
		t.Fatal(err)
	}
	privateReleaseCommand(t, "", "/usr/bin/codesign", "--verify", "--strict", path)
	signature := privateReleaseCommand(t, "", "/usr/bin/codesign", "--display", "--verbose=4", path)
	if !strings.Contains(string(signature), "(adhoc,runtime)") || !strings.Contains(string(signature), "Signature=adhoc") {
		t.Fatalf("macOS internal archive must have an ad-hoc signature and Hardened Runtime: %s", signature)
	}
	header := privateReleaseCommand(t, "", "/usr/bin/otool", "-l", path)
	if !bytes.Contains(header, []byte("\n    minos 13.0\n")) {
		t.Fatal("native macOS archive must retain the supported 13.0 deployment floor")
	}
}

func privateInternalRelease(t *testing.T) buildRequest {
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
		"cmd/aigw/native_darwin.go":       []byte("package main\n\n/*\n#include <unistd.h>\n*/\nimport \"C\"\n\nfunc init() { _ = C.getpid() }\n"),
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
	for _, name := range []string{"AIGW_MACOS_SIGNING_P12", "AIGW_MACOS_SIGNING_PASSWORD_FILE", "AIGW_MACOS_SIGNING_REQUIREMENTS"} {
		t.Setenv(name, "")
	}
	privateReleaseCommand(t, root, "git", "init", "--quiet")
	privateReleaseCommand(t, root, "git", "remote", "add", "origin", "https://github.com/HengYangDS/aigw-cli.git")
	privateReleaseCommand(t, root, "git", "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--allow-empty", "--quiet", "-m", "test fixture")
	request := buildRequest{Root: root, Version: "1.2.3", Epoch: strconv.FormatInt(time.Now().Unix(), 10)}
	return request
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
	err := buildRelease(t.Context(), buildRequest{Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400", SigningKey: "fixture-key"}, func(call toolCall) error {
		if call.Name == "osv-scanner" {
			return writeJSON(call.Args[len(call.Args)-1], dependencyReportFixture(root))
		}
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

func TestHomebrewProjectionUsesExactArchiveBytes(t *testing.T) {
	request := privateInternalRelease(t)
	request.TargetOS = "darwin"
	stage, err := buildArchives(request, t.TempDir(), executeTool(t.Context()))
	if err != nil {
		t.Fatal(err)
	}
	cask, err := os.ReadFile(filepath.Join(stage, "homebrew", "Casks", "aigw.rb"))
	if err != nil {
		t.Fatal(err)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		name := (artifact.Target{OS: "darwin", Arch: arch}).ArchiveName(request.Version)
		archive, err := os.ReadFile(filepath.Join(stage, name))
		if err != nil {
			t.Fatal(err)
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(archive))
		if !strings.Contains(string(cask), digest) {
			t.Fatalf("archive checksum missing for %s", name)
		}
		source := strings.TrimSuffix(name, ".tar.gz") + "/aigw"
		if !strings.Contains(string(cask), fmt.Sprintf("rename %q, %q", source, "aigw")) {
			t.Fatalf("wrapped program location missing for %s", name)
		}
	}
	for _, expected := range []string{`cask "aigw"`, `version "` + request.Version + `"`, `binary "aigw"`, "/releases/download/v#{version}/", "aigw client disable"} {
		if !strings.Contains(string(cask), expected) {
			t.Fatalf("Homebrew projection missing %q", expected)
		}
	}
}
