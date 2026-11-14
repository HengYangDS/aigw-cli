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
// backticks before the words, as a path of short names that starts with a
// letter or a table in brackets. A message is AIGW's own text, so the grammar
// admits no sentence, and a path needs a dot: a single word in backticks can
// be a value, such as a credential, that the client quotes.
var deprecatedKey = regexp.MustCompile("`((?:\\[[A-Za-z][A-Za-z0-9_-]{0,31}(?:\\.[A-Za-z0-9_-]{1,32})*\\]|[A-Za-z][A-Za-z0-9_-]{0,31})(?:\\.[A-Za-z0-9_-]{1,32})+)` is deprecated")

// reportedError is a verification that a native client's own warning or error
// leaves unqualified. Its message is AIGW's and carries none of the client's
// sentences; its report is the client's own lines, for a person who asks.
type reportedError struct {
	message string
	report  []string
}

func (failure reportedError) Error() string { return failure.message }

// ClientReport returns the client's own lines, redacted and bounded.
func (failure reportedError) ClientReport() []string { return slices.Clone(failure.report) }

// Reported is the unqualified verification that message describes in AIGW's
// own words, with diagnostic, the client's output, as its report.
func Reported(message string, diagnostic []byte) error {
	return reportedError{message: message, report: clientReport(diagnostic)}
}

// Diagnostic explains a native client's warning or error output, which leaves
// verification unqualified, and keeps that output as the error's report.
func Diagnostic(label string, diagnostic []byte) error {
	text := strings.ToLower(string(diagnostic))
	if strings.Contains(text, "model metadata") && strings.Contains(text, "fallback metadata") {
		return Reported(fmt.Sprintf("%s emitted a model-metadata warning; native metadata is incomplete and verification is unqualified; select a model supported by the client or update the client", label), diagnostic)
	}
	if keys := deprecatedKeys(diagnostic); len(keys) > 0 {
		return Reported(fmt.Sprintf("%s reports the deprecated configuration key %s; verification is incomplete; remove it from the client's configuration or replace it as the client advises", label, strings.Join(keys, ", ")), diagnostic)
	}
	return Reported(fmt.Sprintf("%s emitted a native-client warning or error; verification is incomplete; read the client's report with --verbose", label), diagnostic)
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

// clientReport is diagnostic for a person: its lines that carry a warning
// marker, or its first lines when none does, without control sequences, with
// credentials redacted and the home directory written as ~.
func clientReport(diagnostic []byte) []string {
	home, _ := os.UserHomeDir()
	var marked, plain []string
	for line := range strings.SplitSeq(domain.Redact(ansi.Strip(string(diagnostic))), "\n") {
		line = strings.Join(strings.FieldsFunc(line, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }), " ")
		if len(home) > 1 {
			line = strings.ReplaceAll(line, home, "~")
		}
		if line == "" || slices.Contains(marked, line) || slices.Contains(plain, line) {
			continue
		}
		if utf8.RuneCountInString(line) > reportLineRunes {
			line = string([]rune(line)[:reportLineRunes]) + "…"
		}
		if process.DiagnosticFailure([]byte(line)) {
			marked = append(marked, line)
		} else {
			plain = append(plain, line)
		}
	}
	report := marked
	if len(report) == 0 {
		report = plain
	}
	return report[:min(len(report), reportLines)]
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
