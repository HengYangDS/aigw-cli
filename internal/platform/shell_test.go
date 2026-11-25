package platform

import (
	"testing"
)

func TestAIGWCommandNamesOnlyWordsThatNeedNoQuoting(t *testing.T) {
	for _, test := range []struct {
		words []string
		want  string
	}{
		{[]string{"use", "codex", "gateway/gpt-6.2-sol"}, "aigw use codex gateway/gpt-6.2-sol"},
		{[]string{"model", "show", "team/model:latest@2"}, "aigw model show team/model:latest@2"},
		{[]string{"model", "show", "two words"}, ""},
		{[]string{"use", "codex", "gateway/gpt sol[1m]"}, ""},
		{[]string{"use", "claude", "team/opus[1m]"}, ""},
		{[]string{"use", "codex", "--unlisted"}, ""},
		{[]string{"client", "edit", "codex", "--target=~/x"}, ""},
		{[]string{"use", "codex", "team/a;b"}, ""},
		{[]string{"use", "codex", ""}, ""},
	} {
		if got := AIGWCommand(test.words...); got != test.want {
			t.Errorf("AIGWCommand(%q) = %q, want %q", test.words, got, test.want)
		}
	}
	// A backslash is an escape in the shells; a percent sign is passed through.
	if PlainWord(`a\b`) || !PlainWord("100%") {
		t.Errorf("platform words: backslash plain = %t, percent plain = %t", PlainWord(`a\b`), PlainWord("100%"))
	}
}
