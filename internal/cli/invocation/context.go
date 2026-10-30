// Package invocation defines the capabilities available to one CLI invocation
// and the CLI behavior that several commands share: Token input, rendering,
// problems, and the rename subcommands' arguments, selection and plan output.
package invocation

import (
	"fmt"
	"io"
	"strings"

	"aigw-cli/internal/app"
	"aigw-cli/internal/terminal"
)

// Prompter supplies interactive secret, text, and bounded-choice input.
type Prompter interface {
	Secret(label string) (string, error)
	Text(label string) (string, error)
	Select(label string, choices []terminal.Choice) (string, error)
}

// Context carries capabilities for one command execution without
// product-global state: the use cases' Dependencies and the terminal that
// this invocation reads from and renders to.
type Context struct {
	app.Dependencies
	In          io.Reader
	Out         io.Writer
	Color       bool
	Width       int
	Interactive bool
	Prompt      Prompter
	RenderOut   io.Writer
	Problem     func(title, evidence, impact, fix string, cause error) error
}

// ReadToken reads an explicitly requested token source without consulting
// process-global input or environment state.
func ReadToken(runtime Context, stdinMode bool, confirm bool) (string, error) {
	if !stdinMode {
		if !runtime.Interactive {
			return "", fmt.Errorf("Token input requires an interactive terminal; pipe it to `aigw` with --token-stdin")
		}
		return terminal.ReadHiddenToken(runtime.Out, confirm)
	}
	if runtime.In == nil {
		return "", fmt.Errorf("token input is unavailable")
	}
	const inputLimit = 64 * 1024
	data, err := io.ReadAll(io.LimitReader(runtime.In, inputLimit+1))
	if err != nil {
		return "", fmt.Errorf("Failed to read token from standard input: %w", err)
	}
	if len(data) > inputLimit {
		return "", fmt.Errorf("Token input exceeds %d bytes", inputLimit)
	}
	value := strings.TrimSuffix(string(data), "\n")
	if len(value) != len(data) {
		value = strings.TrimSuffix(value, "\r")
	}
	if err := ValidateToken(value); err != nil {
		return "", err
	}
	return value, nil
}

// ValidateToken checks an explicit API Token without normalizing its value or exposing it.
func ValidateToken(value string) error {
	if value == "" {
		return fmt.Errorf("Empty tokens are not accepted")
	}
	for _, character := range value {
		if character < '!' || character > '~' {
			return fmt.Errorf("Token input must contain one visible ASCII value")
		}
	}
	return nil
}

// Renderer creates the presentation projection bound to this invocation.
func Renderer(runtime Context) *terminal.Renderer {
	out := runtime.RenderOut
	if out == nil {
		out = runtime.Out
	}
	if out == nil {
		out = io.Discard
	}
	return terminal.NewRendererWithWidth(out, runtime.Color, runtime.Width)
}

// Problem preserves a structured user-facing error when the composition root supplies one.
func Problem(runtime Context, title, evidence, impact, fix string, cause error) error {
	if runtime.Problem != nil {
		return runtime.Problem(title, evidence, impact, fix, cause)
	}
	return cause
}
