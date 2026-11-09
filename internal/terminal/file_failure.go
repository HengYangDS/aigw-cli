package terminal

import (
	"errors"
	"strings"

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
		Fix:   fileFix(failure.Failed, failed, failure.Home),
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

// GivenPathFailure presents err, a failure at a path that the operator gave,
// such as a --path or a --target, which the operation classified or which an
// operating-system error names, as a problem whose title names the path and
// its reason, the same on every platform, and whose fix is the one that the
// path's condition needs in at. operatorCreates says that the operator creates
// what is missing, as for a client's file that AIGW does not create. Any other
// error returns unchanged.
func GivenPathFailure(err error, at platform.PathContext, operatorCreates bool) error {
	path := pathFailureIn(err)
	if path == nil {
		return err
	}
	path = path.Owned(operatorCreates)
	return ProblemError(pathTitle(path, at), "", "", platform.PathFix(path, at), err)
}

// targetReason says why a client file could not be used: an operating-system
// error at a path by its condition's reason, which names no path and reads the
// same on every platform, or else the failure's own text, such as a refusal
// that says why AIGW does not create the file.
func targetReason(err error) string {
	if atOperatingSystem(err) {
		return pathReason(pathFailureIn(err))
	}
	return err.Error()
}

// targetFix is the fix of a client file that a command could not use: the
// fix that an outer refusal states for itself, or else the one that the file's
// path condition needs, named against target's home directory; the path
// failure's own owner says who creates what is missing.
func targetFix(err error, target *adapter.TargetError) string {
	if fix, stated := statedFix(err, ""); stated {
		return fix
	}
	at := platform.PathContext{Home: target.Home}
	// The file is named as configured, never as its links resolve, so the
	// classification is the operation's own, or else made at that file.
	if pathFailureIn(err) != nil {
		path := platform.ClassifyPath(target.Path, platform.FileExpected, err)
		if !path.Given {
			path = path.Owned(target.OperatorCreates)
		}
		return platform.PathFix(path, at)
	}
	return errorFix(err, err.Error(), at)
}

// fileFix is the fix of a file that could not be written, which name names: a
// writable file and folder for a permission error, the same command again for
// a file that changed while AIGW wrote it, which the command then reads as it
// is, and any other operating-system error resolved first. It never names a
// check, whose probes may use quota and fix nothing here.
func fileFix(failed *platform.FileError, name, home string) string {
	if failed.Changed {
		return "Run the same command again; it reads " + name + " as it is now."
	}
	// A client file that AIGW writes, which the operator configured: a flag on
	// the file or on its folder may refuse the write.
	path := platform.ClassifyPath(failed.Path, platform.FileExpected, failed.Err).Owned(false).Written()
	return platform.PathFix(path, platform.PathContext{Home: home})
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
