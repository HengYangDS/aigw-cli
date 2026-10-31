package terminal

import "testing"

// filepath.Rel fails for paths on different volumes, so these cases need
// Windows path semantics: before the volume rule, every one of them reported a
// location that is not known to be outside home.
func TestHomeTargetOnAnotherVolume(t *testing.T) {
	home := `C:\Users\operator`
	for _, test := range []struct {
		path, home, target string
		outside            bool
	}{
		{`D:\work\codex\config.toml`, home, "", true},
		{`\\server\share\codex\config.toml`, home, "", true},
		{`c:\Users\operator\.codex\config.toml`, home, "~/.codex/config.toml", false},
		{`D:\work\codex\config.toml`, `Users\operator`, "", false},
	} {
		target, outside := HomeTarget(test.path, test.home)
		if target != test.target || outside != test.outside {
			t.Errorf("HomeTarget(%q, %q) = %q, %v; want %q, %v", test.path, test.home, target, outside, test.target, test.outside)
		}
	}
}
