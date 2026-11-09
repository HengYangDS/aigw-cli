package terminal

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/platform"
)

// restoreFailedFix is the fix of a failure whose configuration could not be
// restored either: a local check, which inspects the configuration and the
// client files without a request, rather than the same command again, which
// could not tell what it left behind.
const restoreFailedFix = "aigw check --depth local"

// clientFileProblem is the problem of an application of client files that
// failed at one file: the client, the file and why, what undoing the
// application did to each file that it had written, whether the
// configuration was restored, and the fix. A file is named relative to the
// home directory, and a file elsewhere by its full path, which the operator
// chose.
func clientFileProblem(failure *adapter.ApplyError, err error) Problem {
	failed := fileName(failure.Failed.Path, failure.Home)
	problem := Problem{
		Title: failure.Client + " could not write " + failed + ": " + writeReason(failure.Failed),
		Fix:   fileFix(failure.Failed, failed),
	}
	evidence := make([]string, 0, len(failure.Restored)+1)
	for _, outcome := range failure.Restored {
		evidence = append(evidence, restoreOutcome(outcome, failure.Home))
	}
	if failure.Failed.Changed {
		evidence = append(evidence, sentenceStart(failed)+" keeps the change made outside AIGW.")
	} else {
		evidence = append(evidence, sentenceStart(failed)+" is unchanged.")
	}
	problem.Evidence = strings.Join(evidence, " ")
	if recovery, ok := errors.AsType[configurationRecovery](err); ok {
		problem.Impact = "The configuration was restored."
		if !recovery.ConfigurationRestored() {
			problem.Impact = "Restoring the configuration failed too, so it may still hold the change."
			problem.Fix = restoreFailedFix
		}
	}
	return problem
}

// targetProblem is the problem of a client file that a command could not use
// while it planned the client's files: the client, the file as the operator
// configured it and why, what became of the configuration, and the fix that
// the failure names, or else one that acts on a file error.
func targetProblem(failure *adapter.TargetError, err error) Problem {
	name := fileName(failure.Path, failure.Home)
	problem := Problem{
		Title:  failure.Client + " cannot use " + name + ": " + targetReason(failure.Err),
		Impact: "Configuration and client files are unchanged.",
	}
	problem.Fix = targetFix(err, failure)
	if recovery, ok := errors.AsType[configurationRecovery](err); ok {
		problem.Impact = "The configuration was restored."
		if !recovery.ConfigurationRestored() {
			problem.Impact = "Restoring the configuration failed too, so it may still hold the change."
			problem.Fix = restoreFailedFix
		}
	}
	return problem
}

// FileAccessFailure presents an operating-system error at file, which the
// operator named, with the step that resolves it and the same command again.
// A file inside home is named as ~/…, and one outside it as configured, which
// the operator chose. Resolving a path can report a bare error number, with no
// path around it. Any other error returns unchanged.
func FileAccessFailure(err error, file, home string) error {
	_, atPath := errors.AsType[*fs.PathError](err)
	_, atLink := errors.AsType[*os.LinkError](err)
	_, known := knownFileErrorReason(err)
	if !atPath && !atLink && !known {
		return err
	}
	name := namedFile(file, home)
	reason := fileErrorReason(err)
	fix := "Resolve " + reason + " on " + name + ", then run the same command again."
	inTheWay, throughFile := fileInTheWay(err, file)
	switch {
	case errors.Is(err, fs.ErrPermission):
		fix = "Make " + name + " and its folder writable, then run the same command again."
	case throughFile:
		// Before a missing path, which Windows' ENOTDIR also counts as.
		reason = platform.ErrFileAsFolder.Error()
		fix = "Move the file " + namedFile(inTheWay, home) + ", or choose another path, then run the same command again."
	case errors.Is(err, fs.ErrNotExist):
		fix = "Create " + namedFile(filepath.Dir(file), home) + ", or choose another path, then run the same command again."
	}
	return ProblemError("AIGW cannot use "+name+": "+reason, "", "", fix, err)
}

// fileInTheWay is the regular file that stands where a folder is needed in
// err, and whether there is one. RefuseFileAsFolder names it. A raw ENOTDIR
// from an operation means the same on POSIX, but on Windows it is
// ERROR_PATH_NOT_FOUND, which a missing folder gives too, so the file is
// looked for on the path rather than inferred from the error.
func fileInTheWay(err error, file string) (string, bool) {
	failure, atPath := errors.AsType[*fs.PathError](err)
	if errors.Is(err, platform.ErrFileAsFolder) && atPath {
		return failure.Path, true
	}
	if !errors.Is(err, syscall.ENOTDIR) {
		return "", false
	}
	path := file
	if atPath && failure.Path != "" {
		path = failure.Path
	}
	// The path itself is included, because an operation that creates it reports
	// the file in the way as its path.
	if found, atFile := errors.AsType[*fs.PathError](platform.FileInTheWay(path)); atFile {
		return found.Path, true
	}
	return "", false
}

// targetReason says why a client file could not be used: an operating-system
// error by its reason, which names no path, or else the failure's own text.
func targetReason(err error) string {
	if _, atPath := errors.AsType[*fs.PathError](err); atPath {
		return fileErrorReason(err)
	}
	return err.Error()
}

// targetFix is the fix that a failure to use a client file names for itself,
// or else, for an operating-system error, the file made readable or the error
// resolved, and the same command again. A file that stands where a folder is
// needed is named to move, and a client file that does not exist is named with
// the folder that holds it to create, both against home, so a local check
// names no private path.
func targetFix(err error, target *adapter.TargetError) string {
	if fix, stated := refusedFix(err, target.Home); stated {
		return fix
	}
	name := fileName(target.Path, target.Home)
	_, atPath := errors.AsType[*fs.PathError](err)
	blocking, throughFile := fileInTheWay(err, target.Path)
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "Make " + name + " readable, then run the same command again."
	case throughFile:
		// Before a missing path, which Windows' ENOTDIR also counts as.
		return "Move the file " + fileName(blocking, target.Home) + ", or choose another path, then run the same command again."
	case atPath:
		return "Resolve " + fileErrorReason(err) + " on " + name + ", then run the same command again."
	default:
		return errorFix(err, err.Error())
	}
}

// refusedFix is the fix of a refusal that renders itself against home, then
// any other fix that err states for itself.
func refusedFix(err error, home string) (string, bool) {
	if homed, ok := errors.AsType[interface {
		error
		TargetFix(string) string
	}](err); ok {
		return homed.TargetFix(home), true
	}
	if fix, stated := statedFix(err, ""); stated {
		return fix, true
	}
	return "", false
}

// fileFix is the fix of a file that could not be written, which name names: a
// writable file and folder for a permission error, the same command again for
// a file that changed while AIGW wrote it, which the command then reads as it
// is, and any other operating-system error resolved first. It never names a
// check, whose probes may use quota and fix nothing here.
func fileFix(failed *platform.FileError, name string) string {
	switch {
	case errors.Is(failed, fs.ErrPermission):
		return "Make " + name + " and its folder writable, then run the same command again."
	case failed.Changed:
		return "Run the same command again; it reads " + name + " as it is now."
	default:
		return "Resolve " + fileErrorReason(failed) + " on " + name + ", then run the same command again."
	}
}

// restoreOutcome says what undoing a failed application did to one file that
// it had written.
func restoreOutcome(outcome platform.FileOutcome, home string) string {
	if outcome.Path == "" {
		return "Another client file could not be restored: " + fileErrorReason(outcome.Err) + "."
	}
	name := sentenceStart(fileName(outcome.Path, home))
	failed, atFile := errors.AsType[*platform.FileError](outcome.Err)
	switch {
	case outcome.Err == nil:
		return name + " was restored."
	case atFile && failed.Changed:
		return name + " was not restored, because it changed after AIGW wrote it."
	default:
		return name + " could not be restored: " + fileErrorReason(outcome.Err) + "."
	}
}

// writeReason says why a write of a file failed: the operating-system error,
// or that the file changed after AIGW read it.
func writeReason(failed *platform.FileError) string {
	if failed.Changed {
		return "it changed after AIGW read it"
	}
	return fileErrorReason(failed)
}

// namedFile names a path as the operator names it: relative to the home
// directory as ~/…, or else as configured.
func namedFile(path, home string) string { return fileName(path, home) }

// fileName names a file relative to the home directory, or else by its full
// path, as the operator configured it.
func fileName(path, home string) string {
	target, _ := platform.HomeTarget(path, home)
	switch {
	case target != "":
		return target
	case path != "":
		return path
	default:
		return "a client file"
	}
}

// sentenceStart capitalizes a phrase that starts a sentence, leaving a file
// name as it is.
func sentenceStart(phrase string) string {
	if rest, found := strings.CutPrefix(phrase, "a "); found {
		return "A " + rest
	}
	return phrase
}
