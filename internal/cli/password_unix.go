//go:build !windows

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

func readHiddenToken(out io.Writer, confirm bool) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("hidden token input requires an interactive terminal; use --token-stdin")
	}
	if _, err := fmt.Fprint(out, "Token: "); err != nil {
		return "", fmt.Errorf("prompt for token: %w", err)
	}
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
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
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
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
