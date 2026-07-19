package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"charm.land/huh/v2"
)

type terminalPrompt struct {
	in         io.Reader
	out        io.Writer
	accessible bool
}

func (p terminalPrompt) Secret(label string) (string, error) {
	if p.accessible || os.Getenv("AIGW_ACCESSIBLE") != "" {
		return p.plainInput(label, false)
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

func (p terminalPrompt) Text(label string) (string, error) {
	if p.accessible || os.Getenv("AIGW_ACCESSIBLE") != "" {
		return p.plainInput(label, false)
	}
	var value string
	field := huh.NewInput().Title(label).CharLimit(256).Validate(requiredValue).Value(&value)
	if err := p.run(field); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (p terminalPrompt) Select(label string, choices []Choice) (string, error) {
	if len(choices) == 0 {
		return "", fmt.Errorf("no options are available")
	}
	if len(choices) == 1 {
		return choices[0].Value, nil
	}
	if p.accessible || os.Getenv("AIGW_ACCESSIBLE") != "" {
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

func (p terminalPrompt) run(fields ...huh.Field) error {
	form := huh.NewForm(huh.NewGroup(fields...)).
		WithInput(p.in).
		WithOutput(p.out).
		WithAccessible(p.accessible || os.Getenv("AIGW_ACCESSIBLE") != "").
		WithShowHelp(false)
	if err := form.Run(); err != nil {
		return fmt.Errorf("input cancelled: %w", err)
	}
	return nil
}

func requiredValue(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("this value cannot be empty")
	}
	return nil
}

func (p terminalPrompt) plainInput(label string, allowEmpty bool) (string, error) {
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
