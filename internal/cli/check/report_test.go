package check

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/domain"
)

// reportedFailure is a verification error that carries a client's own lines,
// as the adapters' diagnostics do.
type reportedFailure struct{ lines []string }

func (*reportedFailure) Error() string { return "Codex emitted a native-client warning or error" }

func (failure *reportedFailure) ClientReport() []string { return failure.lines }

// A client's own report reaches the check through the error that the adapter
// returns, however it is wrapped. Its fix is the same check with --verbose,
// which keeps a named model; an error without a report keeps its own fix.
func TestSessionFailureNamesTheVerboseCheckForAClientsReport(t *testing.T) {
	spec := domain.ClientSpec{ID: codex.ID, Label: "Codex"}
	reported := fmt.Errorf("verify: %w", errors.Join(&reportedFailure{lines: []string{"first", "second"}}, errors.New("cleanup")))
	for _, test := range []struct {
		name   string
		err    error
		named  named
		report int
		fix    string
	}{
		{name: "selected model", err: reported, report: 2, fix: "aigw check codex --depth client --verbose"},
		{name: "named model", err: reported, named: named{ref: domain.NewModelRef("one", "gpt-test")}, report: 2, fix: "aigw check codex --model one/gpt-test --depth client --verbose"},
		{name: "model that needs quoting", err: reported, named: named{ref: domain.NewModelRef("one", "gpt test")}, report: 2, fix: "Run the same check again with --depth client and --verbose."},
		{name: "empty report", err: &reportedFailure{}, fix: "aigw check"},
		{name: "no report", err: errors.New("Codex final response did not return the private verification challenge"), fix: "aigw check"},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := checker{named: test.named}
			got := c.sessionFailure(result{spec: spec}, test.err)
			if got.depth != depthClient || len(got.report) != test.report || got.fix != test.fix || !errors.Is(got.cause, test.err) {
				t.Fatalf("failure = %+v", got)
			}
		})
	}
}

// Only --verbose shows a client's own lines: in the text between the problem
// and its fix, and in the document as the failure's report.
func TestRowShowsAClientsReportOnlyWhenAsked(t *testing.T) {
	stopped := result{spec: domain.ClientSpec{ID: codex.ID, Label: "Codex"}, model: "one/gpt-test", reached: depthModel,
		failure: &failure{depth: depthClient, problem: "Codex emitted a native-client warning or error", fix: "aigw check codex --depth client --verbose", report: []string{"first line", "second line"}}}
	quiet := strings.Join(stopped.row(false).Details, "\n")
	if strings.Contains(quiet, "first line") || !strings.Contains(quiet, "Fix: aigw check codex --depth client --verbose") {
		t.Fatalf("row without --verbose = %q", quiet)
	}
	want := []string{"Codex emitted a native-client warning or error", "Codex reported: first line", "Codex reported: second line", "Fix: aigw check codex --depth client --verbose"}
	if got := stopped.row(true).Details; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("row with --verbose = %q", got)
	}
	if doc := stopped.documented(false); doc.Failure == nil || doc.Failure.ClientReport != nil || doc.NextAction != "aigw check codex --depth client --verbose" {
		t.Fatalf("document without --verbose = %+v", doc.Failure)
	}
	if doc := stopped.documented(true); strings.Join(doc.Failure.ClientReport, "\n") != "first line\nsecond line" {
		t.Fatalf("document with --verbose = %+v", doc.Failure)
	}
	passed := result{spec: stopped.spec, model: stopped.model, reached: depthClient}
	if got := strings.Join(passed.row(true).Details, "\n"); strings.Contains(got, "reported") {
		t.Fatalf("a passed client shows a report: %q", got)
	}
}
