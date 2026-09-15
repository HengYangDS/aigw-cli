// Package keychain confines native password reads to a bounded, noninteractive worker.
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
	missingExit    = 2
	deniedExit     = 3
	failureExit    = 4
	// Reads get five seconds; the shared runner separately bounds pipe teardown.
	readTimeout = 5 * time.Second
)

var (
	// ErrNotFound means the exact credential item does not exist.
	ErrNotFound = errors.New("Keychain item not found")
	// ErrDenied means the item requires authorization unavailable without interaction.
	ErrDenied = errors.New("Keychain access requires authorization; interaction disabled")
	// ErrUnavailable means a native read failed without returning credential data.
	ErrUnavailable = errors.New("native Keychain read unavailable")
)

// Read returns one exact native item without permitting authentication UI.
func Read(service, account string) (string, error) {
	return invoke(workerCommand, service, account)
}

// Exists observes one exact item without requesting password bytes.
func Exists(service, account string) (bool, error) {
	_, err := invoke(observeCommand, service, account)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func invoke(operation, service, account string) (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", ErrUnavailable
	}
	// The native worker does not consume profiles, Tokens, PATH or loader overrides.
	return read(context.Background(), process.Runner{}, executable, operation, service, account, []string{"HOME=" + os.Getenv("HOME")})
}

func read(parent context.Context, runner process.CaptureRunner, executable, operation, service, account string, env []string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, readTimeout)
	defer cancel()
	output, err := runner.RunCapture(ctx, process.Plan{
		Executable: executable, Args: []string{operation, service, account}, Env: env,
	})
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

// RunWorker handles only the private native read entrypoint, before CLI initialization.
func RunWorker(args []string, out io.Writer, service string) (bool, int) {
	return dispatch(args, out, service, queryNative)
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

func dispatch(args []string, out io.Writer, service string, query func(string, string, bool) ([]byte, error)) (bool, int) {
	if len(args) == 0 || args[0] != workerCommand && args[0] != observeCommand {
		return false, 0
	}
	if len(args) != 3 || args[1] != service || args[2] == "" || strings.ContainsRune(args[2], 0) {
		return true, failureExit
	}
	value, err := query(service, args[2], args[0] == observeCommand)
	defer clear(value)
	switch {
	case errors.Is(err, ErrNotFound):
		return true, missingExit
	case errors.Is(err, ErrDenied):
		return true, deniedExit
	case err != nil:
		return true, failureExit
	}
	if n, err := out.Write(value); err != nil || n != len(value) {
		return true, failureExit
	}
	return true, 0
}
