package terminal

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"charm.land/huh/v2"
)

// ErrInputCancelled marks a prompt that ended without an answer: the person
// cancelled it, or entered nothing where an answer is required.
var ErrInputCancelled = errors.New("input cancelled")

// ErrNoChoice marks an accessible selection that ended because its answers
// named no choice within the answers that it reads, which ends it as a
// cancelled prompt.
var ErrNoChoice = fmt.Errorf("%w: no answer named a choice", ErrInputCancelled)

// maximumUnmatched bounds the answers that name no choice, a number out of
// range, a word for a short list or a filter that matches nothing, that one
// accessible selection reads before it ends without a choice.
const maximumUnmatched = 3

// shortList is the number of choices up to which an accessible selection
// offers no filter: the list is short enough to read.
const shortList = 4

// Choice is one stable value and human label offered by a bounded selection prompt.
type Choice struct {
	Value string
	Label string
}

// Prompt reads interactive input and renders prompts through one input/output pair.
type Prompt struct {
	in         io.Reader
	out        io.Writer
	accessible bool
	hidden     passwordInput
}

// NewPrompter constructs a prompt with explicit accessibility behavior.
func NewPrompter(in io.Reader, out io.Writer, accessible bool) Prompt {
	return Prompt{in: in, out: out, accessible: accessible, hidden: currentPasswordInput()}
}

// Accessible reports whether an environment selects plain prompts for assistive technology.
func Accessible(env map[string]string) bool {
	return env["NO_COLOR"] != "" || env["AIGW_ACCESSIBLE"] != ""
}

// Secret reads a required value that a terminal never echoes, without persisting it.
func (p Prompt) Secret(label string) (string, error) {
	if p.accessible {
		return p.hiddenInput(label)
	}
	var value string
	field := huh.NewInput().
		Title(label).
		EchoMode(huh.EchoModePassword).
		CharLimit(4096).
		Validate(requiredValue).
		Value(&value)
	if err := run(p.form(false, field)); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

// Text reads and trims one required text value.
func (p Prompt) Text(label string) (string, error) {
	if p.accessible {
		return p.plainInput(label, false)
	}
	var value string
	field := huh.NewInput().Title(label).CharLimit(256).Validate(requiredValue).Value(&value)
	if err := run(p.form(false, field)); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

// Select reads one choice. The list filters what the person types after "/",
// which its visible keys say. In accessible mode it numbers the choices and
// reads one answer at a time: a number selects that choice, Enter alone
// selects the first, and for a list longer than shortList words filter the
// choices, selecting the one that matches or numbering the matches again. The
// end of input before an answer cancels the selection, so that Ctrl-D, or a
// script whose input runs out, never chooses.
func (p Prompt) Select(label string, choices []Choice) (string, error) {
	if len(choices) == 0 {
		return "", fmt.Errorf("no options are available")
	}
	if len(choices) == 1 {
		return choices[0].Value, nil
	}
	if p.accessible {
		return p.accessibleSelect(label, choices)
	}
	selected := choices[0].Value
	if err := run(p.selectForm(label, choices, &selected)); err != nil {
		return "", err
	}
	return selected, nil
}

// selectForm is the list that Select shows, which filters what the person
// types after "/", with the keys that it takes.
func (p Prompt) selectForm(label string, choices []Choice, selected *string) *huh.Form {
	options := make([]huh.Option[string], 0, len(choices))
	for _, choice := range choices {
		options = append(options, huh.NewOption(choice.Label, choice.Value))
	}
	return p.form(true, huh.NewSelect[string]().Title(label).Options(options...).Value(selected))
}

// accessibleSelect reads one choice as plain lines from the numbered list.
// An answer that names no choice, a number out of range, a word in a short
// list or a filter that matches nothing, is said and asked again, up to
// maximumUnmatched answers, after which the selection ends without a choice.
func (p Prompt) accessibleSelect(label string, choices []Choice) (string, error) {
	if _, err := fmt.Fprintln(p.out, label); err != nil {
		return "", fmt.Errorf("render selection prompt: %w", err)
	}
	ask := "Select [1]: "
	if len(choices) > shortList {
		ask = "Select a number, or type words to filter [1]: "
	}
	listed := choices
	if err := p.number(listed); err != nil {
		return "", err
	}
	for unmatched := 0; unmatched < maximumUnmatched; {
		answer, err := p.plainInput(ask, true)
		if err != nil {
			return "", err
		}
		if answer == "" {
			return listed[0].Value, nil
		}
		selected, numberErr := strconv.Atoi(answer)
		matches := matching(choices, answer)
		reply := fmt.Sprintf("Choose a number from 1 to %d.", len(listed))
		switch {
		case numberErr == nil && selected >= 1 && selected <= len(listed):
			return listed[selected-1].Value, nil
		case numberErr == nil || len(choices) <= shortList:
		case len(matches) == 1:
			return matches[0].Value, nil
		case len(matches) > 1:
			// A filter that names several choices numbers them again.
			listed = matches
			if err := p.number(listed); err != nil {
				return "", err
			}
			continue
		default:
			reply = fmt.Sprintf("No choice matches %q.", answer)
		}
		unmatched++
		if _, err := fmt.Fprintln(p.out, reply); err != nil {
			return "", fmt.Errorf("render selection prompt: %w", err)
		}
	}
	return "", ErrNoChoice
}

// number writes choices as a numbered list.
func (p Prompt) number(choices []Choice) error {
	for index, choice := range choices {
		if _, err := fmt.Fprintf(p.out, "  %d. %s\n", index+1, choice.Label); err != nil {
			return fmt.Errorf("render selection option: %w", err)
		}
	}
	return nil
}

// matching returns the choices whose label or value contains every word of
// filter, ignoring case, and every choice for an empty filter.
func matching(choices []Choice, filter string) []Choice {
	words := strings.Fields(strings.ToLower(filter))
	var matches []Choice
	for _, choice := range choices {
		text := strings.ToLower(choice.Label + " " + choice.Value)
		if !slices.ContainsFunc(words, func(word string) bool { return !strings.Contains(text, word) }) {
			matches = append(matches, choice)
		}
	}
	return matches
}

// form is one form of fields on the prompt's input and output, which shows
// the keys that they take when help is set.
func (p Prompt) form(help bool, fields ...huh.Field) *huh.Form {
	return huh.NewForm(huh.NewGroup(fields...)).
		WithInput(p.in).
		WithOutput(p.out).
		WithAccessible(p.accessible).
		WithShowHelp(help)
}

// run runs form; a form that ends without an answer was cancelled.
func run(form *huh.Form) error {
	if err := form.Run(); err != nil {
		return fmt.Errorf("%w: %w", ErrInputCancelled, err)
	}
	return nil
}

// requiredValue rejects empty interactive input.
func requiredValue(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("this value cannot be empty")
	}
	return nil
}

// hiddenInput reads a terminal line with echo disabled. Input that is not a terminal is never
// echoed, so it is read as a plain line.
func (p Prompt) hiddenInput(label string) (string, error) {
	if !p.hidden.isTerminal() {
		return p.plainInput(label, false)
	}
	value, err := readHiddenLine(p.out, label, "input", p.hidden)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("%w: no input received", ErrInputCancelled)
	}
	return value, nil
}

// plainInput writes label and reads one line. An empty line is an answer only
// where allowEmpty admits one, and only when the person ended it with Enter:
// input that ends before a line ending with nothing typed is a cancellation.
func (p Prompt) plainInput(label string, allowEmpty bool) (string, error) {
	if _, err := fmt.Fprint(p.out, label); err != nil {
		return "", fmt.Errorf("render input prompt: %w", err)
	}
	line, err := readLine(p.in)
	ended := errors.Is(err, io.EOF)
	if err != nil && !ended {
		return "", fmt.Errorf("read input: %w", err)
	}
	value := strings.TrimSpace(line)
	if value == "" && (!allowEmpty || ended) {
		return "", fmt.Errorf("%w: no input received", ErrInputCancelled)
	}
	return value, nil
}

// readLine reads one line from in a byte at a time, so that the next prompt
// reads the next line: a buffered reader would keep what it read ahead. It
// returns [io.EOF] only for input that ended before a line ending.
func readLine(in io.Reader) (string, error) {
	var line []byte
	next := make([]byte, 1)
	for {
		read, err := in.Read(next)
		if read > 0 {
			if next[0] == '\n' {
				return string(line), nil
			}
			line = append(line, next[0])
		}
		if err != nil {
			return string(line), err
		}
	}
}
