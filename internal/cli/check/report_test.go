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
// and, sometimes, what to change, as the adapters' diagnostics do. It records
// the secrets that its reader names.
type stubReportError struct {
	lines   []string
	remedy  string
	secrets []string
}

func (*stubReportError) Error() string { return "Codex emitted a native-client warning or error" }

func (failure *stubReportError) ClientReport(secrets ...string) []string {
	failure.secrets = secrets
	return failure.lines
}

func (failure *stubReportError) Remedy() string { return failure.remedy }

// A client's own report reaches the check through the error that the adapter
// returns, however it is wrapped, and the check names the Tokens that it must
// not show. The fix is never the same check again, which would fail the same
// way: it is what to change, when the verification names it; how to read the
// report; or, once the output shows the report, to resolve it.
func TestSessionFailureNamesAFixThatIsNotTheSameCheck(t *testing.T) {
	spec := domain.ClientSpec{ID: codex.ID, Label: "Codex"}
	lines := []string{"first", "second"}
	for _, test := range []struct {
		name    string
		failure *stubReportError
		verbose bool
		fix     string
	}{
		{name: "named remedy", failure: &stubReportError{lines: lines, remedy: "Remove a.b from Codex's configuration."}, fix: "Remove a.b from Codex's configuration."},
		{name: "named remedy, shown", failure: &stubReportError{lines: lines, remedy: "Remove a.b from Codex's configuration."}, verbose: true, fix: "Remove a.b from Codex's configuration."},
		{name: "report not shown", failure: &stubReportError{lines: lines}, fix: "Run the same check with --verbose to read what Codex reported."},
		{name: "report shown", failure: &stubReportError{lines: lines}, verbose: true, fix: "Resolve what Codex reported, then run the same check again."},
		{name: "empty report", failure: &stubReportError{}, verbose: true, fix: "Run Codex itself with the selected model to read its warning."},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := checker{named: named{token: "one-time-token"}, verbose: test.verbose}
			err := fmt.Errorf("verify: %w", errors.Join(test.failure, errors.New("cleanup")))
			got := c.sessionFailure(result{spec: spec, token: "stored-token"}, err)
			if got.depth != depthClient || !slices.Equal(got.report, test.failure.lines) || got.fix != test.fix || !errors.Is(got.cause, err) {
				t.Fatalf("failure = %+v", got)
			}
			if !slices.Equal(test.failure.secrets, []string{"one-time-token", "stored-token"}) {
				t.Fatalf("the report was read with the secrets %q", test.failure.secrets)
			}
			if strings.HasPrefix(got.fix, "aigw ") {
				t.Fatalf("the fix %q is a command that fails the same way", got.fix)
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

// Only --verbose shows a client's own lines: in the text between the problem
// and its fix, and in the document as the failure's report. The fix is prose,
// so the document carries it as next_detail.
func TestRowShowsAClientsReportOnlyWhenAsked(t *testing.T) {
	const fix = "Run the same check with --verbose to read what Codex reported."
	stopped := result{spec: domain.ClientSpec{ID: codex.ID, Label: "Codex"}, model: "one/gpt-test", reached: depthModel,
		failure: &failure{depth: depthClient, problem: "Codex emitted a native-client warning or error", fix: fix, report: []string{"first line", "second line"}}}
	quiet := strings.Join(stopped.row(false).Details, "\n")
	if strings.Contains(quiet, "first line") || !strings.Contains(quiet, "Fix: "+fix) {
		t.Fatalf("row without --verbose = %q", quiet)
	}
	want := []string{"Codex emitted a native-client warning or error", "Codex reported: first line", "Codex reported: second line", "Fix: " + fix}
	if got := stopped.row(true).Details; !slices.Equal(got, want) {
		t.Fatalf("row with --verbose = %q", got)
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
