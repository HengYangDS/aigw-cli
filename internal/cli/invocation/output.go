package invocation

import (
	"io"

	"aigw-cli/internal/terminal"
)

// Output owns write progress and the first failure for one invocation: what
// the command writes reaches Writer, and Fields adds the invocation's own
// fields to the command's document, whose ok Failed makes false.
type Output struct {
	Writer  io.Writer
	Fields  func() []terminal.DocumentField
	Failed  func() bool
	err     error
	started bool
}

// Err is the first failure to write, if one happened.
func (w *Output) Err() error { return w.err }

// Started reports whether the command wrote anything.
func (w *Output) Started() bool { return w.started }

// DocumentFailed reports whether the invocation failed beside the command's
// result, which makes its document's ok false.
func (w *Output) DocumentFailed() bool {
	return w.Failed != nil && w.Failed()
}

// DocumentFields are the invocation's own fields of the command's document.
func (w *Output) DocumentFields() []terminal.DocumentField {
	if w.Fields == nil {
		return nil
	}
	return w.Fields()
}

// Write writes data to Writer unless an earlier write failed, and keeps the
// first failure.
func (w *Output) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	count, writeErr := w.Writer.Write(data)
	w.started = w.started || count > 0
	if writeErr == nil && count != len(data) {
		writeErr = io.ErrShortWrite
	}
	w.err = writeErr
	return count, writeErr
}
