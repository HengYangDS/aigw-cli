package verification

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestProtocolTimeoutAllowsColdClientStartup(t *testing.T) {
	if ProtocolTimeout < time.Minute {
		t.Fatalf("ProtocolTimeout = %s, want at least %s", ProtocolTimeout, time.Minute)
	}
}

// A client's advice explains its own diagnostics; another client's failure
// with the same diagnostic gets only the shared summary, and a timeout still
// wins over the advice.
func TestFailureAppliesOnlyTheCallersAdvice(t *testing.T) {
	cause := errors.New("launch /Users/operator/private/client: exit status 1")
	diagnostic := []byte(`{"result":"API Error: 400 Tier_Gate: extra inputs are not permitted; token=must-not-leak; /Users/operator/private"}`)
	advice := []Advice{{Markers: []string{"tier_gate", "extra inputs are not permitted"}, Summary: "endpoint rejects the tier gate; disable it and verify again"}}
	if err := Failure("Other", "other", "", diagnostic, cause, nil); strings.Contains(err.Error(), "tier gate") {
		t.Fatalf("a failure without advice applied another client's advice: %v", err)
	}
	err := Failure("Client", "client", "", diagnostic, cause, advice, "must-not-leak")
	if !strings.Contains(err.Error(), "Client minimal verification request failed: endpoint rejects the tier gate; disable it and verify again; run `aigw check client --depth client`") || !errors.Is(err, cause) {
		t.Fatalf("the caller's advice or cause was lost: %v", err)
	}
	if err := Failure("Client", "client", "", diagnostic, context.DeadlineExceeded, advice); !strings.Contains(err.Error(), "client verification timed out") {
		t.Fatalf("advice outranked a timeout: %v", err)
	}
}

// A client's own warning leaves verification unqualified. AIGW's message is
// its classification and carries none of the client's sentences; the client's
// lines are the report, which only a person who asks reads.
func TestDiagnosticKeepsTheClientsTextOutOfItsMessage(t *testing.T) {
	const private = "token=must-not-leak /Users/operator/private"
	for name, test := range map[string]struct{ diagnostic, want string }{
		"general":    {"warning: capability failed " + private, "Client emitted a native-client warning or error; verification is incomplete; read the client's report with --verbose"},
		"metadata":   {"Model metadata for grok-4.7 not found. Defaulting to fallback metadata; " + private, "Client emitted a model-metadata warning"},
		"deprecated": {"`[features].multi_agent` is deprecated. It still controls the V1 fallback. " + private, "Client reports the deprecated configuration key [features].multi_agent; verification is incomplete"},
		"two keys":   {"`a.b` is deprecated.\n`[c].d-e` is deprecated; `a.b` is deprecated. " + private, "deprecated configuration key a.b, [c].d-e;"},
		"not a key":  {"`rm -rf $HOME; echo` is deprecated " + private, "Client emitted a native-client warning or error"},
	} {
		t.Run(name, func(t *testing.T) {
			err := Diagnostic("Client", []byte(test.diagnostic))
			if !strings.Contains(err.Error(), test.want) || strings.Contains(err.Error(), "must-not-leak") || strings.Contains(err.Error(), "/Users/operator") || strings.Contains(err.Error(), "fallback.") {
				t.Fatalf("message = %q, want %q and none of the client's text", err, test.want)
			}
		})
	}
}

// The report is the client's own lines, for a person: marked lines first, at
// most four distinct ones of at most 300 characters, without control
// sequences, with credentials redacted and the home directory shortened.
func TestClientReportIsBoundedAndRedacted(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("é", 400)
	diagnostic := strings.Join([]string{
		"session_id: 1", "progress one", "progress two", "progress three",
		"\x1b[33mwarning:\x1b[0m api_key=must-not-leak read " + home + "/.config/client.toml\x07",
		"warning: api_key=must-not-leak read " + home + "/.config/client.toml",
		"error: " + long,
		"Authorization: Bearer must-not-leak-either",
		"error: third", "error: fourth", "error: fifth",
	}, "\n")
	reporter, ok := errors.AsType[reportedError](fmt.Errorf("wrapped: %w", Diagnostic("Client", []byte(diagnostic))))
	if !ok {
		t.Fatal("a wrapped diagnostic carries no report")
	}
	report := reporter.ClientReport()
	if len(report) != 4 || report[0] != "warning: api_key=[REDACTED] read ~/.config/client.toml" || report[2] != "error: third" || report[3] != "error: fourth" {
		t.Fatalf("report = %q", report)
	}
	if got := utf8.RuneCountInString(report[1]); got != 301 || !strings.HasSuffix(report[1], "…") {
		t.Fatalf("a long line has %d characters: %q", got, report[1][:40])
	}
	if joined := strings.Join(report, "\n"); strings.Contains(joined, "must-not-leak") || strings.Contains(joined, home) || strings.ContainsAny(joined, "\x1b\x07") {
		t.Fatalf("report leaks: %q", joined)
	}
	// Without a marked line, the first lines are the report: a Codex error item is one.
	item := Diagnostic("Codex", []byte("`[features].multi_agent` is deprecated. It still controls the V1 fallback.\n\n  second  line  "))
	if got := item.(interface{ ClientReport() []string }).ClientReport(); len(got) != 2 || got[0] != "`[features].multi_agent` is deprecated. It still controls the V1 fallback." || got[1] != "second line" {
		t.Fatalf("unmarked report = %q", got)
	}
	// A caller cannot change the error's report.
	reporter.ClientReport()[0] = "changed"
	if reporter.ClientReport()[0] == "changed" {
		t.Fatal("the report is shared with its readers")
	}
	if got := Reported("message", nil).(interface{ ClientReport() []string }).ClientReport(); len(got) != 0 {
		t.Fatalf("an empty diagnostic reports %q", got)
	}
}
