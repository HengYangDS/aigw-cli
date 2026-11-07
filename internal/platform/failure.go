package platform

import (
	"errors"
	"fmt"
	"slices"
)

// FileError is the failure of a guarded write, removal or restore of one
// file: the file, and the failure, whose text it keeps. Changed marks a file
// that differed from what the transaction had read, which the guard refuses
// to overwrite.
type FileError struct {
	Path    string
	Changed bool
	Err     error
}

// Error returns the failure's own text.
func (e *FileError) Error() string { return e.Err.Error() }

// Unwrap returns the failure.
func (e *FileError) Unwrap() error { return e.Err }

// fileError names the file that err concerns, keeping err's text.
func fileError(path string, changed bool, err error) error {
	if err == nil {
		return nil
	}
	return &FileError{Path: path, Changed: changed, Err: err}
}

// FileOutcome is one file that a failed attempt had written, and the failure
// of its restore, nil when it was restored.
type FileOutcome struct {
	Path string
	Err  error
}

// Restoration is what undoing a failed attempt did to each file that the
// attempt had written, in the order of the undo.
type Restoration []FileOutcome

// Err joins the restores that failed, each naming its file, and is nil when
// every file was restored.
func (restoration Restoration) Err() error {
	var failures []error
	for _, outcome := range restoration {
		switch {
		case outcome.Err == nil:
		case outcome.Path == "":
			failures = append(failures, outcome.Err)
		default:
			failures = append(failures, fmt.Errorf("restore %s: %w", outcome.Path, outcome.Err))
		}
	}
	return errors.Join(failures...)
}

// Restore restores path to preimage while it still holds the attempt's
// postimage, and reports the outcome.
func Restore(path string, preimage, postimage FileSnapshot) FileOutcome {
	return FileOutcome{Path: path, Err: RestoreFileAtomicIfPostimage(path, preimage, postimage)}
}

// WriteError is a failed attempt to write several files that undid the
// files it had written: its failure, as its owner reports it, and what the
// undo did to each file. A restore that failed makes the compensation
// incomplete.
type WriteError struct {
	Err      error
	Restored Restoration
}

// Error returns the attempt's failure.
func (e *WriteError) Error() string { return e.Err.Error() }

// Unwrap returns the attempt's failure.
func (e *WriteError) Unwrap() error { return e.Err }

// FailedWrite reports a write that failed with err after the attempt had
// written the files that restored lists, and undid them: err itself, marked
// as an incomplete compensation when one of those restores failed. The report
// starts with what an inner attempt that err reports undid first.
func FailedWrite(err error, restored Restoration) error {
	report := restored
	if inner, ok := errors.AsType[*WriteError](err); ok {
		report = append(slices.Clone(inner.Restored), restored...)
	}
	if len(report) == 0 {
		return err
	}
	return &WriteError{Err: CompensationFailed(err, restored.Err()), Restored: report}
}
