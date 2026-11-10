package terminal

import "testing"

func TestNextStepsSeparatesRunnableCommands(t *testing.T) {
	for _, test := range []struct{ next, action, detail string }{
		{"aigw check", "aigw check", ""},
		{"aigw provider balance team", "aigw provider balance team", ""},
		{"aigw use <client> team/spare", "", "aigw use <client> team/spare"},
		{"Restart Claude Desktop, then run `aigw check`", "", "Restart Claude Desktop, then run `aigw check`"},
		{"run `aigw provider rotate team`", "", "run `aigw provider rotate team`"},
		{"", "", ""},
		// A command that asks a person for a value is a step for that person.
		{"aigw setup", "", "aigw setup"},
		{"aigw setup --from team.toml", "aigw setup --from team.toml", ""},
		{"aigw setup --from team.toml --provider team", "", "aigw setup --from team.toml --provider team"},
		{"aigw setup --from team.toml --provider team --token-stdin", "aigw setup --from team.toml --provider team --token-stdin", ""},
		{"aigw provider add --help", "aigw provider add --help", ""},
		{"aigw provider edit team --balance-token-stdin --balance-user-id 7", "aigw provider edit team --balance-token-stdin --balance-user-id 7", ""},
		{"aigw provider rotate team", "", "aigw provider rotate team"},
		{"aigw provider rotate team --token-stdin", "aigw provider rotate team --token-stdin", ""},
		{"aigw provider add team --endpoint responses=https://api.example.test/v1", "", "aigw provider add team --endpoint responses=https://api.example.test/v1"},
		{"aigw provider show team", "aigw provider show team", ""},
	} {
		if action, detail := NextSteps(test.next); action != test.action || detail != test.detail {
			t.Errorf("NextSteps(%q) = %q, %q; want %q, %q", test.next, action, detail, test.action, test.detail)
		}
	}
}
