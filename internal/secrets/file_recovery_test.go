package secrets

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type interruptingRenameRoot struct {
	*os.Root
	ready string
}

func (root *interruptingRenameRoot) Rename(string, string) error {
	if err := os.WriteFile(root.ready, nil, 0o600); err != nil {
		return err
	}
	for {
		time.Sleep(time.Hour)
	}
}

func TestWriteSecureFileRecoversOwnedStagingAfterAbruptExit(t *testing.T) {
	if os.Getenv("AIGW_TEST_INTERRUPT_SECURE_FILE") == "1" {
		interruptSecureFileReplacement(t)
	}

	base := t.TempDir()
	directory := filepath.Join(base, "secrets")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "alpha")
	if err := os.WriteFile(path, []byte("old-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(base, "ready")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWriteSecureFileRecoversOwnedStagingAfterAbruptExit$")
	command.Env = append(
		os.Environ(),
		"AIGW_TEST_INTERRUPT_SECURE_FILE=1",
		"AIGW_TEST_SECURE_ROOT="+directory,
		"AIGW_TEST_READY_FILE="+ready,
	)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if ctx.Err() != nil {
			t.Fatalf("interrupted replacement did not stage its Token: %v", ctx.Err())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err == nil {
		t.Fatal("interrupted replacement exited successfully")
	}
	staging, err := filepath.Glob(filepath.Join(directory, credentialStagingPrefix+"*"))
	if err != nil || len(staging) != 1 {
		t.Fatalf("staged Tokens after abrupt exit = %#v, %v; want one", staging, err)
	}
	if value, err := os.ReadFile(path); err != nil || string(value) != "old-token" {
		t.Fatalf("committed Token after abrupt exit = %q, %v; want old-token", value, err)
	}
	if err := os.WriteFile(filepath.Join(directory, credentialStagingPrefix+"unrelated"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	if err := writeSecureFile(root, "alpha", []byte("new-token")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staging[0]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned staging remains after the next mutation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, credentialStagingPrefix+"unrelated")); err != nil {
		t.Fatalf("unowned file was not preserved: %v", err)
	}
	if value, err := os.ReadFile(path); err != nil || string(value) != "new-token" {
		t.Fatalf("committed Token after recovery = %q, %v; want new-token", value, err)
	}
}

func TestCredentialMutationsRefuseUnownedStagingObjects(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, credentialStagingPrefix+"AAAAAAAAAAAAAAAAAAAAAAAAAA")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	for _, mutation := range []struct {
		name string
		run  func() error
	}{
		{name: "replace", run: func() error { return writeSecureFile(root, "alpha", []byte("token")) }},
		{name: "delete", run: func() error { return deleteSecureFile(root, "alpha") }},
		{name: "guarded delete", run: func() error { return deleteSecureFileIf(root, "alpha", credentialFileSnapshot{}) }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			err := mutation.run()
			if err == nil || !strings.Contains(err.Error(), "not an owned regular file") {
				t.Fatalf("credential mutation error = %v; want unowned staging refusal", err)
			}
		})
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("unowned staging-shaped directory was not preserved: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(directory, "alpha")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("Token changed despite unowned staging object: %v", statErr)
	}
}

func TestCredentialStagingRecoveryReportsOwnedFilesystemFailures(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	name := credentialStagingPrefix + "AAAAAAAAAAAAAAAAAAAAAAAAAA"
	if err := os.WriteFile(filepath.Join(directory, name), []byte("staged-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	want := errors.New("injected filesystem failure")

	assertFailure := func(t *testing.T, failingRoot writeRoot, message string) {
		t.Helper()
		err := recoverCredentialStaging(failingRoot)
		if !errors.Is(err, want) || !strings.Contains(err.Error(), message) {
			t.Fatalf("recoverCredentialStaging() error = %v; want %q wrapping %v", err, message, want)
		}
		if _, statErr := os.Stat(filepath.Join(directory, name)); statErr != nil {
			t.Fatalf("staging file changed after failed recovery: %v", statErr)
		}
	}
	assertFailure(t, &faultRoot{Root: root, openErrors: map[string]error{".": want}}, "open Token directory")

	ordinaryPath := filepath.Join(t.TempDir(), "ordinary-file")
	if err := os.WriteFile(ordinaryPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ordinary, err := os.Open(ordinaryPath)
	if err != nil {
		t.Fatal(err)
	}
	err = recoverCredentialStaging(&faultRoot{Root: root, openedFile: ordinary})
	if err == nil || !strings.Contains(err.Error(), "inspect Token staging files") {
		t.Fatalf("recoverCredentialStaging() read error = %v", err)
	}
	assertFailure(t, &faultRoot{Root: root, lstatErr: want}, "inspect Token staging file")
	assertFailure(t, &faultRoot{Root: root, removeErr: want}, "remove Token staging file")

	if err := recoverCredentialStaging(&faultRoot{Root: root, lstatErr: os.ErrNotExist}); err != nil {
		t.Fatalf("recoverCredentialStaging() raced with staging removal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
		t.Fatalf("staging file changed after a raced observation: %v", err)
	}

	if err := recoverCredentialStaging(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, name)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staging file remains after recovery: %v", err)
	}
}

func TestCredentialStagingRecoveryReportsDurabilityFailure(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	name := credentialStagingPrefix + "AAAAAAAAAAAAAAAAAAAAAAAAAA"
	if err := os.WriteFile(filepath.Join(directory, name), []byte("staged-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()

	err = recoverCredentialStaging(&failingDirectorySyncRoot{Root: root, remainingFailures: 1})
	if err == nil || !strings.Contains(err.Error(), "directory sync failed") {
		t.Fatalf("recoverCredentialStaging() sync error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, name)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staging file remains after removal with unproven durability: %v", err)
	}
}

func interruptSecureFileReplacement(t *testing.T) {
	t.Helper()
	root, err := os.OpenRoot(os.Getenv("AIGW_TEST_SECURE_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	_, _, err = replaceSecureFile(
		&interruptingRenameRoot{Root: root, ready: os.Getenv("AIGW_TEST_READY_FILE")},
		"alpha",
		[]byte("interrupted-token"),
	)
	t.Fatalf("interrupted replacement returned: %v", err)
}
