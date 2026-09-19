// Package native confines operating-system credential operations to bounded subprocesses.
package native

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"aigw-cli/internal/process"
)

const (
	readCommand   = "__aigw-native-credential-read"
	writeCommand  = "__aigw-native-credential-write"
	deleteCommand = "__aigw-native-credential-delete"
	existsCommand = "__aigw-native-credential-exists"
	missingExit   = 2
	failureExit   = 3
	// The shared runner separately bounds pipe teardown and captures at most 64 KiB.
	operationTimeout = 5 * time.Second
	maxStoredValue   = 64 * 1024
)

var (
	// ErrNotFound means the exact credential item does not exist.
	ErrNotFound = errors.New("native credential item not found")
	// ErrUnavailable means a native operation failed without returning credential data.
	ErrUnavailable = errors.New("native credential operation unavailable")
)

// Read returns one exact native item through a bounded credential subprocess.
// Native authorization UI remains controlled by the operating system.
func Read(executable, service, account string) (string, error) {
	return invoke(executable, readCommand, service, account, "")
}

// Write creates or updates one exact item through a bounded credential subprocess.
func Write(executable, service, account, value string) error {
	_, err := invoke(executable, writeCommand, service, account, value)
	return err
}

// Delete removes one exact item; an already absent item is a successful no-op.
func Delete(executable, service, account string) error {
	_, err := invoke(executable, deleteCommand, service, account, "")
	return err
}

// Exists observes one exact native item without returning its value.
func Exists(executable, service, account string) (bool, error) {
	output, err := invoke(executable, existsCommand, service, account, "")
	if err != nil {
		return false, err
	}
	switch output {
	case "1":
		return true, nil
	case "0":
		return false, nil
	default:
		return false, ErrUnavailable
	}
}

func invoke(executable, operation, service, account, input string) (string, error) {
	if strings.TrimSpace(executable) == "" {
		return "", ErrUnavailable
	}
	// The native worker does not consume profiles, Tokens, PATH or loader overrides.
	return execute(context.Background(), process.Runner{}, process.Plan{
		Executable: executable, Args: []string{operation, service, account}, Stdin: input,
		Env: nativeEnvironment(os.Getenv),
	})
}

func retainedEnvironment(getenv func(string) string, names ...string) []string {
	environment := make([]string, 0, len(names))
	for _, name := range names {
		if value := getenv(name); value != "" {
			environment = append(environment, name+"="+value)
		}
	}
	return environment
}

func execute(parent context.Context, runner process.CaptureRunner, plan process.Plan) (string, error) {
	if len(plan.Stdin) > maxStoredValue {
		return "", ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(parent, operationTimeout)
	defer cancel()
	output, err := runner.RunCapture(ctx, plan)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err == nil {
		return string(output), nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		switch exit.ExitCode() {
		case missingExit:
			return "", ErrNotFound
		}
	}
	return "", ErrUnavailable
}

// RunCredentialSubprocess handles hidden native credential operations before CLI initialization.
func RunCredentialSubprocess(args []string, input io.Reader, out io.Writer, service string) (bool, int) {
	return dispatch(args, input, out, service, queryCredential)
}

func dispatch(args []string, input io.Reader, out io.Writer, service string, query func(string, string, string, []byte) ([]byte, error)) (bool, int) {
	if len(args) == 0 {
		return false, 0
	}
	switch args[0] {
	case readCommand, writeCommand, deleteCommand, existsCommand:
	default:
		return false, 0
	}
	if len(args) != 3 || args[1] != service || args[2] == "" || strings.ContainsRune(args[2], 0) {
		return true, failureExit
	}
	var data []byte
	if args[0] == writeCommand {
		var err error
		data, err = io.ReadAll(io.LimitReader(input, maxStoredValue+1))
		defer clear(data)
		if err != nil || len(data) == 0 || len(data) > maxStoredValue {
			return true, failureExit
		}
	}
	value, err := query(args[0], service, args[2], data)
	defer clear(value)
	switch {
	case errors.Is(err, ErrNotFound):
		return true, missingExit
	case err != nil:
		return true, failureExit
	}
	if args[0] == readCommand || args[0] == existsCommand {
		if n, err := out.Write(value); err != nil || n != len(value) {
			return true, failureExit
		}
	}
	return true, 0
}
