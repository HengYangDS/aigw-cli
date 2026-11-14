package verification

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
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

// A client's own warning leaves verification unqualified. AIGW's message and
// remedy are its classification and carry none of the client's sentences. A
// configuration key that the client calls deprecated is named only as a path
// of lowercase names; whatever else stands in backticks can be a value.
func TestDiagnosticKeepsTheClientsTextOutOfItsMessage(t *testing.T) {
	const (
		private = "token=must-not-leak /Users/operator/private"
		general = "Client emitted a native-client warning or error; verification is incomplete"
	)
	name32 := "a" + strings.Repeat("b", 31)
	for name, test := range map[string]struct{ diagnostic, message, remedy string }{
		"general":  {"warning: capability failed " + private, general, ""},
		"metadata": {"Model metadata for grok-4.7 not found. Defaulting to fallback metadata; " + private, "Client emitted a model-metadata warning; native metadata is incomplete and verification is unqualified", "Select a model that Client supports, or update Client."},
		"deprecated": {"`[features].multi_agent` is deprecated. It still controls the V1 fallback. " + private,
			"Client reports the deprecated configuration key [features].multi_agent; verification is incomplete",
			"Remove [features].multi_agent from Client's configuration, or replace it as Client advises."},
		"two keys": {"`a.b` is deprecated.\n`[c.d].e-f` is deprecated; `a.b` is deprecated. " + private,
			"Client reports the deprecated configuration key a.b, [c.d].e-f; verification is incomplete",
			"Remove a.b, [c.d].e-f from Client's configuration, or replace it as Client advises."},
		"four keys": {"`a.a` is deprecated `b.b` is deprecated `c.c` is deprecated `d.d` is deprecated",
			"Client reports the deprecated configuration key a.a, b.b, c.c; verification is incomplete",
			"Remove a.a, b.b, c.c from Client's configuration, or replace it as Client advises."},
		"longest path": {"`" + name32[:31] + "." + name32 + "` is deprecated",
			"Client reports the deprecated configuration key " + name32[:31] + "." + name32 + "; verification is incomplete",
			"Remove " + name32[:31] + "." + name32 + " from Client's configuration, or replace it as Client advises."},
		// Not a key path: each can be a value that the client quotes.
		"sentence":   {"`rm -rf $HOME; echo` is deprecated " + private, general, ""},
		"one word":   {"API key `sk-live-0123456789abcdef` is deprecated " + private, general, ""},
		"capital":    {"`AKIAIOSFODNN7EXAMPLE.wJalrXUtnFEMI` is deprecated", general, ""},
		"digit name": {"`a.0123456789abcdef0123456789abcdef` is deprecated", general, ""},
		"long name":  {"`a." + name32 + "b` is deprecated", general, ""},
		"long path":  {"`" + name32 + "." + name32 + "` is deprecated", general, ""},
		"other case": {"`a.b` IS DEPRECATED", general, ""},
	} {
		t.Run(name, func(t *testing.T) {
			failure, reported := errors.AsType[reportedError](Diagnostic("Client", []byte(test.diagnostic)))
			if !reported || failure.Error() != test.message || failure.Remedy() != test.remedy {
				t.Fatalf("message = %q, remedy = %q; want %q, %q", failure.Error(), failure.Remedy(), test.message, test.remedy)
			}
		})
	}
}

// reportOf returns the client's report that err carries.
func reportOf(t *testing.T, err error, secrets ...string) []string {
	t.Helper()
	failure, reported := errors.AsType[reportedError](err)
	if !reported {
		t.Fatalf("%v carries no client report", err)
	}
	return failure.ClientReport(secrets...)
}

// The report is the client's own lines, for a reader who asks: marked lines
// first, at most four distinct ones of at most 300 characters, without
// control sequences or format characters, without the known credential forms
// and the reader's secrets, and with the home directory shortened.
func TestClientReportIsBoundedAndRedacted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	long := "error: " + strings.Repeat("é", 400)
	diagnostic := strings.Join([]string{
		"session_id: 1", "progress one", "progress two", "progress three",
		"\x1b[33mwarning:\x1b[0m api_key=must-not-leak read " + home + "/.config/client.toml\x07",
		"warning: api_key=must-not-leak   read " + home + "/.config/client.toml",
		long, long,
		"error: 401 Authorization: Bearer must-not-leak-either for opaque-token-0001",
		"error: \u202edesrever\u200b fourth", "error: fifth",
	}, "\n")
	err := fmt.Errorf("wrapped: %w", Diagnostic("Client", []byte(diagnostic)))
	report := reportOf(t, err, "", "opaque-token-0001")
	want := []string{
		"warning: api_key=[REDACTED] read ~/.config/client.toml",
		"error: " + strings.Repeat("é", 292) + "…",
		"error: 401 Authorization: [REDACTED] for [REDACTED]",
		"error: desrever fourth",
	}
	if !slices.Equal(report, want) {
		t.Fatalf("report = %q, want %q", report, want)
	}
	if got := utf8.RuneCountInString(report[1]); got != 300 {
		t.Fatalf("a long line has %d characters, want 300", got)
	}
	if strings.Contains(err.Error(), "api_key") || strings.Contains(err.Error(), "must-not-leak") || strings.Contains(err.Error(), "desrever") {
		t.Fatalf("the message carries the report: %v", err)
	}
	// Without the reader's secret, a bare value that no form describes stays.
	if got := reportOf(t, err)[2]; got != "error: 401 Authorization: [REDACTED] for opaque-token-0001" {
		t.Fatalf("report without the reader's secret = %q", got)
	}
	// A reader cannot change the error's report.
	reportOf(t, err)[0] = "changed"
	if reportOf(t, err)[0] == "changed" {
		t.Fatal("the report is shared with its readers")
	}
}

// Without a marked line the first lines are the report, as for a Codex error
// item; a marker that stands after the bound still marks its line; and an
// empty output reports nothing.
func TestClientReportSelectsMarkedLinesOrTheFirstOnes(t *testing.T) {
	item := Diagnostic("Codex", []byte("`[features].multi_agent` is deprecated. It still controls the V1 fallback.\n\n  second  line  \nthird\nfourth\nfifth"))
	if got := reportOf(t, item); !slices.Equal(got, []string{"`[features].multi_agent` is deprecated. It still controls the V1 fallback.", "second line", "third", "fourth"}) {
		t.Fatalf("unmarked report = %q", got)
	}
	late := Diagnostic("Client", []byte("one\ntwo\nthree\nfour\n"+strings.Repeat("x", 310)+" error: late"))
	if got := reportOf(t, late); len(got) != 1 || got[0] != strings.Repeat("x", 299)+"…" {
		t.Fatalf("a marker after the bound: report = %q", got)
	}
	if got := reportOf(t, Diagnostic("Client", nil)); len(got) != 0 {
		t.Fatalf("an empty output reports %q", got)
	}
}

// The home directory is shortened only where it stands as a whole path
// prefix: not as the beginning of another user's home, and not inside a
// longer path.
func TestClientReportShortensOnlyTheHomeItself(t *testing.T) {
	home := filepath.Join(t.TempDir(), "operator")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	text := "error: read " + home + "2/file, then " + home + ", then /private" + home + "/file, then " + home + "/file"
	want := "error: read " + home + "2/file, then ~, then /private" + home + "/file, then ~/file"
	if got := reportOf(t, Diagnostic("Client", []byte(text))); len(got) != 1 || got[0] != want {
		t.Fatalf("report = %q, want %q", got, want)
	}
}

// A runner that must not pass a client's output on withholds the report: the
// message stays, and no reader finds the report behind the error.
func TestWithoutReportWithholdsTheClientsReport(t *testing.T) {
	reported := fmt.Errorf("verify: %w", Diagnostic("Client", []byte("warning: helper printed token=must-not-leak")))
	withheld := WithoutReport(reported)
	if _, found := errors.AsType[reportedError](withheld); found || withheld.Error() != reported.Error() {
		t.Fatalf("withheld error = %v, report found = %t", withheld, found)
	}
	other := errors.New("another failure")
	if got := WithoutReport(other); !errors.Is(got, other) {
		t.Fatalf("another error changed: %v", got)
	}
	if WithoutReport(nil) != nil {
		t.Fatal("no error became one")
	}
}
