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

func scriptedPasswordInput(readError error, values ...string) passwordInput {
	index := 0
	return passwordInput{
		isTerminal: func() bool { return true },
		read: func() ([]byte, error) {
			if readError != nil && index == len(values) {
				return nil, readError
			}
			value := values[index]
			index++
			return []byte(value), nil
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
		{name: "prompt write", out: &passwordWriter{failAt: 1}, input: scriptedPasswordInput(nil, "token"), message: "prompt for token"},
		{name: "prompt newline", out: &passwordWriter{failAt: 2}, input: scriptedPasswordInput(nil, "token"), message: "finish token prompt"},
		{name: "first read", out: &passwordWriter{}, input: scriptedPasswordInput(errors.New("read failed")), message: "read hidden token"},
		{name: "empty", out: &passwordWriter{}, input: scriptedPasswordInput(nil, "   "), message: "empty token"},
		{name: "single", out: &passwordWriter{}, input: scriptedPasswordInput(nil, " token "), want: "token"},
		{name: "confirm prompt", out: &passwordWriter{failAt: 3}, confirm: true, input: scriptedPasswordInput(nil, "token", "token"), message: "prompt to confirm token"},
		{name: "confirm newline", out: &passwordWriter{failAt: 4}, confirm: true, input: scriptedPasswordInput(nil, "token", "token"), message: "finish token confirmation"},
		{name: "confirm read", out: &passwordWriter{}, confirm: true, input: scriptedPasswordInput(errors.New("read failed"), "token"), message: "confirm hidden token"},
		{name: "mismatch", out: &passwordWriter{}, confirm: true, input: scriptedPasswordInput(nil, "one", "two"), message: "do not match"},
		{name: "confirmed", out: &passwordWriter{}, confirm: true, input: scriptedPasswordInput(nil, " token ", "token"), want: "token"},
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
