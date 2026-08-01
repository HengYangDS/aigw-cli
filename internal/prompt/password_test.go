package prompt

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type passwordWriter struct {
	writes int
	failAt int
}

func (writer *passwordWriter) Write(data []byte) (int, error) {
	writer.writes++
	if writer.writes == writer.failAt {
		return 0, errors.New("write failed")
	}
	return len(data), nil
}

func scriptedPasswordInput(terminal bool, values ...any) passwordInput {
	index := 0
	return passwordInput{
		isTerminal: func() bool { return terminal },
		read: func() ([]byte, error) {
			value := values[index]
			index++
			if err, ok := value.(error); ok {
				return nil, err
			}
			return []byte(value.(string)), nil
		},
	}
}

func TestReadHiddenTokenRejectsNonTerminal(t *testing.T) {
	_, err := ReadHiddenToken(&bytes.Buffer{}, false)
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("ReadHiddenToken error = %v", err)
	}
}

func TestReadHiddenTokenFlowAndFailures(t *testing.T) {
	tests := []struct {
		name    string
		out     *passwordWriter
		confirm bool
		input   passwordInput
		want    string
		message string
	}{
		{name: "prompt write", out: &passwordWriter{failAt: 1}, input: scriptedPasswordInput(true, "token"), message: "prompt for token"},
		{name: "prompt newline", out: &passwordWriter{failAt: 2}, input: scriptedPasswordInput(true, "token"), message: "finish token prompt"},
		{name: "first read", out: &passwordWriter{}, input: scriptedPasswordInput(true, errors.New("read failed")), message: "read hidden token"},
		{name: "empty", out: &passwordWriter{}, input: scriptedPasswordInput(true, "   "), message: "empty token"},
		{name: "single", out: &passwordWriter{}, input: scriptedPasswordInput(true, " token "), want: "token"},
		{name: "confirm prompt", out: &passwordWriter{failAt: 3}, confirm: true, input: scriptedPasswordInput(true, "token", "token"), message: "prompt to confirm token"},
		{name: "confirm newline", out: &passwordWriter{failAt: 4}, confirm: true, input: scriptedPasswordInput(true, "token", "token"), message: "finish token confirmation"},
		{name: "confirm read", out: &passwordWriter{}, confirm: true, input: scriptedPasswordInput(true, "token", errors.New("read failed")), message: "confirm hidden token"},
		{name: "mismatch", out: &passwordWriter{}, confirm: true, input: scriptedPasswordInput(true, "one", "two"), message: "do not match"},
		{name: "confirmed", out: &passwordWriter{}, confirm: true, input: scriptedPasswordInput(true, " token ", "token"), want: "token"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := readHiddenToken(test.out, test.confirm, test.input)
			if test.message != "" {
				if err == nil || !strings.Contains(err.Error(), test.message) {
					t.Fatalf("readHiddenToken error = %v", err)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("readHiddenToken = %q, %v", got, err)
			}
		})
	}
}

func TestCurrentPasswordInputUsesProcessTerminal(t *testing.T) {
	input := currentPasswordInput()
	if input.isTerminal() {
		t.Skip("test process stdin is a terminal; refusing to consume interactive input")
	}
	if _, err := input.read(); err == nil {
		t.Fatal("non-terminal password read unexpectedly succeeded")
	}
}
