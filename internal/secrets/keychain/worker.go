// Package keychain confines native credential operations to a bounded, noninteractive worker.
package keychain

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"aigw-cli/internal/process"
)

const (
	workerCommand  = "__aigw-keychain-read"
	observeCommand = "__aigw-keychain-observe"
	writeCommand   = "__aigw-keychain-write"
	deleteCommand  = "__aigw-keychain-delete"
	missingExit    = 2
	deniedExit     = 3
	failureExit    = 4
	// The shared runner separately bounds pipe teardown and captures at most 64 KiB.
	operationTimeout = 5 * time.Second
	maxStoredValue   = 64 * 1024
)

var (
	// ErrNotFound means the exact credential item does not exist.
	ErrNotFound = errors.New("Keychain item not found")
	// ErrDenied means the item requires authorization unavailable without interaction.
	ErrDenied = errors.New("Keychain access requires authorization; interaction disabled")
	// ErrUnavailable means a native operation failed without returning credential data.
	ErrUnavailable = errors.New("native Keychain operation unavailable")
)

// Read returns one exact native item without permitting authentication UI.
func Read(service, account string) (string, error) {
	return invoke(workerCommand, service, account, "")
}

// Exists observes one exact item without requesting password bytes.
func Exists(service, account string) (bool, error) {
	_, err := invoke(observeCommand, service, account, "")
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// Write creates or updates one exact item using the same identity as Read.
func Write(service, account, value string) error {
	_, err := invoke(writeCommand, service, account, value)
	return err
}

// Delete removes one exact item; an already absent item is a successful no-op.
func Delete(service, account string) error {
	_, err := invoke(deleteCommand, service, account, "")
	return err
}

func invoke(operation, service, account, input string) (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", ErrUnavailable
	}
	// The native worker does not consume profiles, Tokens, PATH or loader overrides.
	return execute(context.Background(), process.Runner{}, process.Plan{
		Executable: executable, Args: []string{operation, service, account}, Stdin: input,
		Env: []string{"HOME=" + os.Getenv("HOME")},
	})
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
		case deniedExit:
			return "", ErrDenied
		}
	}
	return "", ErrUnavailable
}

// RunWorker handles private native operations before CLI initialization.
func RunWorker(args []string, input io.Reader, out io.Writer, service string) (bool, int) {
	return dispatch(args, input, out, service, queryNative)
}

// DecodeStoredValue preserves the go-keyring storage grammar without rewriting items.
// This differs from explicit stdin admission, which requires a canonical base64 envelope.
func DecodeStoredValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if encoded, ok := strings.CutPrefix(value, "go-keyring-base64:"); ok {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		return string(decoded), err
	}
	if encoded, ok := strings.CutPrefix(value, "go-keyring-encoded:"); ok {
		decoded, err := hex.DecodeString(encoded)
		return string(decoded), err
	}
	return value, nil
}

func dispatch(args []string, input io.Reader, out io.Writer, service string, query func(string, string, string, []byte) ([]byte, error)) (bool, int) {
	if len(args) == 0 {
		return false, 0
	}
	switch args[0] {
	case workerCommand, observeCommand, writeCommand, deleteCommand:
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
	case errors.Is(err, ErrDenied):
		return true, deniedExit
	case err != nil:
		return true, failureExit
	}
	if args[0] == workerCommand {
		if n, err := out.Write(value); err != nil || n != len(value) {
			return true, failureExit
		}
	}
	return true, 0
}
