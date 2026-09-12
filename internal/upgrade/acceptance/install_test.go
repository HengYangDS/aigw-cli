package acceptance_test

import (
	"aigw-cli/internal/upgrade"
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
	"time"
)

func TestCanceledRollbackPreservesBothProgramFiles(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(fmt.Sprintf("deadline_expired=%t", expired), func(t *testing.T) {
			directory := t.TempDir()
			current := filepath.Join(directory, "aigw")
			previous := filepath.Join(directory, ".aigw.previous")
			for path, content := range map[string]string{current: "current-program", previous: "previous-program"} {
				if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			want := context.Canceled
			if expired {
				cancel()
				ctx, cancel = context.WithDeadline(t.Context(), time.Time{})
				want = context.DeadlineExceeded
			}
			cancel()
			message, err := (upgrade.Updater{Executable: current}).Rollback(ctx)
			if message != "" || !errors.Is(err, want) {
				t.Errorf("canceled rollback = %q, %v; want empty result and %v", message, err, want)
			}
			for path, content := range map[string]string{current: "current-program", previous: "previous-program"} {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != content {
					t.Errorf("canceled rollback changed %s: data=%q error=%v", filepath.Base(path), data, err)
				}
			}
		})
	}
}

func TestUpdateRejectsDuplicateChecksumEntries(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	name := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &releaseRunner{archive: archive, checksum: fmt.Sprintf("%x  %s\n%x  ./%s\n", sum, name, sum, name)}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "duplicate checksum") {
		t.Fatalf("error = %v", err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "old-binary" {
		t.Fatalf("binary replaced after duplicate checksum entry: %q", got)
	}
}

func TestUpdateRejectsArchiveWithMultipleBinaries(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("first"), []byte("second"))
	name := "aigw_0.2.0_darwin_arm64.tar.gz"
	sum := sha256.Sum256(archive)
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &releaseRunner{archive: archive, checksum: fmt.Sprintf("%x  %s\n", sum, name)}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "multiple expected AIGW binaries") {
		t.Fatalf("error = %v", err)
	}
	got, readErr := os.ReadFile(binary)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "old-binary" {
		t.Fatalf("binary replaced after ambiguous archive: %q", got)
	}
}

func TestUpdateDownloadsVerifiesAndAtomicallyReplacesBinary(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	name := "aigw_0.2.0_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &releaseRunner{
		archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, name),
	}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	message, err := u.Update(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(binary)
	if string(got) != "new-binary" || !strings.Contains(message, "v0.2.0") {
		t.Fatalf("binary=%q message=%q", got, message)
	}
	backup, err := os.ReadFile(filepath.Join(filepath.Dir(binary), ".aigw.previous"))
	if err != nil {
		t.Fatalf("read rollback binary: %v", err)
	}
	if string(backup) != "old-binary" {
		t.Fatalf("rollback binary=%q, want old-binary", backup)
	}
	info, err := os.Stat(filepath.Join(filepath.Dir(binary), ".aigw.previous"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Fatalf("rollback mode = %o, want 755", info.Mode().Perm())
	}
}

func TestUpdateReplacesOnlyTheSinglePreviousRollbackBinary(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	name := "aigw_0.2.0_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(filepath.Dir(binary), ".aigw.previous")
	if err := os.WriteFile(backupPath, []byte("older-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &releaseRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, name)}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	if _, err := u.Update(context.Background(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != "current-binary" {
		t.Fatalf("rollback binary=%q, want immediate prior binary", backup)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(binary), ".aigw.previous.previous")); !os.IsNotExist(err) {
		t.Fatalf("unexpected chained rollback binary: %v", err)
	}
}

func TestUpdateMakesReplacedRollbackBinaryExecutable(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	name := "aigw_0.2.0_darwin_arm64.tar.gz"
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(filepath.Dir(binary), ".aigw.previous")
	if err := os.WriteFile(backupPath, []byte("stale-rollback"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &releaseRunner{archive: archive, checksum: fmt.Sprintf("%x  ./%s\n", sum, name)}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: runner}
	if _, err := u.Update(context.Background(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Fatalf("rollback mode = %o, want 755", info.Mode().Perm())
	}
}

func TestPortableRollbackRefusesMissingPreviousBinary(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary}
	_, err := u.Rollback(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no previous portable AIGW binary") {
		t.Fatalf("error = %v", err)
	}
}

func TestRollbackExchangesCurrentAndPreviousProgramFiles(t *testing.T) {
	directory := t.TempDir()
	current := filepath.Join(directory, "aigw")
	previous := filepath.Join(directory, ".aigw.previous")
	for path, content := range map[string]string{current: "current-program", previous: "previous-program"} {
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	u := upgrade.Updater{Executable: current}
	for _, expected := range []struct{ current, previous string }{
		{current: "previous-program", previous: "current-program"},
		{current: "current-program", previous: "previous-program"},
	} {
		message, err := u.Rollback(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		for _, guidance := range []string{"restored the previous program version", "older program does not support", "current portable package", "installer", "replaces only AIGW", "retains one predecessor"} {
			if !strings.Contains(message, guidance) {
				t.Errorf("rollback guidance missing %q: %s", guidance, message)
			}
		}
		for path, content := range map[string]string{current: expected.current, previous: expected.previous} {
			data, err := os.ReadFile(path)
			if err != nil || string(data) != content {
				t.Errorf("rollback file %s: data=%q error=%v; want %q", filepath.Base(path), data, err, content)
			}
		}
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		if !slices.Equal(names, []string{".aigw.previous", "aigw"}) {
			t.Fatalf("rollback directory entries = %q", names)
		}
	}
}

func TestUpdateRefusesChecksumMismatch(t *testing.T) {
	archive := tarGz(t, "aigw_0.2.0_linux_amd64/aigw", []byte("new-binary"))
	binary := filepath.Join(t.TempDir(), "aigw")
	_ = os.WriteFile(binary, []byte("old-binary"), 0o755)
	runner := &releaseRunner{
		archive: archive, checksum: strings.Repeat("0", 64) + "  ./aigw_0.2.0_linux_amd64.tar.gz\n",
	}
	u := upgrade.Updater{GOOS: "linux", GOARCH: "amd64", Executable: binary, Runner: runner}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v", err)
	}
	got, _ := os.ReadFile(binary)
	if string(got) != "old-binary" {
		t.Fatalf("old binary replaced after checksum failure: %q", got)
	}
}
