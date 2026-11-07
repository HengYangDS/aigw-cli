package terminal

import (
	"errors"
	"io/fs"
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/platform"
)

// clientFileProblem is the problem of an application of client files that
// failed at one file: the client, the file and why, what undoing the
// application did to each file that it had written, whether the
// configuration was restored, and the fix. A file is named relative to the
// home directory, and a file elsewhere is not named.
func clientFileProblem(failure *adapter.ApplyError, err error) Problem {
	failed := fileName(failure.Failed.Path, failure.Home)
	problem := Problem{
		Title: failure.Client + " could not write " + failed + ": " + writeReason(failure.Failed),
		Fix:   "aigw check",
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
		}
	}
	if errors.Is(failure.Failed, fs.ErrPermission) {
		problem.Fix = "Make " + failed + " and its folder writable, then run the same command again."
	}
	return problem
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

// fileName names a file relative to the home directory, or says that it lies
// outside it, without naming it.
func fileName(path, home string) string {
	target, outside := HomeTarget(path, home)
	switch {
	case target != "":
		return target
	case outside:
		return "a file outside the home directory"
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
