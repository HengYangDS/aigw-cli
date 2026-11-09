package invocation

import (
	"errors"
	"net/http"
	"testing"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// TestTokenValidationProblemNamesAFixThatRuns shows each fix: a rejected Token
// pipes a Token that the provider accepts into the same command, or enters it
// at the command's prompt, except from the environment backend, whose
// variable must change first; a status that a later request may not repeat
// reruns the command; another status corrects the endpoint.
func TestTokenValidationProblemNamesAFixThatRuns(t *testing.T) {
	command := func(args ...string) *cobra.Command {
		cmd := &cobra.Command{Use: "add", RunE: func(*cobra.Command, []string) error { return nil }}
		cmd.Flags().Bool("token-stdin", false, "")
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		return cmd
	}
	environment := secret.NewEnvironmentStore(func(key string) string {
		if key == "AIGW_TOKEN_TEAM" {
			return "rejected"
		}
		return ""
	})
	rejected := &probe.TokenRejectedError{Label: "Claude", Status: http.StatusUnauthorized}
	endpoint := func(protocol domain.EndpointProtocol) string { return "edit " + protocol.Name() }
	for _, test := range []struct {
		name    string
		cmd     *cobra.Command
		args    []string
		secrets secret.Store
		err     error
		fix     string
		impact  string
	}{
		{"rejected", command("team", "--token-stdin"), []string{"team"}, secret.NewMemoryStore(), rejected,
			"Pipe a Token that provider team accepts into `add team --token-stdin`.", "No Token was stored."},
		{"rejected at a prompt", command("team"), []string{"team"}, secret.NewMemoryStore(), rejected,
			"Run `add team` again and enter a Token that provider team accepts.", "No Token was stored."},
		{"rejected, needing quotes", command("two words", "--token-stdin"), []string{"two words"}, secret.NewMemoryStore(), rejected,
			"Pipe a Token that provider team accepts into the same command.", "No Token was stored."},
		{"rejected from the environment", command("team"), []string{"team"}, environment, rejected,
			"Set AIGW_TOKEN_TEAM to a Token that the provider accepts, then run `add team`.", "Nothing changed."},
		{"rejected from the environment, needing quotes", command("two words"), []string{"two words"}, environment, rejected,
			"Set AIGW_TOKEN_TEAM to a Token that the provider accepts, then run the same command again.", "Nothing changed."},
		{"rate limit", command("team"), []string{"team"}, secret.NewMemoryStore(), &probe.EndpointStatusError{Status: http.StatusTooManyRequests}, "add team", "No Token was stored."},
		{"wrong endpoint", command("team"), []string{"team"}, secret.NewMemoryStore(),
			&probe.EndpointStatusError{Status: http.StatusNotFound, Protocol: domain.ProtocolAnthropic}, "edit anthropic", "No Token was stored."},
		{"another failure", command("team"), []string{"team"}, secret.NewMemoryStore(), errors.New("broken"), "aigw check", "No Token was stored."},
	} {
		p := terminal.ProblemFor(TokenValidationProblem(test.cmd, test.args, test.secrets, "team", endpoint, test.err))
		if p.Fix != test.fix || p.Impact != test.impact {
			t.Errorf("%s: fix %q, impact %q", test.name, p.Fix, p.Impact)
		}
	}
}
