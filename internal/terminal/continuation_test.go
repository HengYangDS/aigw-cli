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
	} {
		if action, detail := NextSteps(test.next); action != test.action || detail != test.detail {
			t.Errorf("NextSteps(%q) = %q, %q; want %q, %q", test.next, action, detail, test.action, test.detail)
		}
	}
}
