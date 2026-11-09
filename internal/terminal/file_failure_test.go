package terminal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/platform"
)

// A failure of a client file names the client, the file relative to the home
// directory and its operating-system error, says what became of each file
// that the attempt had written and whether the configuration was restored,
// and in JSON says the same. A permission error asks for a writable file and
// folder, a file that changed is read again by the same command, and any
// other error is resolved first; none names aigw check. A file outside the
// home directory is named by its full path, which the operator chose.
func TestAClientFileFailureNamesEachFile(t *testing.T) {
	// Two directories of one test lie side by side, so the second is outside
	// the first on every platform.
	home, elsewhere := t.TempDir(), t.TempDir()
	failed := filepath.Join(home, "b", "config.toml")
	restoreFailure := &platform.FileError{Path: filepath.Join(home, "c", "config.toml"), Err: os.ErrPermission}
	failure := &adapter.ApplyError{
		Client: "Codex", Home: home,
		Failed: &platform.FileError{Path: failed, Err: &os.PathError{Op: "createtemp", Path: failed, Err: os.ErrPermission}},
		Restored: platform.Restoration{
			{Path: filepath.Join(home, "a", "config.toml.aigw-state.json")},
			{Path: filepath.Join(home, "a", "config.toml")},
			{Path: filepath.Join(home, "c", "config.toml"), Err: restoreFailure},
			{Path: filepath.Join(elsewhere, "config.toml")},
		},
		Err: errors.New("internal detail"),
	}
	err := fmt.Errorf("client edit %w", presentationProjectionError{restored: true, cause: failure})
	problem := ProblemFor(err)
	want := Problem{
		Title: "Codex could not write ~/b/config.toml: permission denied",
		Evidence: "~/a/config.toml.aigw-state.json was restored. ~/a/config.toml was restored. " +
			"~/c/config.toml could not be restored: permission denied. " + filepath.Join(elsewhere, "config.toml") + " was restored. ~/b/config.toml is unchanged.",
		Impact: "The configuration was restored.",
		Fix:    "Make ~/b/config.toml and its folder readable and writable, then run the same command again.",
	}
	if problem != want {
		t.Fatalf("ProblemFor() = %#v\nwant %#v", problem, want)
	}
	var out bytes.Buffer
	RenderError(NewRenderer(&out, false), err, true)
	var document map[string]any
	if json.Unmarshal(out.Bytes(), &document) != nil || document["error"] != want.Title || document["evidence"] != want.Evidence ||
		document["impact"] != want.Impact || document["next_detail"] != want.Fix {
		t.Fatalf("failure document = %s", out.String())
	}
	// No value names the home directory itself; a file outside it is named
	// by its full path, which may share any part of the home's name.
	for key, value := range document {
		if text, ok := value.(string); ok && strings.Contains(text, home) {
			t.Fatalf("failure document names the home directory in %s: %s", key, out.String())
		}
	}

	outside := filepath.Join(elsewhere, "b.toml")
	failure.Failed = &platform.FileError{Path: outside, Changed: true, Err: errors.New("preimage changed")}
	failure.Restored = nil
	problem = ProblemFor(presentationProjectionError{restored: false, cause: failure})
	if problem.Title != "Codex could not write "+outside+": it changed after AIGW read it" ||
		problem.Evidence != outside+" keeps the change made outside AIGW." ||
		problem.Impact != "Restoring the configuration failed too, so it may still hold the change." ||
		problem.Fix != "aigw check --depth local" {
		t.Fatalf("a changed file outside home whose configuration was not restored = %#v", problem)
	}
	// Restored, the same change is read again by the same command.
	if problem = ProblemFor(presentationProjectionError{restored: true, cause: failure}); problem.Fix != "Run the same command again; it reads "+outside+" as it is now." {
		t.Fatalf("a changed file outside home = %#v", problem)
	}

	// Another operating-system error is resolved first, on the file that it
	// names, outside the home directory too.
	for _, path := range []string{failed, outside} {
		failure.Failed = &platform.FileError{Path: path, Err: &os.PathError{Op: "write", Path: path, Err: syscall.ENOSPC}}
		name := fileName(path, home)
		problem = ProblemFor(presentationProjectionError{restored: true, cause: failure})
		if want := "Resolve " + syscall.ENOSPC.Error() + " on " + name + ", then run the same command again."; problem.Fix != want {
			t.Fatalf("a full disk at %s = %#v, want the fix %q", path, problem, want)
		}
	}
	failure.Failed = &platform.FileError{Path: outside, Err: &os.PathError{Op: "createtemp", Path: outside, Err: os.ErrPermission}}
	if problem = ProblemFor(failure); problem.Fix != "Make "+outside+" and its folder readable and writable, then run the same command again." {
		t.Fatalf("a permission error outside home = %#v", problem)
	}
}

// A failure that names no file keeps the generic report, whose fix is aigw
// status, which sends no probe.
func TestAClientFilesFailureWithoutAFileNamesStatus(t *testing.T) {
	problem := ProblemFor(presentationProjectionError{restored: true, cause: errors.New("entrypoint finalization failed")})
	if problem.Title != "Updating the client files failed" || problem.Fix != "aigw status" || problem.Impact == "" {
		t.Fatalf("ProblemFor() = %#v", problem)
	}
	// A configuration that could not be restored needs the local checks, not
	// the same command again.
	if problem := ProblemFor(presentationProjectionError{restored: false, cause: errors.New("entrypoint finalization failed")}); problem.Fix != "aigw check --depth local" {
		t.Fatalf("an unrestored configuration = %#v", problem)
	}
	home := t.TempDir()
	changed := filepath.Join(home, "x")
	outcome := restoreOutcome(platform.FileOutcome{Path: changed, Err: &platform.FileError{Path: changed, Changed: true, Err: errors.New("postimage changed")}}, home)
	if outcome != "~/x was not restored, because it changed after AIGW wrote it." {
		t.Fatalf("a restore that met a change = %q", outcome)
	}
	if outcome := restoreOutcome(platform.FileOutcome{Err: errors.New("rollback failed")}, home); outcome != "Another client file could not be restored: unexpected error." {
		t.Fatalf("a restore without a file = %q", outcome)
	}
	if name := fileName("", home); name != "a client file" {
		t.Fatalf("a file without a path = %q", name)
	}
	if name := fileName("relative", ""); name != "relative" {
		t.Fatalf("a file whose place is unknown = %q", name)
	}
}

// A client file that a plan cannot use is named as the operator configured
// it, relative to the home directory inside it and by its full path outside
// it, with why, that configuration and client files are unchanged or what
// became of the configuration, and the failure's own fix, or else one that
// acts on its file error. No path of the failure's own text reaches it.
func TestAPlanningFailureNamesTheConfiguredFile(t *testing.T) {
	home, elsewhere := t.TempDir(), t.TempDir()
	inside := &adapter.TargetError{Client: "Codex", Home: home, Path: filepath.Join(home, ".codex", "config.toml"), Desired: true, Err: nextActionError{}}
	err := fmt.Errorf("sync synchronization preflight failed; configuration and client files were unchanged: %w", inside)
	want := Problem{
		Title:  "Codex cannot use ~/.codex/config.toml: a directory is in the way",
		Impact: "Configuration and client files are unchanged.",
		Fix:    "Move the directory, then run the command again.",
	}
	if problem := ProblemFor(err); problem != want {
		t.Fatalf("a planning failure inside the home directory = %#v", problem)
	}
	outside := filepath.Join(elsewhere, "config.toml")
	resolved := filepath.Join(elsewhere, "resolved", "config.toml")
	unreadable := &adapter.TargetError{Client: "Codex", Home: home, Path: outside, Err: fmt.Errorf("read Codex config: %w", &os.PathError{Op: "open", Path: resolved, Err: os.ErrPermission})}
	problem := ProblemFor(presentationProjectionError{restored: true, cause: unreadable})
	if problem.Title != "Codex cannot use "+outside+": permission denied" || problem.Impact != "The configuration was restored." ||
		problem.Fix != "Make "+outside+" and its folder readable and writable, then run the same command again." {
		t.Fatalf("an unreadable file outside the home directory = %#v", problem)
	}
	var out bytes.Buffer
	RenderError(NewRenderer(&out, false), unreadable, true)
	if bytes.Contains(out.Bytes(), []byte("resolved")) || !bytes.Contains(out.Bytes(), []byte(`"error": "Codex cannot use `)) {
		t.Fatalf("failure document = %s", out.String())
	}
}

// A failure at a path that the operator gave names that path and why, the
// reason reading the same on every platform, and its fix acts on that path.
// The inputs read the same on every platform.
func TestGivenPathFailureNamesThePathAndTheStepThatResolvesIt(t *testing.T) {
	file := filepath.FromSlash("/opt/tools/aigw")
	folder := filepath.Dir(file)
	for _, test := range []struct {
		name         string
		err          error
		title, fixed string
	}{
		{"a folder that cannot be created", &os.PathError{Op: "mkdir", Path: folder, Err: os.ErrPermission},
			"AIGW cannot use " + folder + ": permission denied", "Make " + folder + " and its folder readable and writable, then run the same command again."},
		{"other error", fmt.Errorf("read: %w", &os.PathError{Op: "open", Path: file, Err: syscall.EIO}),
			"AIGW cannot use " + file + ": " + syscall.EIO.Error(), "Resolve " + syscall.EIO.Error() + " on " + file + ", then run the same command again."},
		{"replacing the file", &os.LinkError{Op: "rename", Old: filepath.Join(folder, ".aigw-write-1"), New: file, Err: os.ErrPermission},
			"AIGW cannot use " + file + ": permission denied", "Make " + file + " and its folder readable and writable, then run the same command again."},
	} {
		t.Run(test.name, func(t *testing.T) {
			problem := ProblemFor(GivenPathFailure(test.err, platform.PathContext{}, false))
			if problem.Title != test.title || problem.Fix != test.fixed {
				t.Fatalf("problem = %+v", problem)
			}
		})
	}
	other := errors.New("not a file failure")
	if err := GivenPathFailure(other, platform.PathContext{}, false); !errors.Is(err, other) || err.Error() != other.Error() {
		t.Fatalf("another failure became %v", err)
	}
}

// A path through a regular file names that file to move, whether the
// operation's refusal classified it or an operation reported a raw ENOTDIR, on
// every platform, and then what completes the repair: the same command for a
// path that AIGW creates, and the file and its folder for one that the
// operator creates. An operation that creates a folder at the file in the way
// names that file. With no file in the way, ENOTDIR never names one.
func TestGivenPathFailureNamesTheFileInTheWay(t *testing.T) {
	home := t.TempDir()
	blocking := filepath.Join(home, "work2")
	if err := os.WriteFile(blocking, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(blocking, "config.toml")
	at := platform.PathContext{Home: home}
	for name, err := range map[string]error{
		"the refusal":   platform.RefuseBlockedFile(target),
		"a raw ENOTDIR": &os.PathError{Op: "open", Path: target, Err: syscall.ENOTDIR},
	} {
		problem := ProblemFor(GivenPathFailure(err, at, true))
		want := Problem{Title: "AIGW cannot use ~/work2/config.toml: not a directory",
			Fix: "Move the file ~/work2, then create ~/work2/config.toml and the folder that holds it, then run the same command again."}
		if problem != want {
			t.Fatalf("%s = %+v", name, problem)
		}
		if fix := ProblemFor(GivenPathFailure(err, at, false)).Fix; fix != "Move the file ~/work2, then run the same command again." {
			t.Fatalf("%s, a path that AIGW creates = %q", name, fix)
		}
	}
	// MkdirAll names the file in the way as its own path.
	problem := ProblemFor(GivenPathFailure(&os.PathError{Op: "mkdir", Path: blocking, Err: syscall.ENOTDIR}, at, false))
	if problem.Fix != "Move the file ~/work2, then run the same command again." {
		t.Fatalf("a create at the file in the way = %+v", problem)
	}
	missing := filepath.Join(t.TempDir(), "gone", "aigw")
	if problem := ProblemFor(GivenPathFailure(&os.PathError{Op: "open", Path: missing, Err: syscall.ENOTDIR}, at, false)); strings.Contains(problem.Fix, "Move the file") {
		t.Fatalf("a missing folder was named a file in the way: %+v", problem)
	}
}

// A path that the operator gave is named as ~/… inside the run's home and as
// given outside it, so a command exposes no absolute home.
func TestGivenPathFailureNamesAPathUnderHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	file := filepath.Join(home, "work", "config.toml")
	err := &os.PathError{Op: "open", Path: file, Err: syscall.EIO}
	problem := ProblemFor(GivenPathFailure(err, platform.PathContext{Home: home}, false))
	if strings.Contains(problem.Title+problem.Fix, home) || !strings.Contains(problem.Title, "~/work/config.toml") {
		t.Fatalf("a file under home = %+v", problem)
	}
	outside := filepath.Join(t.TempDir(), "work", "config.toml")
	problem = ProblemFor(GivenPathFailure(&os.PathError{Op: "open", Path: outside, Err: syscall.EIO}, platform.PathContext{Home: home}, false))
	if !strings.Contains(problem.Title, outside) {
		t.Fatalf("a file outside home = %+v", problem)
	}
}
