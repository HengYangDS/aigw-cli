// Package verification holds what every client's live model probe shares: the
// turn deadline, bounded response reads, failure messages and the retry command.
package verification

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"

	"github.com/charmbracelet/x/ansi"
)

// ProtocolTimeout bounds each native turn, including cold client startup.
const ProtocolTimeout = time.Minute

// ReadBoundedFile reads a client's response file and refuses one larger than limit.
func ReadBoundedFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return data, nil
}

// A client's report is at most reportLines distinct lines of at most
// reportLineRunes characters. A message names at most deprecatedKeyLimit
// configuration keys of at most deprecatedKeyLength characters.
const (
	reportLines         = 4
	reportLineRunes     = 300
	deprecatedKeyLimit  = 3
	deprecatedKeyLength = 64
)

// deprecatedKey is a configuration key that a client calls deprecated: in
// backticks before the words, as a path of at least two lowercase names, each
// starting with a letter, the first of them optionally a table in brackets. A
// message is AIGW's own text, so the grammar admits no sentence; a single
// word, a capital and a name that starts with a digit can each be a value,
// such as a credential, that the client quotes.
var deprecatedKey = regexp.MustCompile("`((?:\\[[a-z][a-z0-9_-]{0,31}(?:\\.[a-z][a-z0-9_-]{0,31})*\\]|[a-z][a-z0-9_-]{0,31})(?:\\.[a-z][a-z0-9_-]{0,31})+)` is deprecated")

// reportedError is a verification that a native client's own warning or error
// leaves unqualified. Its message and remedy are AIGW's and carry none of the
// client's sentences; its report is the client's own lines, for a reader who
// asks.
type reportedError struct {
	message string
	remedy  string
	// text is the client's output without control sequences, with the known
	// credential forms redacted.
	text string
}

func (failure reportedError) Error() string { return failure.message }

// Remedy says what to change, when AIGW can name it without the client's
// sentences, or nothing.
func (failure reportedError) Remedy() string { return failure.remedy }

// ClientReport returns the client's own lines without secrets, bounded: the
// lines that carry a warning marker, or the first lines when none does.
func (failure reportedError) ClientReport(secrets ...string) []string {
	return clientReport(failure.text, secrets...)
}

// unreportedError is a verification failure whose client report a caller
// withheld. It does not unwrap, so no reader finds the report behind it.
type unreportedError struct{ message string }

func (failure unreportedError) Error() string { return failure.message }

// WithoutReport returns err without a client's report, for a client whose
// output a runner must not pass on; another error is returned as it is.
func WithoutReport(err error) error {
	if _, reported := errors.AsType[reportedError](err); !reported {
		return err
	}
	return unreportedError{message: err.Error()}
}

// Diagnostic explains a native client's warning or error output, which leaves
// verification unqualified, and keeps that output as the error's report.
func Diagnostic(label string, diagnostic []byte) error {
	failure := reportedError{text: withoutControl(domain.Redact(ansi.Strip(string(diagnostic))))}
	lower := strings.ToLower(string(diagnostic))
	keys := deprecatedKeys(diagnostic)
	switch {
	case strings.Contains(lower, "model metadata") && strings.Contains(lower, "fallback metadata"):
		failure.message = fmt.Sprintf("%s emitted a model-metadata warning; native metadata is incomplete and verification is unqualified", label)
		failure.remedy = fmt.Sprintf("Select a model that %s supports, or update %s.", label, label)
	case len(keys) > 0:
		named := strings.Join(keys, ", ")
		failure.message = fmt.Sprintf("%s reports the deprecated configuration key %s; verification is incomplete", label, named)
		failure.remedy = fmt.Sprintf("Remove %s from %s's configuration, or replace it as %s advises.", named, label, label)
	default:
		failure.message = fmt.Sprintf("%s emitted a native-client warning or error; verification is incomplete", label)
	}
	return failure
}

// deprecatedKeys returns the distinct configuration keys that diagnostic
// calls deprecated, in the order in which it names them.
func deprecatedKeys(diagnostic []byte) []string {
	var keys []string
	for _, match := range deprecatedKey.FindAllSubmatch(diagnostic, -1) {
		if key := string(match[1]); len(key) <= deprecatedKeyLength && !slices.Contains(keys, key) && len(keys) < deprecatedKeyLimit {
			keys = append(keys, key)
		}
	}
	return keys
}

// withoutControl drops the format characters of text, such as a direction
// override, which change how a terminal shows the characters around them.
func withoutControl(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, text)
}

// clientReport is a client's output for a reader: its lines that carry a
// warning marker, or its first lines when none does, each on one line of at
// most reportLineRunes characters, without the caller's secrets and with the
// home directory written as ~.
func clientReport(text string, secrets ...string) []string {
	home, _ := os.UserHomeDir()
	var marked, plain []string
	for line := range strings.SplitSeq(domain.Redact(text, secrets...), "\n") {
		line = strings.Join(strings.FieldsFunc(line, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }), " ")
		line = shortenHome(line, home)
		// A marker can stand after the bound, so the whole line is classified.
		warns := process.DiagnosticFailure([]byte(line))
		if utf8.RuneCountInString(line) > reportLineRunes {
			line = string([]rune(line)[:reportLineRunes-1]) + "…"
		}
		switch {
		case line == "" || slices.Contains(marked, line) || slices.Contains(plain, line):
		case warns:
			marked = append(marked, line)
		default:
			plain = append(plain, line)
		}
	}
	report := marked
	if len(report) == 0 {
		report = plain
	}
	return report[:min(len(report), reportLines)]
}

// shortenHome writes home as ~ where it stands as a whole path prefix: not
// inside a longer name, such as the home of a user whose name it begins.
func shortenHome(line, home string) string {
	if len(home) < 2 {
		return line
	}
	named := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._-~", r) }
	var shortened strings.Builder
	for {
		at := strings.Index(line, home)
		if at < 0 {
			break
		}
		before, _ := utf8.DecodeLastRuneInString(line[:at])
		after, _ := utf8.DecodeRuneInString(line[at+len(home):])
		shortened.WriteString(line[:at])
		if (at > 0 && named(before)) || (at+len(home) < len(line) && named(after)) {
			shortened.WriteString(home)
		} else {
			shortened.WriteString("~")
		}
		line = line[at+len(home):]
	}
	shortened.WriteString(line)
	return shortened.String()
}

type requestFailureError struct {
	message string
	cause   error
}

func (failure requestFailureError) Error() string { return failure.message }

func (failure requestFailureError) Unwrap() error { return failure.cause }

// RequestFailure is a verification request failure with a complete message;
// [errors.Is] still finds its cause.
func RequestFailure(message string, cause error) error {
	return requestFailureError{message: message, cause: cause}
}

// Advice is a client's own explanation of one diagnostic: a failed request
// whose lowercased, redacted diagnostic contains every marker is summarized as
// Summary.
type Advice struct {
	Markers []string
	Summary string
}

// Failure is the verification request failure that names its retry command.
// The client's advice explains its known diagnostics after a timeout and
// before the shared model and authentication summaries.
func Failure(label, client, explicitModel string, diagnostic []byte, cause error, advice []Advice, secrets ...string) error {
	detail := verificationFailureSummary(diagnostic, cause, advice, secrets...)
	return requestFailureError{
		message: fmt.Sprintf("%s minimal verification request failed: %s; run `%s`", label, detail, Command(client, explicitModel)),
		cause:   cause,
	}
}

// Command returns the check that runs one client's session again. A model
// that the check named, as provider/model, stays in it: a session of a named
// model takes its own isolated path, so a retry without --model would check
// another model.
func Command(client, explicitModel string) string {
	command := "aigw check " + client
	if explicitModel != "" {
		command += " --model " + explicitModel
	}
	return command + " --depth client"
}

func verificationFailureSummary(diagnostic []byte, cause error, advice []Advice, secrets ...string) string {
	text := strings.ToLower(domain.Redact(string(diagnostic), secrets...))
	if errors.Is(cause, context.DeadlineExceeded) || strings.Contains(text, "deadline exceeded") || strings.Contains(text, "timed out") {
		return "client verification timed out"
	}
	for _, item := range advice {
		if !slices.ContainsFunc(item.Markers, func(marker string) bool { return !strings.Contains(text, marker) }) {
			return item.Summary
		}
	}
	switch {
	case strings.Contains(text, "unrecognized_model"),
		strings.Contains(text, "not support for model"),
		strings.Contains(text, "model id") && strings.Contains(text, "incorrect"),
		strings.Contains(text, "model") && strings.Contains(text, "unavailable"),
		strings.Contains(text, "model") && strings.Contains(text, "status 503"):
		return "selected model is unavailable through the client or endpoint"
	case strings.Contains(text, "provider auth command"),
		strings.Contains(text, "unauthorized"),
		strings.Contains(text, "authentication"),
		strings.Contains(text, "status 401"):
		return "client or endpoint rejected authentication"
	default:
		return "client process rejected the request; diagnostics suppressed"
	}
}
