package cli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type terminalPrompt struct {
	in  io.Reader
	out io.Writer
}

func (p terminalPrompt) Secret(label string) (string, error) {
	fmt.Fprintln(p.out, label)
	return readHiddenToken(p.out, false)
}

func (p terminalPrompt) Select(label string, choices []Choice) (string, error) {
	if len(choices) == 0 {
		return "", fmt.Errorf("no choices available")
	}
	if len(choices) == 1 {
		return choices[0].Value, nil
	}
	fmt.Fprintln(p.out, label)
	for index, choice := range choices {
		fmt.Fprintf(p.out, "  %d. %s\n", index+1, choice.Label)
	}
	fmt.Fprint(p.out, "选择 [1]: ")
	line, err := bufio.NewReader(p.in).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return choices[0].Value, nil
	}
	selected, err := strconv.Atoi(line)
	if err != nil || selected < 1 || selected > len(choices) {
		return "", fmt.Errorf("invalid selection %q", line)
	}
	return choices[selected-1].Value, nil
}
