package terminal

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// OptionalSecret returns what the person typed and, unlike Secret, nothing
// for Enter alone, in each of its forms: the form, the plain line that
// accessible mode reads from input that is not a terminal, and the hidden line
// that it reads from one. A cancelled form and input that ends before an
// answer are cancellations, never an empty answer, and no form echoes the
// value.
func TestOptionalSecretTakesEnterAloneAsNoValue(t *testing.T) {
	plain := passwordInput{isTerminal: func() bool { return false }}
	for _, test := range []struct {
		name       string
		input      string
		accessible bool
		hidden     passwordInput
		want       string
		cancelled  bool
	}{
		{name: "form, a value", input: "  s3cret \r", want: "s3cret"},
		{name: "form, Enter alone", input: "\r"},
		{name: "form, cancelled", input: "\x03", cancelled: true},
		{name: "plain line, a value", input: " s3cret \n", accessible: true, hidden: plain, want: "s3cret"},
		{name: "plain line, Enter alone", input: "\n", accessible: true, hidden: plain},
		{name: "plain line, input ends", accessible: true, hidden: plain, cancelled: true},
		{name: "hidden line, a value", accessible: true, hidden: scriptedPasswordInput(nil, " s3cret "), want: "s3cret"},
		{name: "hidden line, Enter alone", accessible: true, hidden: scriptedPasswordInput(nil, ""), want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			prompt := Prompt{in: strings.NewReader(test.input), out: out, accessible: test.accessible, hidden: test.hidden}
			value, err := runPrompt(t, func() (string, error) { return prompt.OptionalSecret("Token, or Enter to skip: ") })
			if test.cancelled {
				if !errors.Is(err, ErrInputCancelled) {
					t.Fatalf("OptionalSecret() = %q, %v, want a cancelled prompt", value, err)
				}
				return
			}
			if err != nil || value != test.want {
				t.Fatalf("OptionalSecret() = %q, %v, want %q", value, err, test.want)
			}
			if strings.Contains(out.String(), "s3cret") {
				t.Fatalf("the prompt echoed the value:\n%s", out)
			}
		})
	}
}

// A hidden line that the terminal cannot read fails the prompt with the read's
// error, as Secret's does.
func TestOptionalSecretReportsAFailedHiddenRead(t *testing.T) {
	prompt := Prompt{in: strings.NewReader(""), out: &bytes.Buffer{}, accessible: true, hidden: scriptedPasswordInput(errors.New("read failed"))}
	if _, err := prompt.OptionalSecret("Token: "); err == nil || !strings.Contains(err.Error(), "read hidden input: read failed") {
		t.Fatalf("OptionalSecret() error = %v", err)
	}
}
