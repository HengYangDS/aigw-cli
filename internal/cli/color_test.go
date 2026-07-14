package cli

import "testing"

func TestColorEnabledFromEnvironment(t *testing.T) {
	tests := []struct {
		name string
		goos string
		env  map[string]string
		term bool
		want bool
	}{
		{"non terminal", "linux", nil, false, false},
		{"NO_COLOR", "linux", map[string]string{"NO_COLOR": "1"}, true, false},
		{"unix terminal", "linux", nil, true, true},
		{"windows legacy ConsoleHost", "windows", map[string]string{"WT_SESSION": "", "ANSICON": "", "ConEmuANSI": "OFF", "TERM": ""}, true, false},
		{"windows VT enabled", "windows", nil, true, true},
		{"windows terminal", "windows", map[string]string{"WT_SESSION": "session"}, true, true},
		{"windows ANSI capable host", "windows", map[string]string{"ANSICON": "1"}, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enableVT := func() bool { return false }
			if tt.name == "windows VT enabled" {
				enableVT = func() bool { return true }
			}
			if got := colorEnabled(tt.goos, tt.env, tt.term, enableVT); got != tt.want {
				t.Fatalf("colorEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
