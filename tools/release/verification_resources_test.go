//go:build native_resource_acceptance

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"aigw-cli/tools/release/readiness"
)

func TestNativeVerificationResources(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	version, err := readiness.ReadProductVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	program, _, _ := nativeReleaseCandidate(t, root, version)
	for _, mode := range []string{"success", "failure", "parent-exit", "interrupt", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			journey := newNativeJourney(t, program, "https://unused.example.test", false)
			t.Cleanup(journey.uninstallAndRequireInstallationRemoved)
			journey.installClientFixture(configuration.ClientHermes)
			journey.setEnvironment("HERMES_HOME", filepath.Join(journey.root, "home", ".hermes"))
			temporary := filepath.Join(journey.root, "temporary")
			if err := os.Mkdir(temporary, 0o700); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
				journey.setEnvironment(key, temporary)
			}
			journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "native-resource-token")
			manifest := `version = 7

[recommendations.hermes.primary]
route = "native-resource-hermes"

[accounts.native-system-keyring-probe]
label = "Native Resource Probe"

[accounts.native-system-keyring-probe.endpoints]
openai_chat_completions = "https://unused.example.test/v1"

[models.native-resource-model]
label = "Native Resource Model"

[routes.native-resource-hermes]
label = "Native Resource Hermes"
account = "native-system-keyring-probe"
model = "native-resource-model"
upstream_model = "native-resource-model"
interfaces = { openai_chat_completions = ["text"] }
`
			if err := os.WriteFile(journey.manifest, []byte(manifest), 0o600); err != nil {
				t.Fatal(err)
			}
			journey.run("setup", "--from", journey.manifest, "--account", "native-system-keyring-probe")
			verifyNativeResourceCase(t, journey, temporary, mode)
		})
	}
}

func TestVerificationResourceCleanupStopsOwnedFixture(t *testing.T) {
	control := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable)
	command.Env = environmentWith(os.Environ(), map[string]string{
		"AIGW_TEST_RESOURCE_ROLE":    "child",
		"AIGW_TEST_RESOURCE_CONTROL": control,
	})
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	completed := make(chan error, 1)
	go func() { completed <- command.Wait() }()
	t.Cleanup(func() { _ = command.Process.Kill() })
	if err := awaitVerificationFile(filepath.Join(control, "child.json")); err != nil {
		t.Fatal(err)
	}
	t.Run("owned assertion cleanup", func(t *testing.T) {
		alive := observeVerificationProcess(t, command.Process.Pid, control, "child")
		if !alive() {
			t.Fatal("owned fixture did not reach the assertion boundary")
		}
	})
	select {
	case <-completed:
	case <-time.After(2 * time.Second):
		t.Fatal("native process observation did not reclaim its owned fixture")
	}
}

type verificationResourceFile struct {
	Mode   fs.FileMode
	Digest [sha256.Size]byte
	Mtime  time.Time
}

func verificationInventory(t *testing.T, root string) map[string]verificationResourceFile {
	t.Helper()
	files := make(map[string]verificationResourceFile)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == filepath.Join(root, "control") {
			return fs.SkipDir
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("resource fixture contains unowned link %s", path)
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		file := verificationResourceFile{Mode: info.Mode()}
		if info.Mode().IsRegular() {
			file.Digest = sha256.Sum256(readFile(t, path))
			file.Mtime = info.ModTime()
		}
		files[name] = file
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func verifyNativeResourceCase(t *testing.T, journey *journeyFixture, temporary, mode string) {
	t.Helper()
	control := filepath.Join(journey.root, "control")
	if err := os.Mkdir(control, 0o700); err != nil {
		t.Fatal(err)
	}
	environment := environmentWith(journey.environment, map[string]string{
		"AIGW_TEST_RESOURCE_CONTROL": control,
		"AIGW_TEST_RESOURCE_CASE":    mode,
	})
	unrelatedAlive := startVerificationSentinel(t, environment, control)
	beforeHome := verificationInventory(t, journey.root)
	beforeTemporary := verificationInventory(t, temporary)
	beforeProgram := sha256.Sum256(readFile(t, journey.binary))
	ctx, cancel := context.WithTimeout(t.Context(), 70*time.Second)
	command := exec.CommandContext(ctx, journey.binary, "verify", "--for", "hermes")
	command.Env, command.Dir = environment, journey.root
	prepareVerificationInterrupt(command)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	started := time.Now()
	if err := command.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	completed := make(chan struct{})
	var runErr error
	go func() { runErr = command.Wait(); close(completed) }()
	defer func() {
		finishVerificationCommand(t, command, environment, completed)
		cancel()
	}()
	if err := awaitVerificationFile(filepath.Join(control, "parent.json")); err != nil {
		finishVerificationCommand(t, command, environment, completed)
		t.Fatalf("public verification did not invoke controlled client: %v\n%s\n%s", err, &stdout, &stderr)
	}
	var parent, child verificationResourceProcess
	for name, target := range map[string]*verificationResourceProcess{"parent": &parent, "child": &child} {
		if err := json.Unmarshal(readFile(t, filepath.Join(control, name+".json")), target); err != nil {
			t.Fatal(err)
		}
	}
	parentAlive := observeVerificationProcess(t, parent.PID, control, "parent")
	childAlive := observeVerificationProcess(t, child.PID, control, "child")
	if !parentAlive() || !childAlive() || !unrelatedAlive() || parent.Home == "" || child.Home != parent.Home {
		t.Fatal("resource fixture did not establish live owned and unrelated processes")
	}
	if err := os.WriteFile(filepath.Join(control, "release"), []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	if mode == "interrupt" {
		if err := interruptVerificationCommand(command, environment); err != nil {
			t.Fatal(err)
		}
	}
	<-completed
	requireVerificationOutcome(t, mode, runErr, ctx, started, &stdout, &stderr)
	requireVerificationProcessesReclaimed(t, parentAlive, childAlive, unrelatedAlive)
	if _, err := os.Lstat(parent.Home); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("verification home remains: %s: %v", parent.Home, err)
	}
	if !reflect.DeepEqual(beforeHome, verificationInventory(t, journey.root)) ||
		!reflect.DeepEqual(beforeTemporary, verificationInventory(t, temporary)) ||
		beforeProgram != sha256.Sum256(readFile(t, journey.binary)) || stderr.Len() != 0 {
		t.Fatalf("verification changed protected inventory or emitted stderr: %s", &stderr)
	}
	t.Logf("case=%s candidate_sha256=%x owned_parent=%d owned_child=%d elapsed=%s resource_inventory=unchanged", mode, beforeProgram, parent.PID, child.PID, time.Since(started))
}

func finishVerificationCommand(t *testing.T, command *exec.Cmd, environment []string, completed <-chan struct{}) {
	t.Helper()
	select {
	case <-completed:
		return
	default:
	}
	_ = interruptVerificationCommand(command, environment)
	select {
	case <-completed:
	case <-time.After(5 * time.Second):
		_ = command.Process.Kill()
		<-completed
		t.Error("resource fixture required forced owned-command cleanup")
	}
}

func requireVerificationOutcome(t *testing.T, mode string, runErr error, ctx context.Context, started time.Time, stdout, stderr *bytes.Buffer) {
	t.Helper()
	var exit *exec.ExitError
	if mode == "success" && runErr != nil || mode != "success" && (!errors.As(runErr, &exit) || exit.ExitCode() != 1) {
		t.Fatalf("public %s exit: %v\n%s\n%s", mode, runErr, stdout, stderr)
	}
	if ctx.Err() != nil || mode == "deadline" && time.Since(started) < 59*time.Second {
		t.Fatal("outer cancellation substituted for the product's actual deadline")
	}
}

func startVerificationSentinel(t *testing.T, environment []string, control string) func() bool {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable)
	command.Env = environmentWith(environment, map[string]string{"AIGW_TEST_RESOURCE_ROLE": "unrelated"})
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	if err := awaitVerificationFile(filepath.Join(control, "unrelated.json")); err != nil {
		t.Fatal(err)
	}
	return observeVerificationProcess(t, command.Process.Pid, control, "unrelated")
}

func requireVerificationProcessesReclaimed(t *testing.T, parent, child, unrelated func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for (parent() || child()) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if parent() || child() || !unrelated() {
		t.Fatal("public verification left an owned process or stopped the unrelated process")
	}
}
