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
	err := Failure("Client", "client", "", diagnostic, cause, advice)
	if !strings.Contains(err.Error(), "Client minimal verification request failed: endpoint rejects the tier gate; disable it and verify again; run `aigw check client --depth client`") || !errors.Is(err, cause) {
		t.Fatalf("the caller's advice or cause was lost: %v", err)
	}
	if err := Failure("Client", "client", "", diagnostic, context.DeadlineExceeded, advice); !strings.Contains(err.Error(), "client verification timed out") {
		t.Fatalf("advice outranked a timeout: %v", err)
	}
}

// The summary reads its markers in the client's own text. A redaction would
// take the rest of a line after a header, and the marker with it; the summary
// itself is one of AIGW's fixed sentences and carries none of that text.
func TestFailureReadsItsMarkersBehindACredentialHeader(t *testing.T) {
	cause := errors.New("exit status 1")
	for diagnostic, want := range map[string]string{
		"Authorization: Bearer must-not-leak; request timed out":                         "client verification timed out",
		`error: Authorization: Bearer must-not-leak; body {"code":"unrecognized_model"}`: "selected model is unavailable through the client or endpoint",
		"Proxy-Authorization: Basic must-not-leak model x status 503":                    "selected model is unavailable through the client or endpoint",
		`cookie: session=must-not-leak {"error":"unauthorized"}`:                         "client or endpoint rejected authentication",
		"Authorization: Bearer must-not-leak; nothing that AIGW knows":                   "client process rejected the request; diagnostics suppressed",
	} {
		err := Failure("Client", "client", "", []byte(diagnostic), cause, nil)
		if !strings.Contains(err.Error(), ": "+want+"; run") || strings.Contains(err.Error(), "must-not-leak") {
			t.Errorf("Failure(%q) = %v, want %q", diagnostic, err, want)
		}
	}
}

// A client's own warning leaves verification unqualified. AIGW's message and
// remedy are its classification and carry none of the client's sentences. A
// configuration key that the client calls deprecated is named only in Codex's
// form, a table in brackets and lowercase names; whatever else stands in
// backticks can be a value, or a field of a provider's request.
func TestDiagnosticKeepsTheClientsTextOutOfItsMessage(t *testing.T) {
	const (
		private = "token=must-not-leak /Users/operator/private"
		general = "Client emitted a native-client warning or error; verification is incomplete"
	)
	name32 := "a" + strings.Repeat("b", 31)
	longest := "[" + name32[:29] + "]." + name32
	for name, test := range map[string]struct{ diagnostic, message, remedy string }{
		"general":  {"warning: capability failed " + private, general, ""},
		"metadata": {"Model metadata for grok-4.7 not found. Defaulting to fallback metadata; " + private, "Client emitted a model-metadata warning; native metadata is incomplete and verification is unqualified", "Select a model that Client supports, or update Client."},
		"deprecated": {"`[features].multi_agent` is deprecated. It still controls the V1 fallback. " + private,
			"Client reports the deprecated configuration key [features].multi_agent; verification is incomplete",
			"Remove [features].multi_agent from Client's configuration, or replace it as Client advises."},
		"in colour": {"\x1b[33m`[features].multi_agent`\x1b[0m is deprecated.",
			"Client reports the deprecated configuration key [features].multi_agent; verification is incomplete",
			"Remove [features].multi_agent from Client's configuration, or replace it as Client advises."},
		"two keys": {"`[a].b` is deprecated.\n`[c.d].e-f` is deprecated; `[a].b` is deprecated. " + private,
			"Client reports the deprecated configuration keys [a].b, [c.d].e-f; verification is incomplete",
			"Remove [a].b, [c.d].e-f from Client's configuration, or replace it as Client advises."},
		"four keys": {"`[a].a` is deprecated `[b].b` is deprecated `[c].c` is deprecated `[d].d` is deprecated",
			"Client reports the deprecated configuration keys [a].a, [b].b, [c].c; verification is incomplete",
			"Remove [a].a, [b].b, [c].c from Client's configuration, or replace it as Client advises."},
		"longest path": {"`" + longest + "` is deprecated",
			"Client reports the deprecated configuration key " + longest + "; verification is incomplete",
			"Remove " + longest + " from Client's configuration, or replace it as Client advises."},
		// Not a configuration key: each can be a value that the client quotes.
		"sentence":      {"`rm -rf $HOME; echo` is deprecated " + private, general, ""},
		"one word":      {"API key `sk-live-0123456789abcdef` is deprecated " + private, general, ""},
		"no table":      {"unexpected status 400: `response_format.json_schema` is deprecated", general, ""},
		"table alone":   {"`[features]` is deprecated", general, ""},
		"capital":       {"`[AKIAIOSFODNN7EXAMPLE].wJalrXUtnFEMI` is deprecated", general, ""},
		"digit name":    {"`[a].0123456789abcdef0123456789abcdef` is deprecated", general, ""},
		"long name":     {"`[a]." + name32 + "b` is deprecated", general, ""},
		"long path":     {"`[" + name32 + "]." + name32 + "` is deprecated", general, ""},
		"other case":    {"`[a].b` IS DEPRECATED", general, ""},
		"quoted values": {"token=`[abcd].efgh` was rejected; `[abcd].efgh` is not deprecated", general, ""},
	} {
		t.Run(name, func(t *testing.T) {
			failure, reported := errors.AsType[reportedError](Diagnostic("Client", []byte(test.diagnostic)))
			if !reported || failure.Error() != test.message || failure.Remedy() != test.remedy {
				t.Fatalf("message = %q, remedy = %q; want %q, %q", failure.Error(), failure.Remedy(), test.message, test.remedy)
			}
		})
	}
}

// credential is the session's credential in these tests: a value that no
// redaction rule describes.
const credential = "opaque.credential/0001+x"

// reportOf returns the client's report that err carries, for a reader who
// holds the session's credential.
func reportOf(t *testing.T, err error) []string {
	t.Helper()
	failure, reported := errors.AsType[reportedError](err)
	if !reported {
		t.Fatalf("%v carries no client report", err)
	}
	return failure.ClientReport(credential)
}

// shortHome makes a short directory the home, so that a line which names it
// stays inside the report's bound on every platform.
func shortHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(string(filepath.Separator)+"home", "operator")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

// The report is the client's own lines, for a reader who asks: only the
// marked lines when there are any, at most four distinct ones of at most 300
// characters, without control sequences, control or format characters,
// without the session's credential and the credential forms, and with the
// home directory shortened.
func TestClientReportIsBoundedAndRedacted(t *testing.T) {
	home := shortHome(t)
	long := "error: " + strings.Repeat("é", 400)
	diagnostic := strings.Join([]string{
		"session_id: 1", "progress one", "progress two", "progress three",
		"\x1b[33mwarning:\x1b[0m api_key=must-not-leak read " + home + "/.config/client.toml\x07",
		"warning: api_key=must-not-leak   read\t" + home + "/.config/client.toml",
		long, long,
		"error: 401 for key " + credential + " and sk-live-\x070123456789abcdef",
		"error: \u202edesrever\u200b fourth", "error: fifth",
	}, "\r\n")
	err := fmt.Errorf("wrapped: %w", Diagnostic("Client", []byte(diagnostic)))
	report := reportOf(t, err)
	want := []string{
		"warning: api_key=[REDACTED] read ~/.config/client.toml",
		"error: " + strings.Repeat("é", 292) + "…",
		"error: 401 for key [REDACTED] and [REDACTED]",
		"error: desrever fourth",
	}
	if !slices.Equal(report, want) {
		t.Fatalf("report = %q, want %q", report, want)
	}
	if got := utf8.RuneCountInString(report[1]); got != 300 {
		t.Fatalf("a long line has %d characters, want 300", got)
	}
	// However the error is printed, alone, wrapped, joined or inside a value
	// that holds it, it shows its message and never the client's output.
	holder := struct{ cause error }{cause: err}
	for _, value := range []any{err, errors.Unwrap(err), errors.Join(errors.New("cleanup"), err), holder, &holder} {
		shown := fmt.Sprintf("%v %+v %#v %s", value, value, value, value)
		if strings.Contains(shown, "api_key") || strings.Contains(shown, "must-not-leak") || strings.Contains(shown, "desrever") || strings.Contains(shown, credential) {
			t.Fatalf("the error shows its report: %s", shown)
		}
	}
	// A reader cannot change the error's report.
	reportOf(t, err)[0] = "changed"
	if reportOf(t, err)[0] == "changed" {
		t.Fatal("the report is shared with its readers")
	}
}

// A report is redacted with the session's credential or not given: a client
// can print its credential in a form that no rule describes. A client that
// authenticates itself, or through an external helper, leaves AIGW none.
func TestClientReportNeedsTheSessionsCredential(t *testing.T) {
	failure, _ := errors.AsType[reportedError](Diagnostic("Client", []byte("warning: rejected key "+credential)))
	for _, none := range []string{"", " \t"} {
		if got := failure.ClientReport(none); got != nil {
			t.Fatalf("a report without a credential: %q", got)
		}
	}
	if got := failure.ClientReport(" " + credential + "\n"); !slices.Equal(got, []string{"warning: rejected key [REDACTED]"}) {
		t.Fatalf("report = %q", got)
	}
}

// The credential is cut out whole, although a rule describes a part of it:
// one redaction sees the client's line as the client wrote it.
func TestClientReportRedactsACredentialThatARuleCutsShort(t *testing.T) {
	for name, test := range map[string]struct{ credential, line, want string }{
		"bare key and more": {"sk-live0000000000000000.tail/part+x", "warning: key sk-live0000000000000000.tail/part+x rejected", "warning: key [REDACTED] rejected"},
		"comma in a field":  {"abc123,def456", "warning: token=abc123,def456 rejected", "warning: token=[REDACTED] rejected"},
		"semicolon, bearer": {"abc123;def456", "warning: sent Bearer abc123;def456 twice", "warning: sent Bearer [REDACTED] twice"},
		"url-escaped":       {"a b/c", "warning: GET /v1?k=a+b%2Fc and /v1/a%20b%2Fc", "warning: GET /v1?k=[REDACTED] and /v1/[REDACTED]"},
	} {
		t.Run(name, func(t *testing.T) {
			failure, _ := errors.AsType[reportedError](Diagnostic("Client", []byte(test.line)))
			if got := failure.ClientReport(test.credential); !slices.Equal(got, []string{test.want}) {
				t.Fatalf("report = %q, want %q", got, test.want)
			}
		})
	}
}

// The client's own line decides whether it is a marked one, and a redaction
// stays on its line: a marker that a rule takes still marks the line, and a
// header at the end of a line does not take the next line with it.
func TestClientReportSelectsMarkedLinesOrTheFirstOnes(t *testing.T) {
	item := Diagnostic("Codex", []byte("`[features].multi_agent` is deprecated. It still controls the V1 fallback.\n\n  second  line  \nthird\nfourth\nfifth"))
	if got := reportOf(t, item); !slices.Equal(got, []string{"`[features].multi_agent` is deprecated. It still controls the V1 fallback.", "second line", "third", "fourth"}) {
		t.Fatalf("unmarked report = %q", got)
	}
	late := Diagnostic("Client", []byte("one\ntwo\nthree\nfour\n"+strings.Repeat("x", 310)+" error: late"))
	if got := reportOf(t, late); len(got) != 1 || got[0] != strings.Repeat("x", 299)+"…" {
		t.Fatalf("a marker after the bound: report = %q", got)
	}
	taken := Diagnostic("Client", []byte("one\ntwo\nthree\nfour\nfive token: error\nsix"))
	if got := reportOf(t, taken); !slices.Equal(got, []string{"five token: [REDACTED]"}) {
		t.Fatalf("a marker that a redaction takes: report = %q", got)
	}
	header := Diagnostic("Client", []byte("note Authorization: Bearer\nwarning: capability degraded\nCookie:\nwarning: another"))
	if got := reportOf(t, header); !slices.Equal(got, []string{"warning: capability degraded", "warning: another"}) {
		t.Fatalf("a header that ends its line: report = %q", got)
	}
	// A marked line stays, although an unmarked one begins with the same 299 characters.
	same := strings.Repeat("y", 299)
	twin := Diagnostic("Client", []byte(same+" progress\nother progress\n"+same+" error: late"))
	if got := reportOf(t, twin); !slices.Equal(got, []string{same + "…"}) {
		t.Fatalf("a marked line after its unmarked twin: report = %q", got)
	}
	if got := reportOf(t, Diagnostic("Client", nil)); len(got) != 0 {
		t.Fatalf("an empty output reports %q", got)
	}
}

// The home directory is shortened only where it stands as a whole path
// prefix: not as the beginning of another user's home, not inside a longer
// path, and not twice in a row. A home that is written with a trailing
// separator is the same home.
func TestClientReportShortensOnlyTheHomeItself(t *testing.T) {
	home := shortHome(t)
	text := "error: read " + home + "2/file, then " + home + ", then /private" + home + "/file, then " + home + "/file, then " + home + home
	want := "error: read " + home + "2/file, then ~, then /private" + home + "/file, then ~/file, then ~" + home
	if got := reportOf(t, Diagnostic("Client", []byte(text))); !slices.Equal(got, []string{want}) {
		t.Fatalf("report = %q, want %q", got, want)
	}
	t.Setenv("HOME", home+string(filepath.Separator))
	t.Setenv("USERPROFILE", home+string(filepath.Separator))
	if got := reportOf(t, Diagnostic("Client", []byte("error: read "+home+"/file"))); !slices.Equal(got, []string{"error: read ~/file"}) {
		t.Fatalf("a home with a trailing separator: report = %q", got)
	}
}
