package setup

import (
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"

	"github.com/spf13/cobra"
)

// storingSetup is a team setup as the command line args gives it, at a
// terminal or not, without its team file read: enough for the line that
// stores a provider's Token.
func storingSetup(t *testing.T, terminal bool, args ...string) teamSetup {
	t.Helper()
	runtime := invocation.Context{}
	runtime.Interactive = terminal
	cmd := NewCommand(runtime)
	root := &cobra.Command{Use: "aigw"}
	root.AddCommand(cmd)
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	request := setupRequest{}
	var err error
	if request.From, err = cmd.Flags().GetString("from"); err != nil {
		t.Fatal(err)
	}
	if request.Providers, err = cmd.Flags().GetStringArray("provider"); err != nil {
		t.Fatal(err)
	}
	if request.TokenStdin, err = cmd.Flags().GetBool("token-stdin"); err != nil {
		t.Fatal(err)
	}
	if request.DryRun, err = cmd.Flags().GetBool("dry-run"); err != nil {
		t.Fatal(err)
	}
	if request.JSON, err = cmd.Flags().GetBool("json"); err != nil {
		t.Fatal(err)
	}
	// A JSON caller reads one document, so the setup never prompts for it.
	runtime.Interactive = terminal && !request.JSON
	return teamSetup{cmd: cmd, runtime: runtime, request: request}
}

// The setup that stores a provider's Token, once the provider's refused item
// is deleted, is this command's line as far as it does that: without
// --dry-run and --json, since a preview stores nothing; naming the provider,
// since a setup reads the Token of a provider that it names alone; and with
// the Token on standard input where the rerun neither has that input nor can
// ask. A line that then differs from the one given names its team file to
// fill in when the path would need quoting, and one that does not differ is
// the same command again, which the empty line stands for.
func TestTheLineThatStoresAProvidersTokenFollowsTheCommandLine(t *testing.T) {
	const quoted = "team files/team.toml"
	for _, test := range []struct {
		name     string
		terminal bool
		args     []string
		want     string
	}{
		{name: "named, at a terminal", terminal: true, args: []string{"--from", "team.toml", "--provider", "gateway"},
			want: "aigw setup --from team.toml --provider gateway"},
		{name: "named, with the Token on standard input", args: []string{"--from", "team.toml", "--provider", "gateway", "--token-stdin"},
			want: "aigw setup --from team.toml --provider gateway --token-stdin"},
		{name: "named, without a terminal or standard input", args: []string{"--from", "team.toml", "--provider", "gateway"},
			want: "aigw setup --from team.toml --provider gateway --token-stdin"},
		{name: "named, as json at a terminal", terminal: true, args: []string{"--from", "team.toml", "--provider", "gateway", "--json"},
			want: "aigw setup --from team.toml --provider gateway --token-stdin"},
		{name: "named among others, at a terminal", terminal: true, args: []string{"--from", "team.toml", "--provider", "other", "--provider", "gateway"},
			want: "aigw setup --from team.toml --provider other --provider gateway"},
		{name: "named among others, without a terminal", args: []string{"--from", "team.toml", "--provider", "other", "--provider", "gateway"},
			want: "aigw setup --from team.toml --provider gateway --token-stdin"},
		{name: "named, a preview at a terminal", terminal: true, args: []string{"--from", "team.toml", "--provider", "gateway", "--dry-run"},
			want: "aigw setup --from team.toml --provider gateway"},
		{name: "named, a preview with the Token on standard input", args: []string{"--from", "team.toml", "--provider", "gateway", "--token-stdin", "--dry-run"},
			want: "aigw setup --from team.toml --provider gateway --token-stdin"},

		{name: "not named, a preview at a terminal", terminal: true, args: []string{"--from", "team.toml", "--dry-run"},
			want: "aigw setup --from team.toml --provider gateway"},
		{name: "not named, a preview without a terminal", args: []string{"--from", "team.toml", "--dry-run"},
			want: "aigw setup --from team.toml --provider gateway --token-stdin"},
		{name: "not named, a preview as json at a terminal", terminal: true, args: []string{"--from", "team.toml", "--dry-run", "--json"},
			want: "aigw setup --from team.toml --provider gateway --token-stdin"},
		{name: "not named, at a terminal", terminal: true, args: []string{"--from", "team.toml"},
			want: "aigw setup --from team.toml --provider gateway"},
		{name: "not named, without a terminal", args: []string{"--from", "team.toml"},
			want: "aigw setup --from team.toml --provider gateway --token-stdin"},
		{name: "not named beside another provider, at a terminal", terminal: true, args: []string{"--from", "team.toml", "--provider", "other", "--dry-run"},
			want: "aigw setup --from team.toml --provider gateway"},
		{name: "not named beside another provider's Token on standard input", terminal: true,
			args: []string{"--from", "team.toml", "--provider", "other", "--token-stdin", "--dry-run"},
			want: "aigw setup --from team.toml --token-stdin --provider gateway"},
		{name: "not named, with a flag that the rerun keeps", terminal: true, args: []string{"--from", "team.toml", "--keep-local", "--dry-run"},
			want: "aigw setup --from team.toml --keep-local --provider gateway"},

		{name: "a path that needs quoting, the same command", terminal: true, args: []string{"--from", quoted, "--provider", "gateway"}},
		{name: "a path that needs quoting, the same command with the Token on standard input",
			args: []string{"--from", quoted, "--provider", "gateway", "--token-stdin"}},
		{name: "a path that needs quoting, named, a preview", terminal: true, args: []string{"--from", quoted, "--provider", "gateway", "--dry-run"},
			want: "aigw setup --provider gateway --from <file>"},
		{name: "a path that needs quoting, named, without a terminal", args: []string{"--from", quoted, "--provider", "gateway"},
			want: "aigw setup --from <file> --provider gateway --token-stdin"},
		{name: "a path that needs quoting, not named, at a terminal", terminal: true, args: []string{"--from", quoted, "--dry-run"},
			want: "aigw setup --from <file> --provider gateway"},
		{name: "a path that needs quoting, not named, without a terminal", args: []string{"--from", quoted, "--dry-run"},
			want: "aigw setup --from <file> --provider gateway --token-stdin"},
		{name: "a path that needs quoting, not named, applied at a terminal", terminal: true, args: []string{"--from", quoted},
			want: "aigw setup --from <file> --provider gateway"},
	} {
		team := storingSetup(t, test.terminal, test.args...)
		got := team.storingLine("gateway")
		if got != test.want {
			t.Errorf("%s: the line is %q, want %q", test.name, got, test.want)
		}
		// Whatever the line, a setup that it names reads the Token of the
		// provider: it names the provider, and never a preview or a document.
		if got != "" && (!strings.Contains(got, "--provider gateway") || strings.Contains(got, "--dry-run") || strings.Contains(got, "--json")) {
			t.Errorf("%s: the line %q stores no Token of the provider", test.name, got)
		}
	}
}
