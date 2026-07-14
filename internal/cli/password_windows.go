//go:build windows

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

func readHiddenToken(out io.Writer, confirm bool) (string, error) {
	fmt.Fprint(out, "Token: ")
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(out)
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
	fmt.Fprint(out, "Confirm token: ")
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(out)
	if err != nil {
		return "", fmt.Errorf("confirm hidden token: %w", err)
	}
	if value != strings.TrimSpace(string(second)) {
		return "", fmt.Errorf("token entries do not match")
	}
	return value, nil
}
