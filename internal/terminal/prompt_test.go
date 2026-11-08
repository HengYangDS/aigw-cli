package terminal

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"
	"github.com/charmbracelet/x/ansi"
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
	p := Prompt{in: strings.NewReader("  hello world  \r"), out: out, accessible: false}
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
	p := Prompt{in: strings.NewReader("\x03"), out: out, accessible: false}
	_, err := runPrompt(t, func() (string, error) { return p.Text("Name: ") })
	if !errors.Is(err, ErrInputCancelled) || !strings.Contains(err.Error(), "input cancelled") {
		t.Fatalf("Text() error = %v, want input cancelled", err)
	}
}

func TestTerminalPromptSecretNonAccessibleSubmitsTrimmedValue(t *testing.T) {
	out := &bytes.Buffer{}
	p := Prompt{in: strings.NewReader("s3cret\r"), out: out, accessible: false}
	value, err := runPrompt(t, func() (string, error) { return p.Secret("Token: ") })
	if err != nil {
		t.Fatalf("Secret() error = %v", err)
	}
	if value != "s3cret" {
		t.Fatalf("Secret() = %q, want s3cret", value)
	}
}

func TestTerminalPromptSecretNonAccessibleCancelReturnsError(t *testing.T) {
	prompt := Prompt{in: strings.NewReader("\x03"), out: &bytes.Buffer{}, accessible: false}
	if _, err := runPrompt(t, func() (string, error) { return prompt.Secret("Token: ") }); err == nil || !strings.Contains(err.Error(), "input cancelled") {
		t.Fatalf("Secret() error = %v", err)
	}
}

func TestTerminalPromptSelectNonAccessibleReturnsChosenValue(t *testing.T) {
	out := &bytes.Buffer{}
	p := Prompt{in: strings.NewReader("\r"), out: out, accessible: false}
	choices := []Choice{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	value, err := runPrompt(t, func() (string, error) { return p.Select("Pick: ", choices) })
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if value != "a" {
		t.Fatalf("Select() = %q, want the first option by default", value)
	}
}

// The list shows its keys, the "/" that filters it among them, while the
// Token and text inputs show none.
func TestTerminalPromptSelectNonAccessibleShowsItsFilterKey(t *testing.T) {
	p := Prompt{in: strings.NewReader(""), out: &bytes.Buffer{}, accessible: false}
	selected := "a"
	form := p.selectForm("Pick: ", []Choice{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}, &selected)
	form.Init()
	if view := ansi.Strip(form.View()); !strings.Contains(view, "/ filter") {
		t.Fatalf("the list does not show its filter key:\n%s", view)
	}
	input := p.form(false, huh.NewInput().Title("Token: "))
	input.Init()
	if view := ansi.Strip(input.View()); !strings.Contains(view, "Token:") || strings.Contains(view, "submit") {
		t.Fatalf("an input shows keys:\n%s", view)
	}
}

func TestTerminalPromptSelectNonAccessibleCancelReturnsError(t *testing.T) {
	prompt := Prompt{in: strings.NewReader("\x03"), out: &bytes.Buffer{}, accessible: false}
	choices := []Choice{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	if _, err := runPrompt(t, func() (string, error) { return prompt.Select("Pick: ", choices) }); err == nil || !strings.Contains(err.Error(), "input cancelled") {
		t.Fatalf("Select() error = %v", err)
	}
}

func TestTerminalPromptSelectRejectsEmptyChoiceList(t *testing.T) {
	p := Prompt{in: strings.NewReader(""), out: &bytes.Buffer{}}
	_, err := p.Select("Pick: ", nil)
	if err == nil || !strings.Contains(err.Error(), "no options are available") {
		t.Fatalf("Select() error = %v", err)
	}
}

func TestTerminalPromptSelectShortCircuitsSingleChoiceWithoutReadingInput(t *testing.T) {
	// A reader that errors on any Read call proves Select never touches
	// input when there is only one possible answer.
	p := Prompt{in: errorReader{err: errors.New("must not be read")}, out: &bytes.Buffer{}}
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
	p := Prompt{in: strings.NewReader("  plain-value  \n"), out: out, accessible: true}
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
	p := Prompt{in: strings.NewReader("\n"), out: &bytes.Buffer{}, accessible: true}
	_, err := p.Text("Name: ")
	if !errors.Is(err, ErrInputCancelled) || err.Error() != "input cancelled: no input received" {
		t.Fatalf("Text() error = %v", err)
	}
}

func TestTerminalPromptAccessibleSecretHidesTerminalInput(t *testing.T) {
	out := &bytes.Buffer{}
	plain := strings.NewReader("echoed\n")
	p := Prompt{in: plain, out: out, accessible: true, hidden: scriptedPasswordInput(nil, " s3cret ")}
	value, err := p.Secret("Token: ")
	if err != nil || value != "s3cret" {
		t.Fatalf("Secret() = %q, %v", value, err)
	}
	if out.String() != "Token: \n" {
		t.Fatalf("prompt output = %q, want only the label and a line ending", out.String())
	}
	if plain.Len() != len("echoed\n") {
		t.Fatal("a terminal Token was read through the echoing line reader")
	}
}

func TestTerminalPromptAccessibleSecretRefusesEmptyOrFailedTerminalInput(t *testing.T) {
	for _, test := range []struct {
		name  string
		input passwordInput
		want  string
	}{
		{name: "empty", input: scriptedPasswordInput(nil, "  "), want: "input cancelled: no input received"},
		{name: "read", input: scriptedPasswordInput(errors.New("read failed")), want: "read hidden input"},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := Prompt{in: strings.NewReader(""), out: &bytes.Buffer{}, accessible: true, hidden: test.input}
			_, err := p.Secret("Token: ")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Secret() error = %v, want %q", err, test.want)
			}
			// Only an empty answer is a cancellation; a failed read is not.
			if cancelled := errors.Is(err, ErrInputCancelled); cancelled != (test.name == "empty") {
				t.Fatalf("Secret() error %v is a cancellation: %t", err, cancelled)
			}
		})
	}
}

func TestTerminalPromptAccessibleSecretReadsPipedLineWithoutEcho(t *testing.T) {
	out := &bytes.Buffer{}
	p := Prompt{in: strings.NewReader("hunter2\n"), out: out, accessible: true, hidden: passwordInput{isTerminal: func() bool { return false }}}
	value, err := p.Secret("Token: ")
	if err != nil || value != "hunter2" {
		t.Fatalf("Secret() = %q, %v", value, err)
	}
	if strings.Contains(out.String(), "hunter2") {
		t.Fatalf("prompt output %q contains the Token", out.String())
	}
}

func TestAccessibleFollowsTheEnvironment(t *testing.T) {
	for _, test := range []struct {
		env  map[string]string
		want bool
	}{
		{env: map[string]string{}, want: false},
		{env: map[string]string{"NO_COLOR": ""}, want: false},
		{env: map[string]string{"NO_COLOR": "1"}, want: true},
		{env: map[string]string{"AIGW_ACCESSIBLE": "1"}, want: true},
	} {
		if got := Accessible(test.env); got != test.want {
			t.Fatalf("Accessible(%v) = %v, want %v", test.env, got, test.want)
		}
	}
}

func TestTerminalPromptAccessibleSelectDefaultsToFirstChoiceOnEmptyInput(t *testing.T) {
	out := &bytes.Buffer{}
	p := Prompt{in: strings.NewReader("\n"), out: out, accessible: true}
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

// Input that ends before an answer, as Ctrl-D or a script whose input runs
// out ends it, cancels an accessible selection instead of choosing the first
// choice: at the first answer, after an answer that named no choice, and
// after a filter that numbered its matches again. An answer typed before the
// end of input still counts.
func TestTerminalPromptAccessibleSelectCancelsAtTheEndOfInput(t *testing.T) {
	short := []Choice{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	long := []Choice{
		{Value: "team/team-model", Label: "Team model"}, {Value: "team/MiniMax-M3", Label: "MiniMax M3"}, {Value: "team/minimal", Label: "Minimal"},
		{Value: "team/spare-model", Label: "Spare model"}, {Value: "team/other-model", Label: "Other model"},
	}
	for _, test := range []struct {
		name, input string
		choices     []Choice
	}{
		{name: "nothing typed", choices: short},
		{name: "blanks without a line ending", input: "  ", choices: short},
		{name: "after an answer that named no choice", input: "9\n", choices: short},
		{name: "a long list", choices: long},
		{name: "after a filter", input: "mini\n", choices: long},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := Prompt{in: strings.NewReader(test.input), out: &bytes.Buffer{}, accessible: true}
			value, err := p.Select("Pick: ", test.choices)
			if !errors.Is(err, ErrInputCancelled) || errors.Is(err, ErrNoChoice) || value != "" {
				t.Fatalf("Select() = %q, %v, want a cancellation", value, err)
			}
		})
	}
	p := Prompt{in: strings.NewReader("2"), out: &bytes.Buffer{}, accessible: true}
	if value, err := p.Select("Pick: ", short); err != nil || value != "b" {
		t.Fatalf("Select() of an answer that the input ends = %q, %v, want b", value, err)
	}
}

func TestTerminalPromptAccessibleSelectAcceptsChosenNumber(t *testing.T) {
	p := Prompt{in: strings.NewReader("2\n"), out: &bytes.Buffer{}, accessible: true}
	choices := []Choice{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	value, err := p.Select("Pick: ", choices)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if value != "b" {
		t.Fatalf("Select() = %q, want b", value)
	}
}

// In accessible mode one prompt takes a number or, for a list longer than
// four, words that filter it: a filter that leaves one choice selects it, one
// that leaves several numbers them again for the next answer, and one that
// matches nothing is said and asked again. The choices whose label or value
// contains each word, ignoring case, match.
func TestTerminalPromptAccessibleSelectFiltersALongList(t *testing.T) {
	choices := []Choice{
		{Value: "team/team-model", Label: "Team model"}, {Value: "team/MiniMax-M3", Label: "MiniMax M3"}, {Value: "team/minimal", Label: "Minimal"},
		{Value: "team/spare-model", Label: "Spare model"}, {Value: "team/other-model", Label: "Other model"},
	}
	for _, test := range []struct {
		name, input, want string
		listed, unlisted  []string
	}{
		{name: "filter", input: "mini\n2\n", want: "team/minimal", listed: []string{"5. Other model", "1. MiniMax M3\n  2. Minimal\n"}},
		{name: "one match", input: "MINI m3\n", want: "team/MiniMax-M3", unlisted: []string{"1. MiniMax M3\n"}},
		{name: "a number", input: "3\n", want: "team/minimal", listed: []string{"1. Team model", "3. Minimal", "5. Other model"}},
		{name: "no match", input: "opus\nspare\n", want: "team/spare-model", listed: []string{`No choice matches "opus".`}},
	} {
		t.Run(test.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			p := Prompt{in: strings.NewReader(test.input), out: out, accessible: true}
			value, err := p.Select("Pick: ", choices)
			if err != nil || value != test.want {
				t.Fatalf("Select() = %q, %v, want %q", value, err, test.want)
			}
			if !strings.Contains(out.String(), "Select a number, or type words to filter [1]: ") {
				t.Fatalf("Select() does not offer a filter: %q", out.String())
			}
			for _, line := range test.listed {
				if !strings.Contains(out.String(), line) {
					t.Fatalf("Select() output lacks %q: %q", line, out.String())
				}
			}
			for _, line := range test.unlisted {
				if strings.Contains(out.String(), line) {
					t.Fatalf("Select() output has %q: %q", line, out.String())
				}
			}
		})
	}
}

// A list of four choices or fewer is numbered and read as a number, with no
// filter: a word, like a number out of range, is said and asked again, and
// after three such answers the selection ends without a choice, as a
// cancelled prompt ends.
func TestTerminalPromptAccessibleSelectRequiresAnOfferedNumber(t *testing.T) {
	choices := []Choice{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	t.Run("asked again", func(t *testing.T) {
		out := &bytes.Buffer{}
		p := Prompt{in: strings.NewReader("9\nbeta\n2\n"), out: out, accessible: true}
		value, err := p.Select("Pick: ", choices)
		if err != nil || value != "b" {
			t.Fatalf("Select() = %q, %v, want b", value, err)
		}
		if strings.Count(out.String(), "Choose a number from 1 to 2.") != 2 || strings.Contains(out.String(), "filter") {
			t.Fatalf("Select() output = %q", out.String())
		}
	})
	t.Run("bounded", func(t *testing.T) {
		p := Prompt{in: strings.NewReader("9\nnope\n0\n1\n"), out: &bytes.Buffer{}, accessible: true}
		if _, err := p.Select("Pick: ", choices); !errors.Is(err, ErrNoChoice) || !errors.Is(err, ErrInputCancelled) {
			t.Fatalf("Select() error = %v, want no choice", err)
		}
	})
}

func TestTerminalPromptAccessibleSelectSurfacesLabelWriteFailure(t *testing.T) {
	p := Prompt{in: strings.NewReader("1\n"), out: failingPromptWriter{err: errors.New("closed pipe")}, accessible: true}
	choices := []Choice{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	_, err := p.Select("Pick: ", choices)
	if err == nil || !strings.Contains(err.Error(), "render selection prompt") {
		t.Fatalf("Select() error = %v", err)
	}
}

// failAfterWriter accepts allowed writes and fails every later one.
type failAfterWriter struct {
	writes, allowed int
	err             error
}

func (writer *failAfterWriter) Write(data []byte) (int, error) {
	writer.writes++
	if writer.writes > writer.allowed {
		return 0, writer.err
	}
	return len(data), nil
}

func TestTerminalPromptAccessibleSelectSurfacesOptionAndInputFailures(t *testing.T) {
	choices := []Choice{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}}
	t.Run("prompt output", func(t *testing.T) {
		writer := &failAfterWriter{allowed: 3, err: errors.New("closed pipe")}
		prompt := Prompt{in: strings.NewReader("1\n"), out: writer, accessible: true}
		if _, err := prompt.Select("Pick: ", choices); err == nil || !strings.Contains(err.Error(), "render input prompt") {
			t.Fatalf("Select() error = %v", err)
		}
	})
	t.Run("option output", func(t *testing.T) {
		writer := &failAfterWriter{allowed: 1, err: errors.New("closed pipe")}
		prompt := Prompt{in: strings.NewReader("1\n"), out: writer, accessible: true}
		if _, err := prompt.Select("Pick: ", choices); err == nil || !strings.Contains(err.Error(), "render selection option") {
			t.Fatalf("Select() error = %v", err)
		}
	})
	t.Run("selection input", func(t *testing.T) {
		prompt := Prompt{in: errorReader{err: errors.New("broken pipe")}, out: &bytes.Buffer{}, accessible: true}
		if _, err := prompt.Select("Pick: ", choices); err == nil || !strings.Contains(err.Error(), "read input") {
			t.Fatalf("Select() error = %v", err)
		}
	})
}

func TestTerminalPromptAccessiblePlainInputSurfacesPromptWriteFailure(t *testing.T) {
	p := Prompt{in: strings.NewReader("value\n"), out: failingPromptWriter{err: errors.New("closed pipe")}, accessible: true}
	_, err := p.Text("Name: ")
	if err == nil || !strings.Contains(err.Error(), "render input prompt") {
		t.Fatalf("Text() error = %v", err)
	}
}

func TestTerminalPromptAccessiblePlainInputSurfacesReadFailure(t *testing.T) {
	p := Prompt{in: errorReader{err: errors.New("broken pipe")}, out: &bytes.Buffer{}, accessible: true}
	_, err := p.Text("Name: ")
	if err == nil || !strings.Contains(err.Error(), "read input") {
		t.Fatalf("Text() error = %v", err)
	}
}

func TestNewWiresAccessibleIO(t *testing.T) {
	out := &bytes.Buffer{}
	prompt := NewPrompter(strings.NewReader("value\n"), out, true)
	value, err := prompt.Text("Name: ")
	if err != nil || value != "value" {
		t.Fatalf("Text = %q, %v", value, err)
	}
}

func TestPromptRemainingValidationBranches(t *testing.T) {
	t.Run("secret empty accessible", func(t *testing.T) {
		prompt := NewPrompter(strings.NewReader("\n"), &bytes.Buffer{}, true)
		if _, err := prompt.Secret("Token: "); err == nil {
			t.Fatal("empty secret accepted")
		}
	})
	t.Run("accessible selection default", func(t *testing.T) {
		prompt := NewPrompter(strings.NewReader("\n"), &bytes.Buffer{}, true)
		value, err := prompt.Select("Pick", []Choice{{Value: "one", Label: "One"}, {Value: "two", Label: "Two"}})
		if err != nil || value != "one" {
			t.Fatalf("Select = %q, %v", value, err)
		}
	})
}
