package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// runPrompt executes a prompt call on a goroutine and fails the test if it
// does not return promptly. huh forms driven by a non-tty reader/writer do
// not block on real terminal state, so a short bound catches accidental
// hangs without flaking under load.
func runPrompt(t *testing.T, fn func() (string, error)) (string, error) {
	t.Helper()
	type result struct {
		value string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		value, err := fn()
		done <- result{value, err}
	}()
	select {
	case r := <-done:
		return r.value, r.err
	case <-time.After(5 * time.Second):
		t.Fatal("prompt did not return within the expected bound")
		return "", nil
	}
}

type failingPromptWriter struct{ err error }

func (w failingPromptWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRequiredValueRejectsBlankAndAcceptsNonBlank(t *testing.T) {
	if err := requiredValue(""); err == nil {
		t.Fatal("empty value should be rejected")
	}
	if err := requiredValue("   "); err == nil {
		t.Fatal("whitespace-only value should be rejected")
	}
	if err := requiredValue(" ok "); err != nil {
		t.Fatalf("non-blank value rejected: %v", err)
	}
}

func TestTerminalPromptTextNonAccessibleSubmitsTrimmedValue(t *testing.T) {
	out := &bytes.Buffer{}
	p := terminalPrompt{in: strings.NewReader("  hello world  \r"), out: out, accessible: false}
	value, err := runPrompt(t, func() (string, error) { return p.Text("Name: ") })
	if err != nil {
		t.Fatalf("Text() error = %v", err)
	}
	if value != "hello world" {
		t.Fatalf("Text() = %q, want trimmed value", value)
	}
}

func TestTerminalPromptTextNonAccessibleCancelReturnsError(t *testing.T) {
	out := &bytes.Buffer{}
	p := terminalPrompt{in: strings.NewReader("\x03"), out: out, accessible: false}
	_, err := runPrompt(t, func() (string, error) { return p.Text("Name: ") })
	if err == nil || !strings.Contains(err.Error(), "input cancelled") {
		t.Fatalf("Text() error = %v, want input cancelled", err)
	}
}

func TestTerminalPromptSecretNonAccessibleSubmitsTrimmedValue(t *testing.T) {
	out := &bytes.Buffer{}
	p := terminalPrompt{in: strings.NewReader("s3cret\r"), out: out, accessible: false}
	value, err := runPrompt(t, func() (string, error) { return p.Secret("Token: ") })
	if err != nil {
		t.Fatalf("Secret() error = %v", err)
	}
	if value != "s3cret" {
		t.Fatalf("Secret() = %q, want s3cret", value)
	}
}

func TestTerminalPromptSelectNonAccessibleReturnsChosenValue(t *testing.T) {
	out := &bytes.Buffer{}
	p := terminalPrompt{in: strings.NewReader("\r"), out: out, accessible: false}
	choices := []Choice{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	value, err := runPrompt(t, func() (string, error) { return p.Select("Pick: ", choices) })
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if value != "a" {
		t.Fatalf("Select() = %q, want the first option by default", value)
	}
}

func TestTerminalPromptSelectRejectsEmptyChoiceList(t *testing.T) {
	p := terminalPrompt{in: strings.NewReader(""), out: &bytes.Buffer{}}
	_, err := p.Select("Pick: ", nil)
	if err == nil || !strings.Contains(err.Error(), "no options are available") {
		t.Fatalf("Select() error = %v", err)
	}
}

func TestTerminalPromptSelectShortCircuitsSingleChoiceWithoutReadingInput(t *testing.T) {
	// A reader that errors on any Read call proves Select never touches
	// input when there is only one possible answer.
	p := terminalPrompt{in: errorReader{err: errors.New("must not be read")}, out: &bytes.Buffer{}}
	value, err := p.Select("Pick: ", []Choice{{Value: "only", Label: "Only"}})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if value != "only" {
		t.Fatalf("Select() = %q, want only", value)
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestTerminalPromptAccessibleTextReadsPlainLine(t *testing.T) {
	out := &bytes.Buffer{}
	p := terminalPrompt{in: strings.NewReader("  plain-value  \n"), out: out, accessible: true}
	value, err := p.Text("Name: ")
	if err != nil {
		t.Fatalf("Text() error = %v", err)
	}
	if value != "plain-value" {
		t.Fatalf("Text() = %q, want plain-value", value)
	}
	if !strings.Contains(out.String(), "Name: ") {
		t.Fatalf("prompt label was not rendered: %q", out.String())
	}
}

func TestTerminalPromptAccessibleTextRejectsEmptyInput(t *testing.T) {
	p := terminalPrompt{in: strings.NewReader("\n"), out: &bytes.Buffer{}, accessible: true}
	_, err := p.Text("Name: ")
	if err == nil || !strings.Contains(err.Error(), "no input received") {
		t.Fatalf("Text() error = %v", err)
	}
}

func TestTerminalPromptAccessibleSecretReadsPlainLine(t *testing.T) {
	p := terminalPrompt{in: strings.NewReader("hunter2\n"), out: &bytes.Buffer{}, accessible: true}
	value, err := p.Secret("Token: ")
	if err != nil {
		t.Fatalf("Secret() error = %v", err)
	}
	if value != "hunter2" {
		t.Fatalf("Secret() = %q, want hunter2", value)
	}
}

func TestTerminalPromptEnvironmentAccessibleOverridesFieldSetting(t *testing.T) {
	t.Setenv("AIGW_ACCESSIBLE", "1")
	p := terminalPrompt{in: strings.NewReader("env-value\n"), out: &bytes.Buffer{}, accessible: false}
	value, err := p.Text("Name: ")
	if err != nil {
		t.Fatalf("Text() error = %v", err)
	}
	if value != "env-value" {
		t.Fatalf("Text() = %q, want env-value", value)
	}
}

func TestTerminalPromptAccessibleSelectDefaultsToFirstChoiceOnEmptyInput(t *testing.T) {
	out := &bytes.Buffer{}
	p := terminalPrompt{in: strings.NewReader("\n"), out: out, accessible: true}
	choices := []Choice{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	value, err := p.Select("Pick: ", choices)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if value != "a" {
		t.Fatalf("Select() = %q, want default first choice", value)
	}
	if !strings.Contains(out.String(), "1. Alpha") || !strings.Contains(out.String(), "2. Beta") {
		t.Fatalf("Select() did not render numbered options: %q", out.String())
	}
}

func TestTerminalPromptAccessibleSelectAcceptsChosenNumber(t *testing.T) {
	p := terminalPrompt{in: strings.NewReader("2\n"), out: &bytes.Buffer{}, accessible: true}
	choices := []Choice{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	value, err := p.Select("Pick: ", choices)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if value != "b" {
		t.Fatalf("Select() = %q, want b", value)
	}
}

func TestTerminalPromptAccessibleSelectRejectsNonNumericInput(t *testing.T) {
	p := terminalPrompt{in: strings.NewReader("nope\n"), out: &bytes.Buffer{}, accessible: true}
	choices := []Choice{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	_, err := p.Select("Pick: ", choices)
	if err == nil || !strings.Contains(err.Error(), "invalid selection") {
		t.Fatalf("Select() error = %v", err)
	}
}

func TestTerminalPromptAccessibleSelectRejectsOutOfRangeNumber(t *testing.T) {
	p := terminalPrompt{in: strings.NewReader("9\n"), out: &bytes.Buffer{}, accessible: true}
	choices := []Choice{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	_, err := p.Select("Pick: ", choices)
	if err == nil || !strings.Contains(err.Error(), "invalid selection") {
		t.Fatalf("Select() error = %v", err)
	}
}

func TestTerminalPromptAccessibleSelectSurfacesLabelWriteFailure(t *testing.T) {
	p := terminalPrompt{in: strings.NewReader("1\n"), out: failingPromptWriter{err: errors.New("closed pipe")}, accessible: true}
	choices := []Choice{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	_, err := p.Select("Pick: ", choices)
	if err == nil || !strings.Contains(err.Error(), "render selection prompt") {
		t.Fatalf("Select() error = %v", err)
	}
}

func TestTerminalPromptAccessiblePlainInputSurfacesPromptWriteFailure(t *testing.T) {
	p := terminalPrompt{in: strings.NewReader("value\n"), out: failingPromptWriter{err: errors.New("closed pipe")}, accessible: true}
	_, err := p.Text("Name: ")
	if err == nil || !strings.Contains(err.Error(), "render input prompt") {
		t.Fatalf("Text() error = %v", err)
	}
}

func TestTerminalPromptAccessiblePlainInputSurfacesReadFailure(t *testing.T) {
	p := terminalPrompt{in: errorReader{err: errors.New("broken pipe")}, out: &bytes.Buffer{}, accessible: true}
	_, err := p.Text("Name: ")
	if err == nil || !strings.Contains(err.Error(), "read input") {
		t.Fatalf("Text() error = %v", err)
	}
}
