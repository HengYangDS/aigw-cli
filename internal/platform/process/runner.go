package process

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// Runner executes process plans without consulting shell startup state.
type Runner struct {
	// StdoutLimit bounds captured result data; zero uses the diagnostic default.
	// Captured standard error retains the smaller diagnostic limit.
	StdoutLimit int
}

// CaptureRunner runs a bounded process. It returns standard output on success
// and standard error with an execution error.
type CaptureRunner interface {
	RunCapture(ctx context.Context, plan Plan) ([]byte, error)
}

// VerificationRunner preserves both streams so a successful native warning
// cannot be mistaken for completed verification.
type VerificationRunner interface {
	CaptureRunner
	RunCaptureStreams(ctx context.Context, plan Plan) ([]byte, []byte, error)
}

// nativeDiagnosticSymbols are the warning and error signs that native clients
// print at the start of a diagnostic line.
const nativeDiagnosticSymbols = "⚠✗✖✘❌"

// DiagnosticFailure reports explicit native warning or error markers, not
// ordinary stderr progress or arbitrary response text. A marker is a warning or
// error word, or a warning or error sign that begins a line.
func DiagnosticFailure(diagnostic []byte) bool {
	return diagnosticMarker(diagnostic, false)
}

// diagnosticMarker reports a marker in text. continued says that text's first
// line began in an earlier piece, so that only a word can mark it.
func diagnosticMarker(text []byte, continued bool) bool {
	for line := range strings.SplitSeq(strings.ToLower(ansi.Strip(string(text))), "\n") {
		sign, _ := utf8.DecodeRuneInString(strings.TrimLeftFunc(line, unicode.IsSpace))
		if !continued && strings.ContainsRune(nativeDiagnosticSymbols, sign) {
			return true
		}
		continued = false
		fields := strings.FieldsFunc(line, func(r rune) bool {
			return r <= ' ' || strings.ContainsRune("[]():", r)
		})
		for _, field := range fields {
			if field == "warn" || field == "error" || field == "fatal" || field == "traceback" || strings.HasSuffix(field, "warning") {
				return true
			}
		}
	}
	return false
}

// DiagnosticLimit bounds the head of standard error that RunDiagnosed keeps and
// the part of a line that it holds at once, the budget that a captured standard
// error has.
const DiagnosticLimit = capturedProcessOutputLimit

// diagnosticWordReach is how much of a long line's piece waits for the next
// piece: more than the longest marker and the escape sequence before it.
const diagnosticWordReach = 64

// Diagnostics is the standard error that RunDiagnosed replayed: its first
// DiagnosticLimit bytes, whether more followed, and whether any of its lines
// carries a DiagnosticFailure marker. A line longer than DiagnosticLimit is
// examined in pieces that keep their last word for the next piece, so its
// markers count as in a whole line while memory stays bounded.
type Diagnostics struct {
	Head      []byte
	Truncated bool
	Failure   bool

	unfinished []byte
	continued  bool
}

func (diagnostics *Diagnostics) observe(piece []byte, whole bool) {
	kept := min(len(piece), DiagnosticLimit-len(diagnostics.Head))
	diagnostics.Head = append(diagnostics.Head, piece[:kept]...)
	diagnostics.Truncated = diagnostics.Truncated || kept < len(piece)
	text := append(diagnostics.unfinished, piece...)
	continued := diagnostics.continued
	diagnostics.unfinished, diagnostics.continued = nil, !whole
	if !whole {
		cut := unfinishedWord(text)
		diagnostics.unfinished = bytes.Clone(text[cut:])
		text = text[:cut]
	}
	diagnostics.Failure = diagnostics.Failure || diagnosticMarker(text, continued)
}

// unfinishedWord returns where the part of text that waits for the next piece
// begins. It starts at a space when one is near, so that no word is examined
// cut in two; a word longer than the reach can then only match by its end.
func unfinishedWord(text []byte) int {
	cut := max(len(text)-diagnosticWordReach, 0)
	floor := max(cut-diagnosticWordReach, 0)
	if space := bytes.LastIndexAny(text[floor:cut], " \t\r"); space >= 0 {
		return floor + space + 1
	}
	return cut
}

// RunDiagnosed runs plan as RunStream does with standard error in a native
// temporary file, which keeps what a child writes just before it exits. Once
// the child has exited, it replays that file to stderr line by line, in pieces
// where a line is too long to hold, and reports what the file held; the file is
// removed before return.
func (runner Runner) RunDiagnosed(ctx context.Context, plan Plan, stdout, stderr io.Writer) (diagnostics Diagnostics, err error) {
	capture, err := os.CreateTemp("", "aigw-diagnostics-*")
	if err != nil {
		return Diagnostics{}, fmt.Errorf("create command diagnostics: %w", err)
	}
	defer func() { err = errors.Join(err, capture.Close(), os.Remove(capture.Name())) }()
	runErr := runner.RunStream(ctx, plan, stdout, capture)
	if _, err := capture.Seek(0, io.SeekStart); err != nil {
		return Diagnostics{}, errors.Join(runErr, err)
	}
	reader := bufio.NewReaderSize(capture, DiagnosticLimit)
	for {
		piece, readErr := reader.ReadSlice('\n')
		var writeErr error
		if len(piece) > 0 {
			_, writeErr = stderr.Write(piece)
		}
		tooLong := errors.Is(readErr, bufio.ErrBufferFull)
		diagnostics.observe(piece, !tooLong)
		if tooLong {
			readErr = nil
		}
		if readErr != nil || writeErr != nil {
			if errors.Is(readErr, io.EOF) {
				readErr = nil
			}
			return diagnostics, errors.Join(runErr, readErr, writeErr)
		}
	}
}

// FileRunner streams a process's standard output into an owned file.
type FileRunner interface {
	RunToFile(ctx context.Context, path string, plan Plan) error
}

const (
	capturedProcessOutputLimit = 64 * 1024
	capturedProcessWaitDelay   = 2 * time.Second
)

var errCapturedProcessOutputLimit = errors.New("captured process output exceeds limit")

// limitedBuffer deliberately does not embed [bytes.Buffer]: embedding would
// promote ReadFrom, and [io.Copy] would bypass Write's capture ceiling.
type limitedBuffer struct {
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.overflow = true
		return 0, errCapturedProcessOutputLimit
	}
	if len(data) > remaining {
		_, _ = b.buf.Write(data[:remaining])
		b.overflow = true
		return remaining, errCapturedProcessOutputLimit
	}
	return b.buf.Write(data)
}

func (b *limitedBuffer) Bytes() []byte { return b.buf.Bytes() }

func (b *limitedBuffer) String() string { return b.buf.String() }

// RunCapture runs a bounded, non-interactive process invocation. It returns
// standard output on success and standard error with a child-process failure.
// Captured bytes remain untrusted until the owning caller redacts them.
func (runner Runner) RunCapture(ctx context.Context, plan Plan) ([]byte, error) {
	stdout, diagnostic, err := runner.RunCaptureStreams(ctx, plan)
	if err != nil {
		return diagnostic, err
	}
	return stdout, nil
}

// RunCaptureStreams retains both bounded output streams, including when the
// child exits unsuccessfully. Callers must redact captured bytes before logging.
func (runner Runner) RunCaptureStreams(ctx context.Context, plan Plan) ([]byte, []byte, error) {
	if runner.StdoutLimit < 0 {
		return nil, nil, fmt.Errorf("captured stdout limit must not be negative")
	}
	outputLimit := runner.StdoutLimit
	if outputLimit == 0 {
		outputLimit = capturedProcessOutputLimit
	}
	stdout := &limitedBuffer{limit: outputLimit}
	diagnostic, err := runCaptured(ctx, plan, stdout)
	if stdout.overflow {
		return nil, nil, fmt.Errorf("captured stdout from %s exceeds %d bytes", plan.Executable, outputLimit)
	}
	return append([]byte(nil), stdout.Bytes()...), diagnostic, err
}

// RunToFile streams standard output without a memory capture limit. Failure
// removes the partial file; diagnostics retain the shared capture budget.
func (Runner) RunToFile(ctx context.Context, destination string, plan Plan) error {
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open command output %s: %w", filepath.Base(destination), err)
	}
	diagnostic, runErr := runCaptured(ctx, plan, file)
	closeErr := file.Close()
	if runErr == nil && closeErr == nil {
		return nil
	}
	removeErr := os.Remove(destination)
	if runErr != nil {
		runErr = fmt.Errorf("%s failed: %w: %s", plan.Executable, runErr, strings.TrimSpace(string(diagnostic)))
	}
	if closeErr != nil {
		closeErr = fmt.Errorf("close command output %s: %w", filepath.Base(destination), closeErr)
	}
	return errors.Join(runErr, closeErr, removeErr)
}

func runCaptured(ctx context.Context, plan Plan, stdout io.Writer) (diagnostic []byte, err error) {
	stderr := &limitedBuffer{limit: capturedProcessOutputLimit}
	// Capture and downloads keep destination handles in the parent.
	err = (Runner{}).RunStream(ctx, plan, struct{ io.Writer }{stdout}, stderr)
	if stderr.overflow {
		return nil, fmt.Errorf("captured stderr from %s exceeds %d bytes", plan.Executable, capturedProcessOutputLimit)
	}
	return append([]byte(nil), stderr.Bytes()...), err
}

// RunStream executes an owned non-interactive process with caller-owned output
// streams. Cancellation and return reclaim its native process group or Job.
func (Runner) RunStream(ctx context.Context, plan Plan, stdout, stderr io.Writer) (err error) {
	if plan.DebugProcess {
		if runtime.GOOS != "windows" || plan.OnStart == nil {
			return errors.New("native debug execution requires a Windows observer")
		}
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	cmd := commandContext(ctx, plan)
	cmd.Dir = plan.Directory
	cmd.Env = plan.Env
	cmd.Stdin = strings.NewReader(plan.Stdin)
	cmd.WaitDelay = capturedProcessWaitDelay
	// Native files retain immediate-exit writes. Other writers use bounded
	// output pipes; the owned process group or job is reclaimed before return.
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cleanup, err := startCapturedProcess(cmd)
	if err != nil {
		// A context that ends during the start stops the child before it runs.
		return fmt.Errorf("start %s: %w", plan.Executable, errors.Join(err, ctx.Err()))
	}
	defer func() { err = errors.Join(err, cleanup()) }()
	if plan.OnStart != nil {
		if observeErr := plan.OnStart(cmd.Process); observeErr != nil {
			return fmt.Errorf("observe started %s: %w", plan.Executable, errors.Join(observeErr, stopCapturedCommand(cmd)))
		}
	}
	if err := cmd.Wait(); err != nil {
		if errors.Is(err, exec.ErrWaitDelay) {
			err = fmt.Errorf("output pipes did not close within %s: %w", capturedProcessWaitDelay, err)
		}
		return fmt.Errorf("run %s: %w", plan.Executable, errors.Join(err, ctx.Err()))
	}
	return nil
}
