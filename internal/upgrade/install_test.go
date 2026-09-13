package upgrade

import (
	"aigw-cli/internal/platform"
	"aigw-cli/internal/process"
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestProgramReplacementCompensatesFailedActivation(t *testing.T) {
	for _, restoreFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("restore_fails=%t", restoreFails), func(t *testing.T) {
			root := t.TempDir()
			current, candidate, previous := filepath.Join(root, "current"), filepath.Join(root, "candidate"), filepath.Join(root, "previous")
			for path, content := range map[string]string{current: "current", candidate: "candidate", previous: "older"} {
				if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			activationErr, restorationErr := errors.New("activation denied"), errors.New("restoration denied")
			var calls []string
			err := commitProgramReplacement(candidate, current, previous, func(from, to string) error {
				calls = append(calls, filepath.Base(from)+"->"+filepath.Base(to))
				if from == candidate {
					return activationErr
				}
				if from == previous && restoreFails {
					return restorationErr
				}
				return os.Rename(from, to)
			})
			if !errors.Is(err, activationErr) || errors.Is(err, restorationErr) != restoreFails {
				t.Fatalf("replacement did not preserve its causes: %v", err)
			}
			if !slices.Equal(calls, []string{"current->previous", "candidate->current", "previous->current"}) {
				t.Fatalf("replacement repeated or skipped an effect: %v", calls)
			}
			recoverable := current
			if restoreFails {
				recoverable = previous
			}
			if data, err := os.ReadFile(recoverable); err != nil || string(data) != "current" {
				t.Fatalf("replacement lost the working program: %q, %v", data, err)
			}
			if data, err := os.ReadFile(candidate); err != nil || string(data) != "candidate" {
				t.Fatalf("failed activation consumed its candidate: %q, %v", data, err)
			}
		})
	}
}

func TestReplacementPreservesForeignRollbackDirectory(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "aigw")
	if err := os.WriteFile(executable, []byte("current"), 0o700); err != nil {
		t.Fatal(err)
	}
	backup := RollbackPath(executable)
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(backup, "foreign")
	if err := os.WriteFile(foreign, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (Updater{Executable: executable}).replacePortableBinary(t.Context(), []byte("next")); err == nil {
		t.Fatal("replacement accepted a directory as its rollback file")
	}
	for path, want := range map[string]string{executable: "current", foreign: "preserve"} {
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Fatalf("failed replacement changed %s: %q, %v", path, got, err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("failed replacement retained staging: %v, %v", entries, err)
	}
}

func TestInstallPortableArchiveRequiresExistingInstallationDirectory(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3_darwin_arm64.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	archive := tarGzForTest(t, "aigw_1.2.3_darwin_arm64/aigw", []byte("new-binary"))
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fileSHA256ForTest(t, archivePath)
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), []byte(sum+"  "+archiveName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	u := Updater{GOOS: "darwin", GOARCH: "arm64", Executable: filepath.Join(directory, "missing", "aigw")}
	if err := u.installPortableArchive(t.Context(), archivePath, filepath.Join(directory, "checksums.txt"), "1.2.3"); err == nil {
		t.Fatal("installPortableArchive accepted a missing executable directory")
	}
}

func TestRollbackRejectsEmptyExecutable(t *testing.T) {
	u := Updater{Executable: "  "}
	if _, err := u.Rollback(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "AIGW executable path is empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestRollbackRejectsUnrunnablePredecessorWithoutReplacingPrograms(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "aigw")
	previous := RollbackPath(current)
	for path, value := range map[string]string{current: "current-program", previous: "not an executable"} {
		if err := os.WriteFile(path, []byte(value), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := (Updater{Executable: current}).Rollback(t.Context(), nil); err == nil {
		t.Fatal("rollback activated an unverified predecessor")
	}
	for path, value := range map[string]string{current: "current-program", previous: "not an executable"} {
		if data, err := os.ReadFile(path); err != nil || string(data) != value {
			t.Fatalf("rollback changed %s: %q, %v", path, data, err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("rollback retained staging: %v, %v", entries, err)
	}
}

func TestRollbackConfigurationVerdictsPreservePrograms(t *testing.T) {
	for _, test := range []struct {
		name       string
		output     string
		failure    error
		compatible bool
	}{
		{name: "compatible", output: "version = 4\n[profiles.team]\nmodel = 'test'\n", compatible: true},
		{name: "unreadable", failure: errors.New("unsupported configuration")},
		{name: "empty export"},
		{name: "invalid export", output: "not a configuration"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			current := filepath.Join(root, "aigw")
			for path, value := range map[string]string{current: "current", RollbackPath(current): "previous"} {
				if err := os.WriteFile(path, []byte(value), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			runner := &recordingRunner{inspect: func(plan process.Plan) ([]byte, error) {
				if slices.Equal(plan.Args, []string{"--version"}) {
					return []byte("aigw version 0.1.0-rc.111\n"), nil
				}
				return []byte(test.output), test.failure
			}}
			_, err := (Updater{Executable: current, Runner: runner}).Rollback(t.Context(), []byte("version = 3\n"))
			if test.compatible && err != nil || !test.compatible && !errors.Is(err, ErrRollbackConfiguration) {
				t.Fatalf("rollback admission: %v", err)
			}
			if len(runner.plans) != 2 {
				t.Fatalf("predecessor verification count = %d", len(runner.plans))
			}
			wantCurrent, wantPrevious := "current", "previous"
			if test.compatible {
				wantCurrent, wantPrevious = wantPrevious, wantCurrent
			}
			for path, want := range map[string]string{current: wantCurrent, RollbackPath(current): wantPrevious} {
				if data, err := os.ReadFile(path); err != nil || string(data) != want {
					t.Fatalf("program %s = %q, %v; want %q", path, data, err, want)
				}
			}
			if entries, err := os.ReadDir(root); err != nil || len(entries) != 2 {
				t.Fatalf("rollback retained temporary resources: %v, %v", entries, err)
			}
		})
	}
}

func TestRollbackVerificationReadsExactBytesWithoutHostCredentials(t *testing.T) {
	root := t.TempDir()
	config := []byte("version = 3\n# exact snapshot\n[recommended_routes]\nclaude = 'team'\n")
	t.Setenv("AIGW_TOKEN_TEAM", "ambient-secret")
	runner := &recordingRunner{inspect: func(plan process.Plan) ([]byte, error) {
		environment := map[string]string{}
		for _, entry := range plan.Env {
			name, value, _ := strings.Cut(entry, "=")
			environment[name] = value
		}
		if environment["AIGW_TOKEN_TEAM"] != "" || environment["AIGW_SECRET_BACKEND"] != "env" || environment["PATH"] != "" || environment["HOME"] != filepath.Join(filepath.Dir(plan.Executable), "home") {
			t.Fatal("predecessor inherited host credentials or client paths")
		}
		for _, goos := range []string{"darwin", "linux", "windows"} {
			configPath, err := platform.ConfigPathFor(goos, environment)
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(filepath.ToSlash(configPath), filepath.ToSlash(plan.Executable)+"/") {
				t.Fatalf("%s configuration directory collides with the staged program: %s", goos, configPath)
			}
		}
		if data, err := os.ReadFile(plan.Executable); err != nil || string(data) != "previous" {
			t.Fatalf("verified different predecessor bytes: %q, %v", data, err)
		}
		if slices.Equal(plan.Args, []string{"--version"}) {
			return []byte("aigw version 0.1.0-rc.111\n"), nil
		}
		if !slices.Equal(plan.Args, []string{"config", "export"}) {
			t.Fatalf("unexpected predecessor command: %v", plan.Args)
		}
		path, err := platform.ConfigPathFor(runtime.GOOS, environment)
		if err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, config) {
			t.Fatalf("predecessor read different configuration: %q, %v", data, err)
		}
		return []byte("version = 4\n[profiles.team]\nmodel = 'test'\n"), nil
	}}
	if err := (Updater{Executable: filepath.Join(root, "aigw"), Runner: runner}).verifyProgram(t.Context(), []byte("previous"), "", config); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("verification residue: %v, %v", entries, err)
	}
}

func TestRollbackRejectsMissingCurrentExecutable(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "missing")
	if err := os.WriteFile(RollbackPath(executable), []byte("previous-program"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := Updater{Executable: executable}
	if _, err := u.Rollback(t.Context(), nil); err == nil || !strings.Contains(err.Error(), "inspect current AIGW executable") {
		t.Fatalf("error = %v", err)
	}
}

func TestRollbackRejectsUnreadableBackup(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "aigw")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A directory at the backup path makes os.ReadFile fail with an error
	// other than os.ErrNotExist.
	if err := os.Mkdir(RollbackPath(executable), 0o700); err != nil {
		t.Fatal(err)
	}
	u := Updater{Executable: executable}
	if _, err := u.Rollback(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "read previous AIGW executable") {
		t.Fatalf("error = %v", err)
	}
}

// tarGzForTest builds the single executable used by installation tests.
func tarGzForTest(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	writer := tar.NewWriter(gz)
	err := writer.AddFS(fstest.MapFS{name: &fstest.MapFile{Data: data, Mode: 0o755}})
	if err := errors.Join(err, writer.Close(), gz.Close()); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func zipArchive(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	err := writer.AddFS(fstest.MapFS{name: &fstest.MapFile{Data: data, Mode: 0o755}})
	if err := errors.Join(err, writer.Close()); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestRollbackPathVariants(t *testing.T) {
	if got := RollbackPath("/opt/aigw/aigw"); got != "/opt/aigw/.aigw.previous" {
		t.Fatalf("rollbackPath = %q", got)
	}
	if got := RollbackPath("/opt/aigw/aigw.EXE"); got != "/opt/aigw/.aigw.previous.exe" {
		t.Fatalf("rollbackPath = %q", got)
	}
	if got := RollbackPath(`C:\aigw\aigw.exe`); got != `C:\aigw\.aigw.previous.exe` {
		t.Fatalf("rollbackPath = %q", got)
	}
	if got := RollbackPath(`C:\aigw\aigw`); got != `C:\aigw\.aigw.previous` {
		t.Fatalf("rollbackPath = %q", got)
	}
}

func TestReplacePortableBinaryPropagatesPreserveFailure(t *testing.T) {
	if err := (Updater{Executable: filepath.Join(t.TempDir(), "missing")}).replacePortableBinary(t.Context(), []byte("data")); err == nil {
		t.Fatal("replacePortableBinary accepted a missing executable")
	}
}

func TestInstallPortableArchiveRejectsChecksumMismatch(t *testing.T) {
	directory := t.TempDir()
	archiveName := "aigw_1.2.3_darwin_arm64.tar.gz"
	archivePath := filepath.Join(directory, archiveName)
	if err := os.WriteFile(archivePath, []byte("archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "checksums.txt"), []byte(strings.Repeat("0", 64)+"  "+archiveName+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	u := Updater{GOOS: "darwin", GOARCH: "arm64", Executable: filepath.Join(directory, "aigw")}
	if err := u.installPortableArchive(t.Context(), archivePath, filepath.Join(directory, "checksums.txt"), "1.2.3"); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v", err)
	}
}

func fileSHA256ForTest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func TestPortableInstallHelpersRejectEmptyTargetAndPreserveForeignSeparators(t *testing.T) {
	if err := (Updater{}).replacePortableBinary(t.Context(), []byte("binary")); err == nil || !strings.Contains(err.Error(), "path is empty") {
		t.Fatalf("empty target error = %v", err)
	}
	if got := RollbackPath(`C:\\tools\\aigw.exe`); got != `C:\\tools\\.aigw.previous.exe` {
		t.Fatalf("Windows rollback path = %q", got)
	}
}
