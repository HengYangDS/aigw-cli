package construction

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestReleaseToolQualifiesNativeDiagnostics(t *testing.T) {
	if diagnostic := os.Getenv("AIGW_TEST_RELEASE_DIAGNOSTIC"); diagnostic != "" {
		_, _ = fmt.Fprint(os.Stdout, "Warning output remains ordinary tool data\n")
		_, _ = fmt.Fprint(os.Stderr, diagnostic)
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name       string
		diagnostic string
		rejected   bool
	}{
		{"progress", "Preparing native artifacts\n", false},
		{"warning", "[WARN] native tool needs attention\n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := executeTool(t.Context())(toolCall{
				Name: binary, Directory: t.TempDir(),
				Args:   []string{"-test.run=^TestReleaseToolQualifiesNativeDiagnostics$"},
				Env:    []string{"AIGW_TEST_RELEASE_DIAGNOSTIC=" + test.diagnostic},
				Stdout: &stdout, Stderr: &stderr,
			})
			if (err != nil) != test.rejected {
				t.Fatalf("native diagnostic qualification: rejected=%t error=%v", test.rejected, err)
			}
			if !strings.HasPrefix(stdout.String(), "Warning output remains ordinary tool data\n") || stderr.String() != test.diagnostic {
				t.Fatalf("native tool streams changed: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestReleaseToolPreservesNativeImmediateExitDiagnostics(t *testing.T) {
	captureRoot := t.TempDir()
	workingRoot := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, captureRoot)
	}
	for _, test := range []struct {
		name, tail string
		size, exit int
		invalid    bool
	}{
		{"progress", " native progress\n", 8192, 0, false},
		{"large progress", " native progress\n", 64<<10 - 1, 0, false},
		{"warning", " warning: incomplete analysis\n", 8192, 0, true},
		{"large warning", " warning: incomplete analysis\n", 64<<10 - 1, 0, true},
		{"large failure", " native failure detail\n", 64<<10 - 1, 7, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := executeTool(t.Context())(toolCall{
				Name: "node", Directory: workingRoot,
				Args: []string{"-e", "process.stdout.write('x'.repeat(Number(process.env.AIGW_TEST_NATIVE_SIZE)) + ' Warning is output data\\n'); process.stderr.write('x'.repeat(Number(process.env.AIGW_TEST_NATIVE_SIZE)) + process.env.AIGW_TEST_NATIVE_TAIL); process.exit(Number(process.env.AIGW_TEST_NATIVE_EXIT));"},
				Env: []string{
					"AIGW_TEST_NATIVE_SIZE=" + strconv.Itoa(test.size),
					"AIGW_TEST_NATIVE_TAIL=" + test.tail,
					"AIGW_TEST_NATIVE_EXIT=" + strconv.Itoa(test.exit),
				},
				Stdout: &stdout, Stderr: &stderr,
			})
			if (err != nil) != test.invalid {
				t.Fatalf("native exit qualification: invalid=%t error=%v", test.invalid, err)
			}
			if stdout.String() != strings.Repeat("x", test.size)+" Warning is output data\n" || stderr.String() != strings.Repeat("x", test.size)+test.tail {
				t.Fatalf("native output lost bytes: stdout=%d stderr=%d prefix=%d", stdout.Len(), stderr.Len(), test.size)
			}
			if entries, readErr := os.ReadDir(captureRoot); readErr != nil || len(entries) != 0 {
				t.Fatalf("native capture residue: %v error=%v", entries, readErr)
			}
		})
	}
}

func TestReleaseToolSeparatesAcquisitionCredentials(t *testing.T) {
	credentialNames := []string{
		"GH_TOKEN", "GITHUB_TOKEN", "GITLAB_TOKEN", "CI_JOB_TOKEN", "AIGW_GITHUB_TOKEN",
		"MISE_GITHUB_TOKEN", "MISE_GITLAB_TOKEN", "MISE_NETRC_FILE",
		"MISE_GITHUB_CREDENTIAL_COMMAND", "MISE_GITLAB_CREDENTIAL_COMMAND",
	}
	if os.Getenv("AIGW_TEST_FORGE_ENV_CHILD") == "1" {
		for _, name := range credentialNames {
			if os.Getenv(name) != "" {
				_, _ = fmt.Fprint(os.Stdout, name+" ")
			}
		}
		if data, err := os.ReadFile(os.Getenv("MISE_NETRC_FILE")); err == nil && string(data) == "fixture-only" {
			_, _ = fmt.Fprint(os.Stdout, "readable-job-file ")
		}
		_, _ = fmt.Fprint(os.Stdout, os.Getenv("AIGW_TEST_RELEASE_VALUE"))
		os.Exit(0)
	}
	private := t.TempDir()
	netrc := filepath.Join(private, "job.netrc")
	if err := os.WriteFile(netrc, []byte("fixture-only"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range credentialNames {
		t.Setenv(name, "fixture-only")
	}
	t.Setenv("MISE_NETRC_FILE", netrc)
	t.Setenv("MISE_NETRC", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"acceptance", "construction", "acquisition"} {
		t.Run(mode, func(t *testing.T) {
			request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
			var observed bytes.Buffer
			stopConstruction := errors.New("stop after exact construction child")
			child := func(call toolCall) error {
				call.Name = executable
				call.Args = []string{"-test.run=^TestReleaseToolSeparatesAcquisitionCredentials$"}
				call.Env = append(call.Env, "AIGW_TEST_FORGE_ENV_CHILD=1", "AIGW_TEST_RELEASE_VALUE=owned")
				call.Stdout = &observed
				call.Timeout = 10 * time.Second
				if err := executeTool(t.Context())(call); err != nil {
					return err
				}
				if mode == "construction" {
					return stopConstruction
				}
				return nil
			}
			var err error
			switch mode {
			case "acceptance":
				err = acceptNative(request, "", "", NativeAcceptance{}, child)
			case "construction":
				_, err = buildArchives(request, t.TempDir(), child)
				if errors.Is(err, stopConstruction) {
					err = nil
				}
			case "acquisition":
				err = downloadNativeRelease(request.Root, NativeAcceptance{Peer: "gitlab", Repository: "group/product"}, "v1.2.3", filepath.Join(t.TempDir(), "download"), child)
			}
			if err != nil {
				t.Fatal(err)
			}
			if mode == "acquisition" {
				if !strings.Contains(observed.String(), "readable-job-file owned") {
					t.Fatalf("acquisition lost its declared synthetic authentication: %s", &observed)
				}
			} else if observed.String() != "owned" {
				t.Fatalf("subject inherited acquisition credentials or a readable job-file pointer: %s", &observed)
			}
			if data, err := os.ReadFile(netrc); err != nil || string(data) != "fixture-only" || os.Getenv("CI_JOB_TOKEN") != "fixture-only" {
				t.Fatal("subject mutation changed the parent acquisition authority")
			}
		})
	}
}

func TestReleaseToolDeadlineLeavesItsParentLive(t *testing.T) {
	if os.Getenv("AIGW_TEST_DEADLINE_CHILD") == "1" {
		time.Sleep(30 * time.Second)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	parent, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	err = executeTool(parent)(toolCall{Name: executable, Args: []string{"-test.run=^TestReleaseToolDeadlineLeavesItsParentLive$"}, Env: []string{"AIGW_TEST_DEADLINE_CHILD=1"}, Timeout: 100 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) || parent.Err() != nil {
		t.Fatalf("child deadline failed or cancelled its parent: %v, %v", err, parent.Err())
	}
}

func TestReleaseToolOwnsNativeFilesAndExplicitContext(t *testing.T) {
	if os.Getenv("AIGW_TEST_RELEASE_TOOL") == "child" {
		for _, output := range []*os.File{os.Stdout, os.Stderr} {
			info, err := output.Stat()
			if err != nil || !info.Mode().IsRegular() {
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
		Args:   []string{"-test.run=^TestReleaseToolOwnsNativeFilesAndExplicitContext$"},
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
