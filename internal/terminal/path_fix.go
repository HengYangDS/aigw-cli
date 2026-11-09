package terminal

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"aigw-cli/internal/platform"
)

// PathContext is what a run knows about the paths that its failures name: the
// home directory, under which a path is named ~/…, AIGW's configuration file,
// whose unreadable file and blocked folder have fixes of their own, whether a
// read-only check presents the failure, and whether the operator gave the
// path, as a client's file or a --path is given. A check completes a repair
// with `aigw sync`, since its next failure would otherwise be a drift that
// only sync repairs, and names no path outside the home directory; a command
// completes it by running again and names such a path as the operator
// configured it. A path of AIGW's own outside home, such as a test's or a
// relocated configuration folder, is never named, and a run that does not
// know its home names no path.
type PathContext struct {
	Home          string
	Configuration string
	Check         bool
	Given         bool
}

// configurationReadFix is the fix of a configuration file that cannot be read.
const configurationReadFix = "Restore read access to the configuration file; `aigw config path` prints its location."

// pathFix is the one fix of a path failure, which failure classifies:
// operatorCreates says that the operator creates the missing file and its
// folder, as for a client's file that AIGW does not create, while AIGW
// creates its own paths as it writes them. The fix ends with the step that
// completes the repair in at.
func pathFix(failure *platform.PathConditionError, operatorCreates bool, at PathContext) string {
	switch failure.Condition {
	case platform.PathPermission:
		if at.configurationFile(failure.Path) {
			return configurationReadFix
		}
		if at.unnamed(failure.Path) {
			return "Make AIGW's folder and its files readable and writable; `aigw config path` prints where the folder is. Then " + at.step() + "."
		}
		return "Make " + at.name(failure.Path, theFile) + " and its folder readable and writable, then " + at.step() + "."
	case platform.PathFileInTheWay, platform.PathDanglingLink:
		return at.unblock(failure, operatorCreates)
	case platform.PathMissingFolder, platform.PathMissingFile:
		return at.create(failure, operatorCreates)
	case platform.PathOther:
	}
	if at.unnamed(failure.Path) {
		return "Resolve " + pathReason(failure) + " on a file in AIGW's folder; `aigw config path` prints where the folder is. Then " + at.step() + "."
	}
	return "Resolve " + pathReason(failure) + " on " + at.name(failure.Path, theFile) + ", then " + at.step() + "."
}

// unnamed reports a path of AIGW's own that a fix cannot name: one outside the
// home directory, or in a run that does not know its home. `aigw config path`
// prints AIGW's folder in every state, so the fix points there.
func (at PathContext) unnamed(path string) bool {
	target, _ := platform.HomeTarget(path, at.Home)
	return target == "" && !at.Given
}

// pathReason is why an operation failed at a path: its condition's reason,
// the same on every platform, or the closed-set reason of any other error.
func pathReason(failure *platform.PathConditionError) string {
	if reason := failure.Reason(); reason != "" {
		return reason
	}
	return fileErrorReason(failure.Err)
}

// pathFailureIn is the path failure that err reports: the classification that
// the failed operation made, or else the operating-system error at a path,
// classified here as a folder that an operation creates and otherwise a file.
// An error at no path has none.
func pathFailureIn(err error) *platform.PathConditionError {
	if classified, done := errors.AsType[*platform.PathConditionError](err); done {
		return classified
	}
	if failure, atPath := errors.AsType[*fs.PathError](err); atPath {
		kind := platform.FileExpected
		if failure.Op == "mkdir" {
			kind = platform.FolderExpected
		}
		return platform.ClassifyPath(failure.Path, kind, err)
	}
	if failure, atLink := errors.AsType[*os.LinkError](err); atLink {
		return platform.ClassifyPath(failure.New, platform.FileExpected, err)
	}
	return nil
}

// unblock moves the file in the way, or removes the link that leads nowhere,
// and then completes the repair: by putting AIGW's own folder back, by
// creating the file that the operator creates, or by the step alone, after
// which AIGW creates what it writes.
func (at PathContext) unblock(failure *platform.PathConditionError, operatorCreates bool) string {
	lead := "Move the " + at.name(failure.Culprit, theFileInTheWay)
	if failure.Condition == platform.PathDanglingLink {
		lead = "Remove the " + at.name(failure.Culprit, theLink)
	}
	switch {
	case at.blocksConfiguration(failure.Culprit):
		return lead + ". If it replaced AIGW's folder, put that folder back from a copy; otherwise run `aigw setup`. Then run the same command again."
	case operatorCreates:
		return lead + ", then create " + at.name(failure.Path, theFile) + " and the folder that holds it, then " + at.step() + "."
	default:
		return lead + ", then " + at.step() + "."
	}
}

// create names what the operator creates of a missing path, the file alone
// when its folder exists. AIGW creates its own paths as it writes them, so
// only a path that disappeared while the command ran reaches a fix of AIGW's:
// a check names the sync that creates it again, and a command runs again.
func (at PathContext) create(failure *platform.PathConditionError, operatorCreates bool) string {
	switch {
	case !operatorCreates && at.Check:
		return "aigw sync"
	case !operatorCreates:
		return "Run the same command again."
	case failure.Condition == platform.PathMissingFile:
		return "Create " + at.name(failure.Path, theFile) + ", then " + at.step() + "."
	default:
		return "Create " + at.name(failure.Path, theFile) + " and the folder that holds it, then " + at.step() + "."
	}
}

// step is the step that completes a repair: a check's sync, or a command
// again.
func (at PathContext) step() string {
	if at.Check {
		return "run `aigw sync`"
	}
	return "run the same command again"
}

// pathRole is what a fix names a path as when it names no path.
type pathRole int

const (
	theFile pathRole = iota
	theFileInTheWay
	theLink
)

// name names path in a fix: inside the home directory as ~/…, and outside it
// as configured by a command. A check names no path outside home, and says
// that the file lies outside it; a run that does not know its home names no
// path at all. A file in the way and a link follow "Move the" and "Remove the",
// and a file stands alone.
func (at PathContext) name(path string, role pathRole) string {
	prefix := map[pathRole]string{theFile: "", theFileInTheWay: "file ", theLink: "link "}[role]
	if target, _ := platform.HomeTarget(path, at.Home); target != "" {
		return prefix + target
	}
	if at.Home != "" && !at.Check && at.Given && path != "" {
		return prefix + path
	}
	outside := ""
	if at.Home != "" {
		outside = ", outside the home directory"
	}
	switch role {
	case theFileInTheWay:
		return "file in the way" + outside
	case theLink:
		return "link in the way" + outside
	case theFile:
	}
	if outside != "" {
		return "the file outside the home directory"
	}
	return "the file"
}

// configurationFile reports whether path is AIGW's configuration file.
func (at PathContext) configurationFile(path string) bool {
	return at.Configuration != "" && filepath.Clean(path) == filepath.Clean(at.Configuration)
}

// blocksConfiguration reports whether culprit, a file in the way or a link,
// stands at AIGW's configuration folder or above it.
func (at PathContext) blocksConfiguration(culprit string) bool {
	if at.Configuration == "" || culprit == "" {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(culprit), filepath.Dir(filepath.Clean(at.Configuration)))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
