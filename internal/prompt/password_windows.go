//go:build windows

package prompt

import (
	"os"

	"golang.org/x/term"
)

func currentPasswordInput() passwordInput {
	return passwordInput{
		isTerminal: func() bool { return term.IsTerminal(int(os.Stdin.Fd())) },
		read:       func() ([]byte, error) { return term.ReadPassword(int(os.Stdin.Fd())) },
	}
}
