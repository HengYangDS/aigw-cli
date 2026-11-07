package adapter

import (
	"errors"
	"slices"

	"aigw-cli/internal/platform"
)

// ApplyError is an application of client files that failed at one file:
// the client whose file failed, the file and its failure, and what undoing
// the application did to each file that it had written. Home is the home
// directory from which the adapters derived the files, against which a report
// names them.
type ApplyError struct {
	Client   string
	Home     string
	Failed   *platform.FileError
	Restored platform.Restoration
	Err      error
}

// Error returns the application's failure.
func (e *ApplyError) Error() string { return e.Err.Error() }

// Unwrap returns the application's failure.
func (e *ApplyError) Unwrap() error { return e.Err }

// RestoringReceipt is a receipt whose rollback reports what it did to each
// file that its application wrote.
type RestoringReceipt interface {
	ProjectionReceipt
	Restore() platform.Restoration
}

// Restore rolls every receipt back in reverse order and reports each file. A
// receipt that reports no files is rolled back, and a failure of its rollback
// is reported without a file.
func (receipts ProjectionReceipts) Restore() platform.Restoration {
	var restored platform.Restoration
	for _, receipt := range slices.Backward(receipts) {
		switch typed := receipt.(type) {
		case nil:
		case RestoringReceipt:
			restored = append(restored, typed.Restore()...)
		default:
			if err := typed.Rollback(); err != nil {
				restored = append(restored, platform.FileOutcome{Err: err})
			}
		}
	}
	return restored
}

// applyFailure reports err, the failure of client's application after the
// receipts of the clients before it were rolled back as restored reports, as
// the failure of a file when err names one.
func applyFailure(client ClientLabel, home string, failure, err error, restored platform.Restoration) error {
	failed, atFile := errors.AsType[*platform.FileError](failure)
	if !atFile {
		return err
	}
	var own platform.Restoration
	if written, ok := errors.AsType[*platform.WriteError](failure); ok {
		own = written.Restored
	}
	return &ApplyError{Client: string(client), Home: home, Failed: failed, Restored: append(slices.Clone(own), restored...), Err: err}
}

// ClientLabel is the label of the client that a report names.
type ClientLabel string
