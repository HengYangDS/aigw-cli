package credential

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/secret/native"
	"aigw-cli/internal/terminal"
)

// deadlineProblem renders the problem of a credential command whose Token
// read fails with readError.
func deadlineProblem(t *testing.T, readError error) string {
	t.Helper()
	runtime, stdout := helperRuntime(t, codex.ID, true)
	runtime.Secrets = &refusingSecretStore{readError: readError}
	command := NewCommand(runtime)
	err := command.RunE(command, helperArgs(t, runtime, codex.ID))
	var diagnostic bytes.Buffer
	terminal.RenderCredentialError(terminal.NewRenderer(&diagnostic, false), err)
	if err == nil || stdout.Len() != 0 {
		t.Fatalf("a failed read returned %v and wrote %q", err, stdout)
	}
	return diagnostic.String()
}

func TestCredentialReadDeadlineHasPreciseRecovery(t *testing.T) {
	text := deadlineProblem(t, context.DeadlineExceeded)
	if !strings.Contains(text, "deadline") || !strings.Contains(text, "Check the selected credential service") || strings.Contains(text, "aigw sync") {
		t.Fatalf("read diagnostic is not specific: %q", text)
	}
}

func TestCredentialReadDeadlineShowsWhereTheOperationStood(t *testing.T) {
	for _, test := range []struct {
		name             string
		created, started time.Duration
		evidence, fix    string
		absent           string
	}{
		{"never started", 1200 * time.Millisecond, -1,
			"The native credential subprocess was created after 1.2s and was not started within 5s.",
			"Retry once this machine is less busy: the credential subprocess was not started in time", "credential service"},
		{"no answer", 200 * time.Millisecond, 300 * time.Millisecond,
			"The native credential subprocess was created after 200ms, was started after 300ms and did not answer within 5s.",
			"Check the selected credential service", "less busy"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stood := &native.DeadlineError{Limit: 5 * time.Second, Created: test.created, Started: test.started}
			text := deadlineProblem(t, fmt.Errorf("read AIGW_TOKEN/team in system keyring: %w", stood))
			for _, want := range []string{"Token read exceeded its deadline", test.evidence, test.fix, "The read was stopped; no Token was returned."} {
				if !strings.Contains(text, want) {
					t.Fatalf("deadline problem lacks %q:\n%s", want, text)
				}
			}
			if strings.Contains(text, test.absent) || strings.Contains(text, "AIGW_TOKEN") || strings.Contains(text, "aigw sync") {
				t.Fatalf("deadline problem names what it must not:\n%s", text)
			}
		})
	}
}
