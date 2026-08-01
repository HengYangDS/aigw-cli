package prompt

import (
	"fmt"
	"io"
	"strings"
)

type passwordInput struct {
	isTerminal func() bool
	read       func() ([]byte, error)
}

func ReadHiddenToken(out io.Writer, confirm bool) (string, error) {
	return readHiddenToken(out, confirm, currentPasswordInput())
}

func readHiddenToken(out io.Writer, confirm bool, input passwordInput) (string, error) {
	if !input.isTerminal() {
		return "", fmt.Errorf("hidden token input requires an interactive terminal; use --token-stdin")
	}
	if _, err := fmt.Fprint(out, "Token: "); err != nil {
		return "", fmt.Errorf("prompt for token: %w", err)
	}
	first, err := input.read()
	if _, writeErr := fmt.Fprintln(out); writeErr != nil {
		return "", fmt.Errorf("finish token prompt: %w", writeErr)
	}
	if err != nil {
		return "", fmt.Errorf("read hidden token: %w", err)
	}
	value := strings.TrimSpace(string(first))
	if value == "" {
		return "", fmt.Errorf("empty token is not accepted")
	}
	if !confirm {
		return value, nil
	}
	if _, err := fmt.Fprint(out, "Confirm token: "); err != nil {
		return "", fmt.Errorf("prompt to confirm token: %w", err)
	}
	second, err := input.read()
	if _, writeErr := fmt.Fprintln(out); writeErr != nil {
		return "", fmt.Errorf("finish token confirmation prompt: %w", writeErr)
	}
	if err != nil {
		return "", fmt.Errorf("confirm hidden token: %w", err)
	}
	if value != strings.TrimSpace(string(second)) {
		return "", fmt.Errorf("token entries do not match")
	}
	return value, nil
}
