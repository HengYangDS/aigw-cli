package terminal

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

type passwordInput struct {
	isTerminal func() bool
	read       func() ([]byte, error)
}

func currentPasswordInput() passwordInput {
	return passwordInput{
		isTerminal: func() bool { return term.IsTerminal(int(os.Stdin.Fd())) },
		read:       func() ([]byte, error) { return term.ReadPassword(int(os.Stdin.Fd())) },
	}
}

// ReadHiddenToken reads and optionally confirms one token without echoing its value.
func ReadHiddenToken(out io.Writer, confirm bool) (string, error) {
	return readHiddenToken(out, confirm, currentPasswordInput())
}

func readHiddenToken(out io.Writer, confirm bool, input passwordInput) (string, error) {
	if !input.isTerminal() {
		return "", fmt.Errorf("hidden token input requires an interactive terminal; use --token-stdin")
	}
	value, err := readHiddenLine(out, "Token: ", "token", input)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("empty token is not accepted")
	}
	if !confirm {
		return value, nil
	}
	second, err := readHiddenLine(out, "Confirm token: ", "token confirmation", input)
	if err != nil {
		return "", err
	}
	if value != second {
		return "", fmt.Errorf("token entries do not match")
	}
	return value, nil
}

// readHiddenLine renders label, reads one terminal line with echo disabled and ends the prompt line.
func readHiddenLine(out io.Writer, label, subject string, input passwordInput) (string, error) {
	if _, err := fmt.Fprint(out, label); err != nil {
		return "", fmt.Errorf("prompt for %s: %w", subject, err)
	}
	value, err := input.read()
	if _, writeErr := fmt.Fprintln(out); writeErr != nil {
		return "", fmt.Errorf("finish %s prompt: %w", subject, writeErr)
	}
	if err != nil {
		return "", fmt.Errorf("read hidden %s: %w", subject, err)
	}
	return strings.TrimSpace(string(value)), nil
}
