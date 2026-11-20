package platform

import (
	"runtime"
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
	// A backslash is a path separator in Windows shells and an escape in the
	// others; a percent sign expands in Windows shells only.
	windows := runtime.GOOS == "windows"
	if PlainWord(`C:\Users\x`) != windows || PlainWord("100%") == windows {
		t.Errorf("platform words: backslash plain = %t, percent plain = %t on %s", PlainWord(`C:\Users\x`), PlainWord("100%"), runtime.GOOS)
	}
}
