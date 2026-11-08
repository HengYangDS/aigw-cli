package native

import (
	"context"
	"os"
	"slices"
	"strings"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
)

// List returns the provider IDs of a service's items through a bounded
// credential subprocess. The worker reads item names only, and refuses
// rather than asks when the store would need a person.
func List(executable, service string) ([]string, error) {
	return listThrough(process.Runner{}, executable, service)
}

// listThrough runs the worker's listing through runner. The plan names only
// the service and carries no input.
func listThrough(runner process.CaptureRunner, executable, service string) ([]string, error) {
	if strings.TrimSpace(executable) == "" {
		return nil, ErrUnavailable
	}
	output, err := execute(context.Background(), runner, process.Plan{
		Executable: executable, Args: []string{listCommand, service}, Env: nativeEnvironment(os.Getenv),
	})
	if err != nil {
		return nil, err
	}
	return parseAccounts(output)
}

// joinAccounts is all that a listing carries across the worker's boundary:
// each name that is a provider ID, once, sorted, one per line. A name of any
// other shape, such as a staged migration item, stays in the worker.
func joinAccounts(names []string) []byte {
	var joined strings.Builder
	for _, account := range slices.Compact(slices.Sorted(slices.Values(names))) {
		if domain.ValidIdentifier(account) {
			joined.WriteString(account)
			joined.WriteByte('\n')
		}
	}
	return []byte(joined.String())
}

// parseAccounts reads a worker's listing, refusing a line that is not a
// provider ID: the worker writes nothing else.
func parseAccounts(output string) ([]string, error) {
	accounts := []string{}
	for line := range strings.SplitSeq(strings.TrimSuffix(output, "\n"), "\n") {
		if line == "" {
			continue
		}
		if !domain.ValidIdentifier(line) {
			return nil, ErrUnavailable
		}
		accounts = append(accounts, line)
	}
	return accounts, nil
}

// admittedAccount checks a worker's arguments and returns the item that they
// name: a listing names only the service and no item, and every other
// operation one exact item.
func admittedAccount(args []string, service string) (string, bool) {
	if args[0] == listCommand {
		return "", len(args) == 2 && args[1] == service
	}
	if len(args) != 3 || args[1] != service || args[2] == "" || strings.ContainsRune(args[2], 0) {
		return "", false
	}
	return args[2], true
}
