package terminal

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"charm.land/huh/v2"
)

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
	if err := p.run(field); err != nil {
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
	if err := p.run(field); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

// Select reads one choice, accepting either its stable value or displayed ordinal.
func (p Prompt) Select(label string, choices []Choice) (string, error) {
	if len(choices) == 0 {
		return "", fmt.Errorf("no options are available")
	}
	if len(choices) == 1 {
		return choices[0].Value, nil
	}
	if !p.accessible {
		options := make([]huh.Option[string], 0, len(choices))
		for _, choice := range choices {
			options = append(options, huh.NewOption(choice.Label, choice.Value))
		}
		selected := choices[0].Value
		field := huh.NewSelect[string]().Title(label).Options(options...).Value(&selected)
		if err := p.run(field); err != nil {
			return "", err
		}
		return selected, nil
	}
	if _, err := fmt.Fprintln(p.out, label); err != nil {
		return "", fmt.Errorf("render selection prompt: %w", err)
	}
	for index, choice := range choices {
		if _, err := fmt.Fprintf(p.out, "  %d. %s\n", index+1, choice.Label); err != nil {
			return "", fmt.Errorf("render selection option: %w", err)
		}
	}
	value, err := p.plainInput("Select [1]: ", true)
	if err != nil {
		return "", err
	}
	if value == "" {
		return choices[0].Value, nil
	}
	selected, err := strconv.Atoi(value)
	if err != nil || selected < 1 || selected > len(choices) {
		return "", fmt.Errorf("invalid selection %q", value)
	}
	return choices[selected-1].Value, nil
}

func (p Prompt) run(fields ...huh.Field) error {
	form := huh.NewForm(huh.NewGroup(fields...)).
		WithInput(p.in).
		WithOutput(p.out).
		WithAccessible(p.accessible).
		WithShowHelp(false)
	if err := form.Run(); err != nil {
		return fmt.Errorf("input cancelled: %w", err)
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
		return "", fmt.Errorf("setup cancelled: no input received")
	}
	return value, nil
}

func (p Prompt) plainInput(label string, allowEmpty bool) (string, error) {
	if _, err := fmt.Fprint(p.out, label); err != nil {
		return "", fmt.Errorf("render input prompt: %w", err)
	}
	line, err := bufio.NewReader(p.in).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read input: %w", err)
	}
	value := strings.TrimSpace(line)
	if value == "" && !allowEmpty {
		return "", fmt.Errorf("setup cancelled: no input received")
	}
	return value, nil
}
