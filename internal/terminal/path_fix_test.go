package terminal

import (
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"aigw-cli/internal/platform"
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
	command, check := PathContext{Home: home, Given: true}, PathContext{Home: home, Check: true, Given: true}
	configured := PathContext{Home: home, Configuration: in("aigw", "config.toml")}
	failure := func(condition platform.PathCondition, path, culprit string) *platform.PathConditionError {
		return &platform.PathConditionError{Condition: condition, Path: path, Culprit: culprit, Err: fs.ErrPermission}
	}
	for _, test := range []struct {
		name     string
		failure  *platform.PathConditionError
		operator bool
		at       PathContext
		want     string
	}{
		{"permission, command, inside home", failure(platform.PathPermission, in("work", "config.toml"), ""), true, command,
			"Make ~/work/config.toml and its folder readable and writable, then run the same command again."},
		{"permission, command, outside home", failure(platform.PathPermission, out("config.toml"), ""), true, command,
			"Make " + out("config.toml") + " and its folder readable and writable, then run the same command again."},
		{"permission, check, outside home", failure(platform.PathPermission, out("config.toml"), ""), true, check,
			"Make the file outside the home directory and its folder readable and writable, then run `aigw sync`."},
		{"permission, the configuration file", failure(platform.PathPermission, in("aigw", "config.toml"), ""), false, configured, configurationReadFix},
		{"a file in the way, AIGW creates", failure(platform.PathFileInTheWay, in("blocked", "aigw"), in("blocked")), false, command,
			"Move the file ~/blocked, then run the same command again."},
		{"a file in the way, the operator creates, check", failure(platform.PathFileInTheWay, in("work", "config.toml"), in("work")), true, check,
			"Move the file ~/work, then create ~/work/config.toml and the folder that holds it, then run `aigw sync`."},
		{"a file in the way, the operator creates, command, outside home", failure(platform.PathFileInTheWay, out("work", "config.toml"), out("work")), true, command,
			"Move the file " + out("work") + ", then create " + out("work", "config.toml") + " and the folder that holds it, then run the same command again."},
		{"a file in the way, the operator creates, check, outside home", failure(platform.PathFileInTheWay, out("work", "config.toml"), out("work")), true, check,
			"Move the file in the way, outside the home directory, then create the file outside the home directory and the folder that holds it, then run `aigw sync`."},
		{"a file at AIGW's configuration folder", failure(platform.PathFileInTheWay, in("aigw", "config.toml"), in("aigw")), false, configured,
			"Move the file ~/aigw. If it replaced AIGW's folder, put that folder back from a copy; otherwise run `aigw setup`. Then run the same command again."},
		{"a file above AIGW's configuration folder", failure(platform.PathFileInTheWay, in("aigw", "config.toml"), home+string(filepath.Separator)+"aigw"), false,
			PathContext{Home: home, Configuration: in("aigw", "deeper", "config.toml")},
			"Move the file ~/aigw. If it replaced AIGW's folder, put that folder back from a copy; otherwise run `aigw setup`. Then run the same command again."},
		{"a file inside AIGW's folder is no folder of its own", failure(platform.PathFileInTheWay, in("aigw", "secrets", "tokens"), in("aigw", "secrets")), false, configured,
			"Move the file ~/aigw/secrets, then run the same command again."},
		{"a link that leads nowhere, the operator creates", failure(platform.PathDanglingLink, in("link", "config.toml"), in("link")), true, command,
			"Remove the link ~/link, then create ~/link/config.toml and the folder that holds it, then run the same command again."},
		{"a link that leads nowhere, AIGW creates, check", failure(platform.PathDanglingLink, in("link", "config.toml"), in("link")), false, check,
			"Remove the link ~/link, then run `aigw sync`."},
		{"a link that leads nowhere, check, outside home", failure(platform.PathDanglingLink, out("link", "config.toml"), out("link")), false, check,
			"Remove the link in the way, outside the home directory, then run `aigw sync`."},
		{"a missing file, the operator creates, check", failure(platform.PathMissingFile, in("work", "config.toml"), ""), true, check,
			"Create ~/work/config.toml, then run `aigw sync`."},
		{"a missing folder, the operator creates, command", failure(platform.PathMissingFolder, in("work", "config.toml"), ""), true, command,
			"Create ~/work/config.toml and the folder that holds it, then run the same command again."},
		{"a missing folder, the operator creates, check, outside home", failure(platform.PathMissingFolder, out("work", "config.toml"), ""), true, check,
			"Create the file outside the home directory and the folder that holds it, then run `aigw sync`."},
		{"a missing file that AIGW creates, check", failure(platform.PathMissingFile, in("work", "config.toml"), ""), false, check, "aigw sync"},
		{"a missing file that AIGW creates, command", failure(platform.PathMissingFile, in("work", "config.toml"), ""), false, command, "Run the same command again."},
		{"another error", &platform.PathConditionError{Path: in("work", "config.toml"), Err: syscall.EIO}, false, command,
			"Resolve " + syscall.EIO.Error() + " on ~/work/config.toml, then run the same command again."},
		{"a run that does not know its home, a file in the way", failure(platform.PathFileInTheWay, in("blocked", "aigw"), in("blocked")), false, PathContext{},
			"Move the file in the way, then run the same command again."},
		{"a run that does not know its home, permission", failure(platform.PathPermission, in("work", "config.toml"), ""), false, PathContext{},
			"Make AIGW's folder and its files readable and writable; `aigw config path` prints where the folder is. Then run the same command again."},
		{"a path of AIGW's own outside home, another error", &platform.PathConditionError{Path: out("aigw", "x"), Err: syscall.EIO}, false, PathContext{Home: home},
			"Resolve " + syscall.EIO.Error() + " on a file in AIGW's folder; `aigw config path` prints where the folder is. Then run the same command again."},
		{"a path of AIGW's own outside home is not named", failure(platform.PathFileInTheWay, out("aigw", "config.toml"), out("aigw")), false, PathContext{Home: home, Configuration: out("aigw", "config.toml")},
			"Move the file in the way, outside the home directory. If it replaced AIGW's folder, put that folder back from a copy; otherwise run `aigw setup`. Then run the same command again."},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := pathFix(test.failure, test.operator, test.at)
			if got != test.want {
				t.Fatalf("pathFix = %q\nwant      %q", got, test.want)
			}
			if strings.Contains(got, home) {
				t.Fatalf("the fix names the absolute home: %q", got)
			}
		})
	}
}

// A path failure's title reads its condition's reason on every platform, and
// any other error's closed-set reason.
func TestPathReasonReadsTheConditionOnEveryPlatform(t *testing.T) {
	blocked := &platform.PathConditionError{Condition: platform.PathFileInTheWay, Err: fs.ErrNotExist}
	if got := pathReason(blocked); got != "not a directory" {
		t.Fatalf("a file in the way = %q", got)
	}
	if got := pathReason(&platform.PathConditionError{Err: syscall.EIO}); got != syscall.EIO.Error() {
		t.Fatalf("another error = %q", got)
	}
}
