package terminal

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

var multiChoices = []Choice{{Value: "dmxapi", Label: "DMXAPI"}, {Value: "ucloud", Label: "UCloud"}, {Value: "aihubmix", Label: "AIHubMix"}}

// runMultiSelect runs a multiple-choice prompt on a goroutine and fails the
// test if it does not return promptly.
func runMultiSelect(t *testing.T, p Prompt) ([]string, error) {
	t.Helper()
	type result struct {
		values []string
		err    error
	}
	done := make(chan result, 1)
	go func() {
		values, err := p.MultiSelect("Which providers' Tokens do you have?", multiChoices)
		done <- result{values, err}
	}()
	select {
	case r := <-done:
		return r.values, r.err
	case <-time.After(5 * time.Second):
		t.Fatal("prompt did not return within the expected bound")
		return nil, nil
	}
}

// The list confirms what Space marked, in the order of the choices, and Enter
// alone confirms none.
func TestTerminalPromptMultiSelectReturnsMarkedChoices(t *testing.T) {
	for _, test := range []struct {
		name, input string
		want        []string
	}{
		{"none", "\r", nil},
		{"the first", " \r", []string{"dmxapi"}},
		{"the third and the first", "\x1b[B\x1b[B \x1b[A\x1b[A \r", []string{"dmxapi", "aihubmix"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			values, err := runMultiSelect(t, Prompt{in: strings.NewReader(test.input), out: &bytes.Buffer{}})
			if err != nil || !slices.Equal(values, test.want) {
				t.Fatalf("MultiSelect() = %q, %v, want %q", values, err, test.want)
			}
		})
	}
	if _, err := runMultiSelect(t, Prompt{in: strings.NewReader("\x03"), out: &bytes.Buffer{}}); !errors.Is(err, ErrInputCancelled) {
		t.Fatalf("a cancelled list: %v", err)
	}
	if values, err := (Prompt{}).MultiSelect("Pick", nil); err != nil || values != nil {
		t.Fatalf("no choices = %q, %v", values, err)
	}
}

// The accessible form numbers the choices and reads one line of numbers;
// Enter alone chooses none, an answer that is not such numbers is asked
// again, and input that ends before an answer cancels.
func TestTerminalPromptAccessibleMultiSelectReadsNumbers(t *testing.T) {
	for _, test := range []struct {
		name, input string
		want        []string
		err         error
	}{
		{"none", "\n", nil, nil},
		{"two, in the order of the choices", "3, 1\n", []string{"dmxapi", "aihubmix"}, nil},
		{"after an answer out of range", "4\n2\n", []string{"ucloud"}, nil},
		{"after a word", "dmxapi\n1\n", []string{"dmxapi"}, nil},
		{"three answers that name nothing", "0\nx\n9\n", nil, ErrNoChoice},
		{"the end of input", "", nil, ErrInputCancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			values, err := runMultiSelect(t, Prompt{in: strings.NewReader(test.input), out: out, accessible: true})
			if !slices.Equal(values, test.want) || !errors.Is(err, test.err) || (test.err == nil && err != nil) {
				t.Fatalf("MultiSelect() = %q, %v, want %q, %v", values, err, test.want, test.err)
			}
			if !strings.Contains(out.String(), "1. DMXAPI") || !strings.Contains(out.String(), "3. AIHubMix") {
				t.Fatalf("the choices are not numbered:\n%s", out)
			}
		})
	}
	if _, err := (Prompt{out: failingPromptWriter{err: errors.New("closed")}, accessible: true}).MultiSelect("Pick", multiChoices); err == nil {
		t.Fatal("a failed write did not end the prompt")
	}
}
