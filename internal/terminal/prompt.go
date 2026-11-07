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
// which its visible keys say. In accessible mode it asks first for an
// optional filter, numbers the choices that match, and reads the number of
// one, the first by default.
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

// accessibleSelect reads one choice as plain lines: an optional filter, asked
// again while it matches no choice, and then the number of a matching
// choice.
func (p Prompt) accessibleSelect(label string, choices []Choice) (string, error) {
	if _, err := fmt.Fprintln(p.out, label); err != nil {
		return "", fmt.Errorf("render selection prompt: %w", err)
	}
	var matches []Choice
	for {
		filter, err := p.plainInput(fmt.Sprintf("Filter the %d choices, or press Enter for all: ", len(choices)), true)
		if err != nil {
			return "", err
		}
		if matches = matching(choices, filter); len(matches) > 0 {
			break
		}
		if _, err := fmt.Fprintf(p.out, "No choice matches %q.\n", filter); err != nil {
			return "", fmt.Errorf("render selection prompt: %w", err)
		}
	}
	for index, choice := range matches {
		if _, err := fmt.Fprintf(p.out, "  %d. %s\n", index+1, choice.Label); err != nil {
			return "", fmt.Errorf("render selection option: %w", err)
		}
	}
	value, err := p.plainInput("Select [1]: ", true)
	if err != nil {
		return "", err
	}
	if value == "" {
		return matches[0].Value, nil
	}
	selected, err := strconv.Atoi(value)
	if err != nil || selected < 1 || selected > len(matches) {
		return "", fmt.Errorf("invalid selection %q", value)
	}
	return matches[selected-1].Value, nil
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

func (p Prompt) plainInput(label string, allowEmpty bool) (string, error) {
	if _, err := fmt.Fprint(p.out, label); err != nil {
		return "", fmt.Errorf("render input prompt: %w", err)
	}
	line, err := readLine(p.in)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read input: %w", err)
	}
	value := strings.TrimSpace(line)
	if value == "" && !allowEmpty {
		return "", fmt.Errorf("%w: no input received", ErrInputCancelled)
	}
	return value, nil
}

// readLine reads one line from in a byte at a time, so that the next prompt
// reads the next line: a buffered reader would keep what it read ahead.
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
