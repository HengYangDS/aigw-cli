package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type command struct {
	Name  string
	Args  []string
	Env   []string
	Dir   string
	Input string
}

const commandDiagnosticLimit = 64 * 1024

type diagnosticCapture struct {
	output    bytes.Buffer
	truncated bool
}

func (capture *diagnosticCapture) Write(data []byte) (int, error) {
	remaining := commandDiagnosticLimit - capture.output.Len()
	if remaining > 0 {
		_, _ = capture.output.Write(data[:min(len(data), remaining)])
	}
	capture.truncated = capture.truncated || len(data) > remaining
	return len(data), nil
}

func (capture *diagnosticCapture) text() string {
	diagnostics := strings.TrimSpace(capture.output.String())
	if capture.truncated {
		return diagnostics + "\n[diagnostics truncated]"
	}
	return diagnostics
}

type commandRunner func(command) error

type outputRunner func(command) ([]byte, error)

func systemOutputRunner(call command) ([]byte, error) {
	process := exec.Command(call.Name, call.Args...)
	process.Dir = call.Dir
	process.Env = append(process.Environ(), call.Env...)
	process.Stdin = strings.NewReader(call.Input)
	return process.CombinedOutput()
}

func runCommands(commands []command, stdout io.Writer, runner commandRunner) error {
	for _, call := range commands {
		if _, err := fmt.Fprintf(stdout, "==> %s\n", call.Name); err != nil {
			return fmt.Errorf("report gate %s before execution: %w", call.Name, err)
		}
		if err := runner(call); err != nil {
			return fmt.Errorf("%s: %w", call.Name, err)
		}
	}
	return nil
}

func systemRunner(call command) error {
	process := exec.Command(call.Name, call.Args...)
	process.Dir = call.Dir
	process.Env = append(process.Environ(), call.Env...)
	process.Stdin = strings.NewReader(call.Input)
	process.Stdout = os.Stdout
	var failureOutput diagnosticCapture
	process.Stderr = io.MultiWriter(os.Stderr, &failureOutput)
	if err := process.Run(); err != nil {
		if text := failureOutput.text(); text != "" {
			return fmt.Errorf("%w: %s", err, text)
		}
		return err
	}
	return nil
}
