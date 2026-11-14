// Package verification holds what every client's live model probe shares: the
// turn deadline, bounded response reads, failure messages and the retry command.
package verification

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// deprecatedKey is a configuration key that a client calls deprecated, in the
// form in which Codex names one: in backticks before the words, a table in
// brackets and at least one name after it, in lowercase names that start with
// a letter. A message is AIGW's own text, so the grammar admits no sentence,
// and no path without a table: a provider's error can quote one of its own
// request fields that way.
var deprecatedKey = regexp.MustCompile("`(\\[[a-z][a-z0-9_-]{0,31}(?:\\.[a-z][a-z0-9_-]{0,31})*\\](?:\\.[a-z][a-z0-9_-]{0,31})+)` is deprecated")

// reportedError is a verification that a native client's own warning or error
// leaves unqualified. Its message and remedy are AIGW's and carry none of the
// client's sentences; its report is the client's own lines, for a reader who
// holds the session's credential.
type reportedError struct {
	message string
	remedy  string
	// output is the client's output without terminal sequences. It is not
	// redacted: only ClientReport reads it, and redacts what it returns. It
	// stands behind a pointer, so that no verb prints it, also not %#v of an
	// error that wraps this one or of a value that holds it.
	output *string
}

func (failure reportedError) Error() string { return failure.message }

// Remedy says what to change, when AIGW can name it without the client's
// sentences, or nothing.
func (failure reportedError) Remedy() string { return failure.remedy }

// ClientReport returns the client's own lines for a reader: the lines that
// carry a warning marker, or the first lines when none does, each on one line
// of at most reportLineRunes characters, without credential and with the home
// directory written as ~. credential is the session's credential, which the
// client can print in a form that no redaction rule describes; without it
// there is no report.
func (failure reportedError) ClientReport(credential string) []string {
	if failure.output == nil || strings.TrimSpace(credential) == "" {
		return nil
	}
	home, _ := os.UserHomeDir()
	var marked, plain []string
	for line := range strings.FieldsFuncSeq(*failure.output, func(r rune) bool { return r == '\n' || r == '\r' }) {
		line = withoutControl(line)
		// The line is classified before its redaction, which can take its marker.
		lines := &plain
		if process.DiagnosticFailure([]byte(line)) {
			lines = &marked
		}
		// One redaction, of one line: a second one would miss a credential
		// that the first had cut, and a rule would otherwise reach the next line.
		line = strings.Join(strings.Fields(domain.Redact(line, credential)), " ")
		line = shortenHome(line, home)
		if utf8.RuneCountInString(line) > reportLineRunes {
			line = string([]rune(line)[:reportLineRunes-1]) + "…"
		}
		if line != "" && !slices.Contains(*lines, line) {
			*lines = append(*lines, line)
		}
	}
	if len(marked) == 0 {
		marked = plain
	}
	return marked[:min(len(marked), reportLines)]
}

// Diagnostic explains a native client's warning or error output, which leaves
// verification unqualified, and keeps that output as the error's report.
func Diagnostic(label string, diagnostic []byte) error {
	output := ansi.Strip(string(diagnostic))
	failure := reportedError{output: &output}
	lower := strings.ToLower(output)
	keys := deprecatedKeys(output)
	switch {
	case strings.Contains(lower, "model metadata") && strings.Contains(lower, "fallback metadata"):
		failure.message = fmt.Sprintf("%s emitted a model-metadata warning; native metadata is incomplete and verification is unqualified", label)
		failure.remedy = fmt.Sprintf("Select a model that %s supports, or update %s.", label, label)
	case len(keys) > 0:
		noun, named := "key", strings.Join(keys, ", ")
		if len(keys) > 1 {
			noun = "keys"
		}
		failure.message = fmt.Sprintf("%s reports the deprecated configuration %s %s; verification is incomplete", label, noun, named)
		failure.remedy = fmt.Sprintf("Remove %s from %s's configuration, or replace it as %s advises.", named, label, label)
	default:
		failure.message = fmt.Sprintf("%s emitted a native-client warning or error; verification is incomplete", label)
	}
	return failure
}

// deprecatedKeys returns the distinct configuration keys that output calls
// deprecated, in the order in which it names them.
func deprecatedKeys(output string) []string {
	var keys []string
	for _, match := range deprecatedKey.FindAllStringSubmatch(output, -1) {
		if key := match[1]; len(key) <= deprecatedKeyLength && !slices.Contains(keys, key) && len(keys) < deprecatedKeyLimit {
			keys = append(keys, key)
		}
	}
	return keys
}

// withoutControl drops the control and format characters of line, such as a
// bell or a direction override: they change how a terminal shows the
// characters around them, and one inside a credential would hide it from
// redaction. A tab stays, as the space that it is.
func withoutControl(line string) string {
	return strings.Map(func(r rune) rune {
		if r != '\t' && (unicode.IsControl(r) || unicode.Is(unicode.Cf, r)) {
			return -1
		}
		return r
	}, line)
}

// shortenHome writes home as ~ where it stands as a whole path prefix: not
// inside a longer name, such as the home of a user whose name it begins.
func shortenHome(line, home string) string {
	home = filepath.Clean(home)
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
		shortened.WriteString(line[:at])
		before, _ := utf8.DecodeLastRuneInString(shortened.String())
		after, _ := utf8.DecodeRuneInString(line[at+len(home):])
		if (shortened.Len() > 0 && named(before)) || (at+len(home) < len(line) && named(after)) {
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
func Failure(label, client, explicitModel string, diagnostic []byte, cause error, advice []Advice) error {
	detail := verificationFailureSummary(diagnostic, cause, advice)
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

// verificationFailureSummary is one of AIGW's fixed sentences for a failed
// request. It reads its markers in the client's own text: a redaction takes
// the rest of a line after a credential header, and a marker with it.
func verificationFailureSummary(diagnostic []byte, cause error, advice []Advice) string {
	text := strings.ToLower(string(diagnostic))
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
