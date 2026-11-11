package platform

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// The one fix of each path condition follows who creates the missing parts and
// the step that completes the repair: a command runs again, and a read-only
// check names the sync whose drift would otherwise be its next failure. A path
// inside home is named ~/…; outside it a command names it as configured and a
// check names none; and no fix names the absolute home or a folder that the
// operator did not give.
func TestPathFixFollowsTheConditionWhoCreatesAndTheStep(t *testing.T) {
	home, outside := t.TempDir(), t.TempDir()
	in := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	out := func(parts ...string) string { return filepath.Join(append([]string{outside}, parts...)...) }
	command, check := PathContext{Home: home}, PathContext{Home: home, Check: true}
	configured := PathContext{Home: home, Configuration: in("aigw", "aigw.toml")}
	// A client's file: the operator gave it, and creates it when operator says.
	failure := func(condition PathCondition, path, culprit string) *PathConditionError {
		return &PathConditionError{Condition: condition, Path: path, Culprit: culprit, Err: fs.ErrPermission, Given: true}
	}
	locked := func(entries ...LockedEntry) *PathConditionError {
		return &PathConditionError{Condition: PathPermission, Path: in("work", "config.toml"), Err: syscall.EPERM, Given: true, Writing: true, Locked: entries}
	}
	own := func(condition PathCondition, path, culprit string) *PathConditionError {
		return &PathConditionError{Condition: condition, Path: path, Culprit: culprit, Err: fs.ErrPermission}
	}
	for _, test := range []struct {
		name     string
		failure  *PathConditionError
		operator bool
		at       PathContext
		want     string
	}{
		{"permission, command, inside home", failure(PathPermission, in("work", "config.toml"), ""), true, command,
			"Make ~/work/config.toml and its folder readable and writable, then run the same command again."},
		{"permission, command, outside home", failure(PathPermission, out("config.toml"), ""), true, command,
			"Make " + out("config.toml") + " and its folder readable and writable, then run the same command again."},
		{"permission, check, outside home", failure(PathPermission, out("config.toml"), ""), true, check,
			"Make the file outside the home directory and its folder readable and writable, then run `aigw sync`."},
		{"a write that a flag on the file refuses", locked(LockedEntry{Path: in("work", "config.toml")}), true, command,
			"Clear the lock flag, such as macOS's uchg or Linux's immutable attribute, on ~/work/config.toml, then run the same command again."},
		{"a write that a flag on the folder refuses", locked(LockedEntry{Path: in("work")}), true, command,
			"Clear the lock flag, such as macOS's uchg or Linux's immutable attribute, on ~/work, then run the same command again."},
		{"a write that flags on the file and the folder refuse", locked(LockedEntry{Path: in("work", "config.toml")}, LockedEntry{Path: in("work")}), true, command,
			"Clear the lock flag, such as macOS's uchg or Linux's immutable attribute, on ~/work/config.toml and ~/work, then run the same command again."},
		{"a link to a locked file inside home", locked(LockedEntry{Path: in("work", "config.toml"), Through: in("dotfiles", "config.toml")}), true, command,
			"Clear the lock flag, such as macOS's uchg or Linux's immutable attribute, on ~/dotfiles/config.toml (which ~/work/config.toml leads to), then run the same command again."},
		{"a link to a locked file outside home", locked(LockedEntry{Path: in("work", "config.toml"), Through: out("dotfiles", "config.toml")}), true, command,
			"Clear the lock flag, such as macOS's uchg or Linux's immutable attribute, on the file that ~/work/config.toml leads to, then run the same command again."},
		{"a flag and a denial of the read together", &PathConditionError{Condition: PathPermission, Path: in("work", "config.toml"), Err: syscall.EPERM, Given: true, Writing: true, Denied: true, Locked: []LockedEntry{{Path: in("work", "config.toml")}}}, true, command,
			"Clear the lock flag, such as macOS's uchg or Linux's immutable attribute, on ~/work/config.toml, and allow the sandbox or system protection that also denies access to ~/work/config.toml, then run the same command again."},
		{"a denial alone", &PathConditionError{Condition: PathPermission, Path: in("work", "config.toml"), Err: syscall.EPERM, Given: true, Writing: true, Denied: true}, true, command,
			"A sandbox or system protection denies access to ~/work/config.toml; allow it, then run the same command again."},
		{"an EPERM that no flag causes", &PathConditionError{Condition: PathPermission, Path: in("work", "config.toml"), Err: syscall.EPERM, Given: true, Writing: true}, true, command,
			"A lock, a sandbox or system protection forbids this operation on ~/work/config.toml; allow it, then run the same command again."},
		{"a read of the configuration that EPERM refuses keeps its read fix", &PathConditionError{Condition: PathPermission, Path: in("aigw", "aigw.toml"), Err: syscall.EPERM}, false,
			PathContext{Home: home, Configuration: in("aigw", "aigw.toml"), Check: true}, ConfigurationReadFix},
		{"a read of a client file that EPERM refuses names no flag", &PathConditionError{Condition: PathPermission, Path: in("work", "config.toml"), Err: syscall.EPERM, Given: true}, true, check,
			"A lock, a sandbox or system protection forbids this operation on ~/work/config.toml; allow it, then run `aigw sync`."},
		{"permission, the configuration file", failure(PathPermission, in("aigw", "aigw.toml"), ""), false, configured, ConfigurationReadFix},
		{"a file in the way, AIGW creates", failure(PathFileInTheWay, in("blocked", "aigw"), in("blocked")), false, command,
			"Move the file ~/blocked, then run the same command again."},
		{"a file in the way, the operator creates, check", failure(PathFileInTheWay, in("work", "config.toml"), in("work")), true, check,
			"Move the file ~/work, then create ~/work/config.toml and the folder that holds it, then run `aigw sync`."},
		{"a file in the way, the operator creates, command, outside home", failure(PathFileInTheWay, out("work", "config.toml"), out("work")), true, command,
			"Move the file " + out("work") + ", then create " + out("work", "config.toml") + " and the folder that holds it, then run the same command again."},
		{"a file in the way, the operator creates, check, outside home", failure(PathFileInTheWay, out("work", "config.toml"), out("work")), true, check,
			"Move the file in the way, outside the home directory, then create the file outside the home directory and the folder that holds it, then run `aigw sync`."},
		{"a file at AIGW's configuration folder", failure(PathFileInTheWay, in("aigw", "aigw.toml"), in("aigw")), false, configured,
			"Move the file ~/aigw. If it replaced AIGW's folder, put that folder back from a copy; otherwise run `aigw setup`. Then run the same command again."},
		{"a file at AIGW's configuration folder, for a rollback", failure(PathFileInTheWay, in("aigw", "aigw.toml"), in("aigw")), false,
			PathContext{Home: home, Configuration: in("aigw", "aigw.toml"), Rollback: true},
			"Move the file ~/aigw. If it replaced AIGW's folder, put that folder back from a copy, then run the same command again; otherwise run `aigw setup`, which leaves nothing to roll back."},
		{"a file above AIGW's configuration folder", failure(PathFileInTheWay, in("aigw", "aigw.toml"), home+string(filepath.Separator)+"aigw"), false,
			PathContext{Home: home, Configuration: in("aigw", "deeper", "aigw.toml")},
			"Move the file ~/aigw. If it replaced AIGW's folder, put that folder back from a copy; otherwise run `aigw setup`. Then run the same command again."},
		{"a file inside AIGW's folder is no folder of its own", failure(PathFileInTheWay, in("aigw", "secrets", "tokens"), in("aigw", "secrets")), false, configured,
			"Move the file ~/aigw/secrets, then run the same command again."},
		{"a link that leads nowhere, the operator creates", failure(PathDanglingLink, in("link", "config.toml"), in("link")), true, command,
			"Remove the link ~/link, then create ~/link/config.toml and the folder that holds it, then run the same command again."},
		{"a link that leads nowhere, AIGW creates, check", failure(PathDanglingLink, in("link", "config.toml"), in("link")), false, check,
			"Remove the link ~/link, then run `aigw sync`."},
		{"a link that leads nowhere, check, outside home", failure(PathDanglingLink, out("link", "config.toml"), out("link")), false, check,
			"Remove the link in the way, outside the home directory, then run `aigw sync`."},
		{"a missing file, the operator creates, check", failure(PathMissingFile, in("work", "config.toml"), ""), true, check,
			"Create ~/work/config.toml, then run `aigw sync`."},
		{"a missing folder, the operator creates, command", failure(PathMissingFolder, in("work", "config.toml"), ""), true, command,
			"Create ~/work/config.toml and the folder that holds it, then run the same command again."},
		{"a missing folder, the operator creates, check, outside home", failure(PathMissingFolder, out("work", "config.toml"), ""), true, check,
			"Create the file outside the home directory and the folder that holds it, then run `aigw sync`."},
		{"a missing file that AIGW creates, check", failure(PathMissingFile, in("work", "config.toml"), ""), false, check, "aigw sync"},
		{"a missing file that AIGW creates, command", failure(PathMissingFile, in("work", "config.toml"), ""), false, command, "Run the same command again."},
		{"another error", &PathConditionError{Path: in("work", "config.toml"), Err: syscall.EIO}, false, command,
			"Resolve " + syscall.EIO.Error() + " on ~/work/config.toml, then run the same command again."},
		{"a run that does not know its home, a file in the way", own(PathFileInTheWay, in("blocked", "aigw"), in("blocked")), false, PathContext{},
			"Move the file in the way in AIGW's folder, which `aigw config path` prints, then run the same command again."},
		{"a run that does not know its home, permission", own(PathPermission, in("work", "config.toml"), ""), false, PathContext{},
			"Make AIGW's folder and its files readable and writable; `aigw config path` prints where the folder is. Then run the same command again."},
		{"a path of AIGW's own outside home, another error", &PathConditionError{Path: out("aigw", "x"), Err: syscall.EIO}, false, PathContext{Home: home},
			"Resolve " + syscall.EIO.Error() + " on a file in AIGW's folder; `aigw config path` prints where the folder is. Then run the same command again."},
		{"a path of AIGW's own outside home is not named", own(PathFileInTheWay, out("aigw", "aigw.toml"), out("aigw")), false, PathContext{Home: home, Configuration: out("aigw", "aigw.toml")},
			"Move the file in the way in AIGW's folder, which `aigw config path` prints. If it replaced AIGW's folder, put that folder back from a copy; otherwise run `aigw setup`. Then run the same command again."},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.failure.OperatorCreates = test.operator
			got := PathFix(test.failure, test.at)
			if got != test.want {
				t.Fatalf("PathFix = %q\nwant      %q", got, test.want)
			}
			if strings.Contains(got, home) {
				t.Fatalf("the fix names the absolute home: %q", got)
			}
		})
	}
}

// A path failure's reason reads its condition on every platform, and any other
// failure's operating-system number, which names no path.
func TestAPathFailuresReasonReadsTheConditionOnEveryPlatform(t *testing.T) {
	blocked := &PathConditionError{Condition: PathFileInTheWay, Err: fs.ErrNotExist}
	if got := blocked.Reason(); got != "not a directory" {
		t.Fatalf("a file in the way = %q", got)
	}
	if got := (&PathConditionError{Err: syscall.EIO}).Reason(); got != syscall.EIO.Error() {
		t.Fatalf("another error = %q", got)
	}
}

// AIGW's configuration file that is a link that leads nowhere, which a
// dotfiles setup keeps on purpose, names its target to make available again,
// inside the home directory only, or else a new configuration in its place.
func TestPathFixNamesTheConfigurationLinksTarget(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, "aigw", "aigw.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0o700); err != nil {
		t.Fatal(err)
	}
	inside, outside := filepath.Join(home, "dotfiles", "aigw.toml"), filepath.Join(t.TempDir(), "mounted", "aigw.toml")
	for destination, want := range map[string]string{
		inside:  "Make ~/dotfiles/aigw.toml, which the link ~/aigw/aigw.toml leads to, available again; otherwise remove the link and run `aigw setup`. Then run the same command again.",
		outside: "Make the target of the link ~/aigw/aigw.toml available again; otherwise remove the link and run `aigw setup`. Then run the same command again.",
	} {
		_ = os.Remove(config)
		if err := os.Symlink(destination, config); err != nil {
			t.Skipf("symbolic links are unavailable: %v", err)
		}
		failure := ClassifyPath(config, FileExpected, &fs.PathError{Op: "open", Path: config, Err: fs.ErrNotExist})
		at := PathContext{Home: home, Configuration: config}
		if got := PathFix(failure, at); got != want {
			t.Fatalf("a link to %s = %q\nwant %q", destination, got, want)
		}
		if strings.Contains(PathFix(failure, at), outside) {
			t.Fatalf("the fix names a target outside home")
		}
	}
}

// A denial is observed, never inferred from the error's number: only a read
// that the operating system refuses with EPERM, which no lock flag and no file
// mode causes, is a denial. A readable or missing file is none.
func TestAReadDenialIsObservedNotInferred(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if readDenied(file) {
		t.Fatal("a readable file is denied")
	}
	if readDenied(filepath.Join(filepath.Dir(file), "missing.toml")) {
		t.Fatal("a missing file is denied")
	}
}
