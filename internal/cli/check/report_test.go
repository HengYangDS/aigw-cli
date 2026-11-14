package check

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/domain"
)

// stubReportError is a verification error that carries a client's own lines
// and, sometimes, what to change, as the adapters' diagnostics do. Like them
// it gives its lines only to a reader who names a credential, which it
// records.
type stubReportError struct {
	lines      []string
	remedy     string
	credential string
}

func (*stubReportError) Error() string { return "Codex emitted a native-client warning or error" }

func (failure *stubReportError) ClientReport(credential string) []string {
	failure.credential = credential
	if credential == "" {
		return nil
	}
	return failure.lines
}

func (failure *stubReportError) Remedy() string { return failure.remedy }

// A client's own report reaches the check through the error that the adapter
// returns, however it is wrapped. The check reads it with the Token that the
// endpoint depth read, which is the session's credential; a client that
// authenticates itself, or through an external helper, has none there, and
// so no report. The fix is never `aigw check`, which starts no client and
// passes: it is what to change, when the verification names it; how to read
// the report; once the output shows the report, to resolve it; and without a
// report, to run the client itself.
func TestSessionFailureNamesAFixThatIsNotTheSameCheck(t *testing.T) {
	const (
		remedy = "Remove [a].b from Codex's configuration."
		itself = "Run Codex itself with the same model to read its warning."
	)
	spec := domain.ClientSpec{ID: codex.ID, Label: "Codex"}
	lines := []string{"first", "second"}
	for _, test := range []struct {
		name    string
		failure *stubReportError
		token   string
		verbose bool
		report  []string
		fix     string
	}{
		{name: "named remedy", failure: &stubReportError{lines: lines, remedy: remedy}, token: "stored-token", report: lines, fix: remedy},
		{name: "named remedy, shown", failure: &stubReportError{lines: lines, remedy: remedy}, token: "stored-token", verbose: true, report: lines, fix: remedy},
		{name: "report not shown", failure: &stubReportError{lines: lines}, token: "stored-token", report: lines, fix: "Run the same check with --verbose to read what Codex reported."},
		{name: "report shown", failure: &stubReportError{lines: lines}, token: "stored-token", verbose: true, report: lines, fix: "Resolve what Codex reported, then run the same check again."},
		{name: "empty report", failure: &stubReportError{}, token: "stored-token", fix: itself},
		{name: "empty report, asked", failure: &stubReportError{}, token: "stored-token", verbose: true, fix: itself},
		{name: "no credential", failure: &stubReportError{lines: lines}, fix: itself},
		{name: "no credential, asked", failure: &stubReportError{lines: lines}, verbose: true, fix: itself},
		{name: "no credential, named remedy", failure: &stubReportError{lines: lines, remedy: remedy}, verbose: true, fix: remedy},
	} {
		t.Run(test.name, func(t *testing.T) {
			// A Token that the command line names is the session's only once the endpoint depth read it.
			c := checker{named: named{token: "named-token"}, verbose: test.verbose}
			err := fmt.Errorf("verify: %w", errors.Join(test.failure, errors.New("cleanup")))
			got := c.sessionFailure(result{spec: spec, token: test.token}, err)
			if got.depth != depthClient || !slices.Equal(got.report, test.report) || got.fix != test.fix || !errors.Is(got.cause, err) {
				t.Fatalf("failure = %+v", got)
			}
			if test.failure.credential != test.token {
				t.Fatalf("the report was read with the credential %q, want %q", test.failure.credential, test.token)
			}
			if strings.HasPrefix(got.fix, "aigw ") {
				t.Fatalf("the fix %q is a command", got.fix)
			}
		})
	}
	// Another failure of a session keeps its own fix.
	plain := errors.New("Codex final response did not return the private verification challenge")
	var c checker
	if got := c.sessionFailure(result{spec: spec}, plain); got.report != nil || got.fix != "aigw check" {
		t.Fatalf("a failure without a report = %+v", got)
	}
}

// Only --verbose shows a client's own lines: in the text as quotes after
// AIGW's own problem and fix, and in the document as the failure's report.
// The fix is prose, so the document carries it as next_detail.
func TestRowShowsAClientsReportOnlyWhenAsked(t *testing.T) {
	const fix = "Run the same check with --verbose to read what Codex reported."
	stopped := result{spec: domain.ClientSpec{ID: codex.ID, Label: "Codex"}, model: "one/gpt-test", reached: depthModel,
		failure: &failure{depth: depthClient, problem: "Codex emitted a native-client warning or error", fix: fix, report: []string{"first line", "second line"}}}
	quiet := strings.Join(stopped.row(false).Details, "\n")
	if strings.Contains(quiet, "first line") || !strings.Contains(quiet, "Fix: "+fix) {
		t.Fatalf("row without --verbose = %q", quiet)
	}
	if got := stopped.row(false); len(got.Quotes) != 0 {
		t.Fatalf("row without --verbose quotes %q", got.Quotes)
	}
	shown := stopped.row(true)
	if want := []string{"Codex emitted a native-client warning or error", "Fix: " + fix}; !slices.Equal(shown.Details, want) {
		t.Fatalf("row with --verbose = %q", shown.Details)
	}
	if want := []string{"Codex reported: first line", "Codex reported: second line"}; !slices.Equal(shown.Quotes, want) {
		t.Fatalf("row with --verbose quotes %q", shown.Quotes)
	}
	if doc := stopped.documented(false); doc.Failure == nil || doc.Failure.ClientReport != nil || doc.NextAction != "" || doc.NextDetail != fix {
		t.Fatalf("document without --verbose = %+v, next %q, %q", doc.Failure, doc.NextAction, doc.NextDetail)
	}
	if doc := stopped.documented(true); !slices.Equal(doc.Failure.ClientReport, []string{"first line", "second line"}) {
		t.Fatalf("document with --verbose = %+v", doc.Failure)
	}
	passed := result{spec: stopped.spec, model: stopped.model, reached: depthClient}
	if got := strings.Join(passed.row(true).Details, "\n"); strings.Contains(got, "reported") {
		t.Fatalf("a passed client shows a report: %q", got)
	}
}
