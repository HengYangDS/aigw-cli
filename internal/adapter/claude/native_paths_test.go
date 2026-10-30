package claude

import (
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
)

func TestClaudeSettingsPathUsesTheOfficialUserScope(t *testing.T) {
	for _, test := range []struct {
		name string
		goos string
		env  map[string]string
		want string
	}{
		{name: "macOS", goos: "darwin", env: map[string]string{"HOME": "/Users/alex"}, want: "/Users/alex/.claude/settings.json"},
		{name: "Linux", goos: "linux", env: map[string]string{"HOME": "/home/alex"}, want: "/home/alex/.claude/settings.json"},
		{name: "Windows", goos: "windows", env: map[string]string{"USERPROFILE": `C:\Users\alex`}, want: `C:\Users\alex\.claude\settings.json`},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := SettingsPathFor(test.goos, test.env)
			if err != nil || filepath.Clean(got) != filepath.Clean(test.want) {
				t.Fatalf("SettingsPathFor() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
	for _, goos := range []string{"darwin", "linux", "windows", "plan9"} {
		if _, err := SettingsPathFor(goos, map[string]string{}); err == nil {
			t.Fatalf("SettingsPathFor(%q) accepted missing home", goos)
		}
	}
}

// The settings path keeps the Windows namespace that USERPROFILE selects.
func TestSettingsPathPreservesTheSelectedWindowsNamespace(t *testing.T) {
	for _, test := range []struct {
		name string
		base string
		want string
	}{
		{"drive", `C:\Users\alex`, `C:\Users\alex\.claude\settings.json`},
		{"UNC trailing separator", `\\server\share\`, `\\server\share\.claude\settings.json`},
		{"UNC forward slashes", `//server/share`, `\\server\share\.claude\settings.json`},
		{"extended UNC", `\\?\UNC\server\share`, `\\?\UNC\server\share\.claude\settings.json`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, err := SettingsPathFor("windows", map[string]string{"USERPROFILE": test.base}); err != nil || got != test.want {
				t.Fatalf("SettingsPathFor() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestSettingsPathFollowsTheObservedHost(t *testing.T) {
	if got, err := SettingsPath(discovery.Host{}); err != nil || got != "" {
		t.Fatalf("unobserved host settings = %q, %v; want none", got, err)
	}
	host := discovery.Host{GOOS: "windows", Env: map[string]string{"UserProfile": `C:\Users\alex`}}
	if got, err := SettingsPath(host); err != nil || got != `C:\Users\alex\.claude\settings.json` {
		t.Fatalf("Windows settings = %q, %v", got, err)
	}
	if _, err := SettingsPath(discovery.Host{GOOS: "linux"}); err == nil || err.Error() != "HOME is not set" {
		t.Fatalf("missing home error = %v", err)
	}
}

// AIGW does not start on a host where it cannot derive Claude Code's settings
// file. The check reads the environment as AIGW's own platform paths do.
func TestAdapterRefusesAHostWithoutTheSettingsInputs(t *testing.T) {
	validator, validates := New().(adapter.HostValidator)
	if !validates {
		t.Fatal("the Claude Code adapter does not validate the host")
	}
	for _, test := range []struct {
		name string
		host discovery.Host
		want string
	}{
		{name: "macOS", host: discovery.Host{GOOS: "darwin", Env: map[string]string{"HOME": "/Users/alex"}}},
		{name: "Linux without HOME", host: discovery.Host{GOOS: "linux", Home: "/home/alex", Env: map[string]string{"XDG_CONFIG_HOME": "/cfg"}}, want: "HOME is not set"},
		{name: "Windows", host: discovery.Host{GOOS: "windows", Env: map[string]string{"USERPROFILE": `C:\Users\alex`}}},
		{name: "Windows without USERPROFILE", host: discovery.Host{GOOS: "windows", Env: map[string]string{"APPDATA": `C:\Users\alex\AppData\Roaming`}}, want: "USERPROFILE is not set"},
		{name: "unsupported", host: discovery.Host{GOOS: "plan9", Env: map[string]string{"HOME": "/usr/alex"}}, want: `unsupported operating system "plan9"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validator.ValidateHost(test.host)
			if (err == nil) != (test.want == "") || err != nil && err.Error() != test.want {
				t.Fatalf("ValidateHost() = %v, want %q", err, test.want)
			}
		})
	}
}
