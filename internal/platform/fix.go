package platform

import (
	"path/filepath"
	"strings"
)

// PathContext is what a run knows about the paths that its failures name: the
// home directory, under which a path is named ~/…, AIGW's configuration file,
// whose unreadable file and blocked folder have fixes of their own, and
// whether a read-only check presents the failure. A check completes a repair
// with `aigw sync`, since its next failure would otherwise be a drift that
// only sync repairs, and names no path outside the home directory; a command
// completes it by running again and names such a path as the operator gave it.
// A path of AIGW's own outside home, such as a test's or a relocated
// configuration folder, is never named, and a run that does not know its home
// names no path. The home comes from the run, never from the process's
// environment.
type PathContext struct {
	Home          string
	Configuration string
	Check         bool
}

// ConfigurationReadFix is the fix of a configuration file that cannot be read.
const ConfigurationReadFix = "Restore read access to the configuration file; `aigw config path` prints its location."

// PathFix is the one fix of a path failure: its condition, who creates what is
// missing, as the failure says (the operator creates a client's file that AIGW
// does not create, and AIGW creates its own paths as it writes them), and the
// step that completes the repair in at. Every surface that presents a path
// failure renders its fix here.
func PathFix(failure *PathConditionError, at PathContext) string {
	switch failure.Condition {
	case PathPermission:
		if at.configurationFile(failure.Path) {
			return ConfigurationReadFix
		}
		if at.unnamed(failure, failure.Path) {
			return "Make AIGW's folder and its files readable and writable; `aigw config path` prints where the folder is. Then " + at.step() + "."
		}
		return "Make " + at.name(failure, failure.Path, theFile) + " and its folder readable and writable, then " + at.step() + "."
	case PathFileInTheWay, PathDanglingLink:
		return at.unblock(failure)
	case PathMissingFolder, PathMissingFile:
		return at.create(failure)
	case PathOther:
	}
	if at.unnamed(failure, failure.Path) {
		return "Resolve " + failure.reasonOr("unexpected error") + " on a file in AIGW's folder; `aigw config path` prints where the folder is. Then " + at.step() + "."
	}
	return "Resolve " + failure.reasonOr("unexpected error") + " on " + at.name(failure, failure.Path, theFile) + ", then " + at.step() + "."
}

// Name names path, a path of failure, as every fix and title does: inside the
// home directory as ~/…, outside it as the operator gave it, when the operator
// did and no check presents it, and otherwise not at all.
func (at PathContext) Name(failure *PathConditionError, path string) string {
	return at.name(failure, path, theFile)
}

// unnamed reports a path of AIGW's own that a fix cannot name: one outside the
// home directory, or in a run that does not know its home. `aigw config path`
// prints AIGW's folder in every state, so the fix points there.
func (at PathContext) unnamed(failure *PathConditionError, path string) bool {
	target, _ := HomeTarget(path, at.Home)
	return target == "" && !failure.Given
}

// unblock moves the file in the way, or removes the link that leads nowhere,
// and then completes the repair: by putting AIGW's own folder back, by
// creating the file that the operator creates, or by the step alone, after
// which AIGW creates what it writes.
func (at PathContext) unblock(failure *PathConditionError) string {
	lead := "Move the " + at.name(failure, failure.Culprit, theFileInTheWay)
	if failure.Condition == PathDanglingLink {
		lead = "Remove the " + at.name(failure, failure.Culprit, theLink)
	}
	switch {
	case at.blocksConfiguration(failure.Culprit):
		return lead + ". If it replaced AIGW's folder, put that folder back from a copy; otherwise run `aigw setup`. Then run the same command again."
	case failure.OperatorCreates:
		return lead + ", then create " + at.name(failure, failure.Path, theFile) + " and the folder that holds it, then " + at.step() + "."
	default:
		return lead + ", then " + at.step() + "."
	}
}

// create names what the operator creates of a missing path, the file alone
// when its folder exists. AIGW creates its own paths as it writes them, so
// only a path that disappeared while the command ran reaches a fix of AIGW's:
// a check names the sync that creates it again, and a command runs again.
func (at PathContext) create(failure *PathConditionError) string {
	switch {
	case !failure.OperatorCreates && at.Check:
		return "aigw sync"
	case !failure.OperatorCreates:
		return "Run the same command again."
	case failure.Condition == PathMissingFile:
		return "Create " + at.name(failure, failure.Path, theFile) + ", then " + at.step() + "."
	default:
		return "Create " + at.name(failure, failure.Path, theFile) + " and the folder that holds it, then " + at.step() + "."
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
func (at PathContext) name(failure *PathConditionError, path string, role pathRole) string {
	prefix := map[pathRole]string{theFile: "", theFileInTheWay: "file ", theLink: "link "}[role]
	if target, _ := HomeTarget(path, at.Home); target != "" {
		return prefix + target
	}
	// A path that the operator gave is safe to name as given, even by a run
	// that does not know its home; a check names none outside home.
	if !at.Check && failure.Given && path != "" {
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
