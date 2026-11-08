package native

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/platform/process"
)

// The worker's listing takes the service alone and no input, and only
// provider IDs cross its boundary: a name of any other shape, such as a staged
// migration item, or anything a backend might wrongly hand back with a space
// or a line of its own, stays in the worker.
func TestCredentialSubprocessListingReturnsOnlyProviderIDs(t *testing.T) {
	const value = "sk secret value"
	var out, diagnostic bytes.Buffer
	handled, code := dispatch([]string{listCommand, "AIGW_TOKEN"}, strings.NewReader("must not be read"), &out, &diagnostic, "AIGW_TOKEN",
		func(operation, service, account string, data []byte) ([]byte, error) {
			if operation != listCommand || service != "AIGW_TOKEN" || account != "" || len(data) != 0 {
				t.Fatalf("listing request = %q %q %q %q", operation, service, account, data)
			}
			return []byte(strings.Join([]string{"team", "migration@team", "diagnostic@team", value, "orphan", "team", ""}, "\n")), nil
		})
	if !handled || code != 0 || out.String() != "orphan\nteam\n" || diagnostic.Len() != 0 {
		t.Fatalf("listing = handled %t, code %d, out %q, diagnostic %q", handled, code, &out, &diagnostic)
	}
}

// A listing names a service and nothing else: an item, a foreign service and
// extra arguments are refused before the native store is reached.
func TestCredentialSubprocessListingRestrictsItsArguments(t *testing.T) {
	for _, args := range [][]string{
		{listCommand}, {listCommand, "foreign-service"}, {listCommand, "AIGW_TOKEN", "team"},
	} {
		var out bytes.Buffer
		handled, code := dispatch(args, strings.NewReader(""), &out, io.Discard, "AIGW_TOKEN", func(string, string, string, []byte) ([]byte, error) {
			t.Fatal("invalid listing reached the native credential service")
			return nil, nil
		})
		if !handled || code != failureExit || out.Len() != 0 {
			t.Fatalf("listing %q = %v, %d, %q", args, handled, code, &out)
		}
	}
}

// A store that refuses reports its typed status and lists nothing.
func TestCredentialSubprocessListingReportsARefusal(t *testing.T) {
	var out, diagnostic bytes.Buffer
	handled, code := dispatch([]string{listCommand, "AIGW_TOKEN"}, strings.NewReader(""), &out, &diagnostic, "AIGW_TOKEN",
		func(string, string, string, []byte) ([]byte, error) { return nil, &KeychainError{Status: -25308} })
	if !handled || code != failureExit || out.Len() != 0 || !strings.Contains(diagnostic.String(), "keychain_status") {
		t.Fatalf("refusal = %v, %d, %q, %q", handled, code, &out, &diagnostic)
	}
}

type listingRunner struct {
	output string
	err    error
	plan   *process.Plan
}

func (r listingRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	*r.plan = plan
	return []byte(r.output), r.err
}

// The parent names only the service, sends no input, and accepts nothing but
// provider IDs back.
func TestListAcceptsOnlyProviderIDsFromItsWorker(t *testing.T) {
	var plan process.Plan
	accounts, err := listThrough(listingRunner{output: "orphan\nteam\n", plan: &plan}, "/owned/aigw", "AIGW_TOKEN")
	if err != nil || !slices.Equal(accounts, []string{"orphan", "team"}) {
		t.Fatalf("listing = %q, %v", accounts, err)
	}
	if !slices.Equal(plan.Args, []string{listCommand, "AIGW_TOKEN"}) || plan.Stdin != "" {
		t.Fatalf("listing plan = %q, stdin %q", plan.Args, plan.Stdin)
	}
	if accounts, err := listThrough(listingRunner{plan: &plan}, "/owned/aigw", "AIGW_TOKEN"); err != nil || len(accounts) != 0 {
		t.Fatalf("an empty listing = %q, %v", accounts, err)
	}
	if accounts, err := listThrough(listingRunner{output: "team\nsk secret value\n", plan: &plan}, "/owned/aigw", "AIGW_TOKEN"); !errors.Is(err, ErrUnavailable) || accounts != nil {
		t.Fatalf("a line that is no provider ID = %q, %v", accounts, err)
	}
	if accounts, err := List("", "AIGW_TOKEN"); !errors.Is(err, ErrUnavailable) || accounts != nil {
		t.Fatalf("a listing without an executable = %q, %v", accounts, err)
	}
}
