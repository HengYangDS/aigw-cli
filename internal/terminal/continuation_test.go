package terminal

import "testing"

func TestNextStepsSeparatesRunnableCommands(t *testing.T) {
	for _, test := range []struct{ next, action, detail string }{
		{"aigw check", "aigw check", ""},
		{"aigw balance team", "aigw balance team", ""},
		{"aigw use --for <client> spare", "", "aigw use --for <client> spare"},
		{"Restart Claude Desktop, then run `aigw check`", "", "Restart Claude Desktop, then run `aigw check`"},
		{"run `aigw rotate team`", "", "run `aigw rotate team`"},
		{"", "", ""},
		// A command that asks a person for a value is a step for that person.
		{"aigw setup", "", "aigw setup"},
		{"aigw setup --from team.toml", "aigw setup --from team.toml", ""},
		{"aigw setup --from team.toml --account team", "", "aigw setup --from team.toml --account team"},
		{"aigw setup --account team --token-stdin", "aigw setup --account team --token-stdin", ""},
		// Without --account, setup connects an Account named after its Route.
		{"aigw setup --route team --for claude --anthropic-url https://x.test", "", "aigw setup --route team --for claude --anthropic-url https://x.test"},
		{"aigw setup --route team --for claude --token-stdin", "aigw setup --route team --for claude --token-stdin", ""},
		{"aigw rotate team", "", "aigw rotate team"},
		{"aigw rotate team --token-stdin", "aigw rotate team --token-stdin", ""},
		{"aigw add team --openai-url https://api.example.test/v1", "", "aigw add team --openai-url https://api.example.test/v1"},
		{"aigw account diagnostics enable team", "", "aigw account diagnostics enable team"},
		{"aigw account diagnostics enable team --system-token-stdin --user-id 7", "aigw account diagnostics enable team --system-token-stdin --user-id 7", ""},
		{"aigw account diagnostics disable team", "aigw account diagnostics disable team", ""},
		{"aigw add --help", "aigw add --help", ""},
		{"aigw rotate --help", "aigw rotate --help", ""},
		{"aigw add", "", "aigw add"},
	} {
		if action, detail := NextSteps(test.next); action != test.action || detail != test.detail {
			t.Errorf("NextSteps(%q) = %q, %q; want %q, %q", test.next, action, detail, test.action, test.detail)
		}
	}
}
