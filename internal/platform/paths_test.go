package platform_test

import (
	"path/filepath"
	"testing"

	"aigw-cli/internal/platform"
)

func TestHostPathsAreDerivedAsOnePlatformContract(t *testing.T) {
	tests := []struct {
		name        string
		goos        string
		env         map[string]string
		config      string
		data        string
		claude      string
		installDir  string
		installName string
	}{
		{name: "macOS", goos: "darwin", env: map[string]string{"HOME": "/Users/alex"}, config: "/Users/alex/Library/Application Support/aigw/config.toml", data: "/Users/alex/Library/Application Support/aigw", claude: "/Users/alex/.claude/settings.json", installDir: "/Users/alex/.local/bin", installName: "aigw"},
		{name: "Linux", goos: "linux", env: map[string]string{"HOME": "/home/alex", "XDG_CONFIG_HOME": "/cfg", "XDG_DATA_HOME": "/data"}, config: "/cfg/aigw/config.toml", data: "/data/aigw", claude: "/home/alex/.claude/settings.json", installDir: "/home/alex/.local/bin", installName: "aigw"},
		{name: "Windows", goos: "windows", env: map[string]string{"APPDATA": `C:\Users\alex\AppData\Roaming`, "LOCALAPPDATA": `C:\Users\alex\AppData\Local`, "USERPROFILE": `C:\Users\alex`}, config: `C:\Users\alex\AppData\Roaming\aigw\config.toml`, data: `C:\Users\alex\AppData\Local\aigw`, claude: `C:\Users\alex\.claude\settings.json`, installDir: `C:\Users\alex\AppData\Local\Programs\aigw\bin`, installName: "aigw.exe"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := platform.PathsFor(test.goos, test.env)
			if err != nil {
				t.Fatal(err)
			}
			if got.Config != test.config || got.Data != test.data || got.ClaudeSettings != test.claude || got.InstallDir != test.installDir || got.InstallName != test.installName {
				t.Fatalf("PathsFor() = %#v", got)
			}
		})
	}
}

func TestHostPathContractReportsTheFirstMissingPlatformInput(t *testing.T) {
	for _, test := range []struct {
		name string
		goos string
		env  map[string]string
		want string
	}{
		{name: "config", goos: "darwin", env: map[string]string{}, want: "HOME is not set"},
		{name: "data", goos: "linux", env: map[string]string{"XDG_CONFIG_HOME": "/cfg"}, want: "HOME and XDG_DATA_HOME are not set"},
		{name: "Claude", goos: "linux", env: map[string]string{"XDG_CONFIG_HOME": "/cfg", "XDG_DATA_HOME": "/data"}, want: "HOME is not set"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := platform.PathsFor(test.goos, test.env); err == nil || err.Error() != test.want {
				t.Fatalf("PathsFor() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestConfigPathUsesPlatformConvention(t *testing.T) {
	tests := []struct {
		goos string
		env  map[string]string
		want string
	}{
		{"darwin", map[string]string{"HOME": "/Users/alex"}, "/Users/alex/Library/Application Support/aigw/config.toml"},
		{"linux", map[string]string{"HOME": "/home/alex"}, "/home/alex/.config/aigw/config.toml"},
		{"linux", map[string]string{"HOME": "/home/alex", "XDG_CONFIG_HOME": "/cfg"}, "/cfg/aigw/config.toml"},
		{"windows", map[string]string{"APPDATA": `C:\Users\alex\AppData\Roaming`}, `C:\Users\alex\AppData\Roaming\aigw\config.toml`},
	}
	for _, tt := range tests {
		got, err := platform.ConfigPathFor(tt.goos, tt.env)
		if err != nil || filepath.Clean(got) != filepath.Clean(tt.want) {
			t.Errorf("ConfigPathFor(%s) = %q, %v; want %q", tt.goos, got, err, tt.want)
		}
	}
}

func TestConfigPathRefusesMissingHomeOrUnsupportedOS(t *testing.T) {
	if _, err := platform.ConfigPathFor("darwin", map[string]string{}); err == nil {
		t.Fatal("darwin without HOME unexpectedly admitted")
	}
	if _, err := platform.ConfigPathFor("linux", map[string]string{}); err == nil {
		t.Fatal("linux without HOME or XDG_CONFIG_HOME unexpectedly admitted")
	}
	if _, err := platform.ConfigPathFor("windows", map[string]string{}); err == nil {
		t.Fatal("windows without APPDATA unexpectedly admitted")
	}
	if _, err := platform.ConfigPathFor("plan9", map[string]string{}); err == nil {
		t.Fatal("unsupported operating system unexpectedly admitted")
	}
}

func TestDataDirUsesPlatformConvention(t *testing.T) {
	tests := []struct {
		goos string
		env  map[string]string
		want string
	}{
		{"darwin", map[string]string{"HOME": "/Users/alex"}, "/Users/alex/Library/Application Support/aigw"},
		{"linux", map[string]string{"HOME": "/home/alex"}, "/home/alex/.local/share/aigw"},
		{"linux", map[string]string{"HOME": "/home/alex", "XDG_DATA_HOME": "/data"}, "/data/aigw"},
		{"windows", map[string]string{"LOCALAPPDATA": `C:\Users\alex\AppData\Local`}, `C:\Users\alex\AppData\Local\aigw`},
		{"windows", map[string]string{"APPDATA": `C:\Users\alex\AppData\Roaming`}, `C:\Users\alex\AppData\Roaming\aigw`},
	}
	for _, tt := range tests {
		got, err := platform.DataDirFor(tt.goos, tt.env)
		if err != nil || filepath.Clean(got) != filepath.Clean(tt.want) {
			t.Errorf("DataDirFor(%s, %v) = %q, %v; want %q", tt.goos, tt.env, got, err, tt.want)
		}
	}
}

func TestDataDirRefusesMissingHomeOrUnsupportedOS(t *testing.T) {
	if _, err := platform.DataDirFor("darwin", map[string]string{}); err == nil {
		t.Fatal("darwin without HOME unexpectedly admitted")
	}
	if _, err := platform.DataDirFor("linux", map[string]string{}); err == nil {
		t.Fatal("linux without HOME or XDG_DATA_HOME unexpectedly admitted")
	}
	if _, err := platform.DataDirFor("windows", map[string]string{}); err == nil {
		t.Fatal("windows without LOCALAPPDATA or APPDATA unexpectedly admitted")
	}
	if _, err := platform.DataDirFor("plan9", map[string]string{}); err == nil {
		t.Fatal("unsupported operating system unexpectedly admitted")
	}
}

func TestUserBinDirUsesWritableUserBoundary(t *testing.T) {
	tests := []struct {
		goos string
		env  map[string]string
		want string
	}{
		{"darwin", map[string]string{"HOME": "/Users/alex"}, "/Users/alex/.local/bin"},
		{"linux", map[string]string{"HOME": "/home/alex"}, "/home/alex/.local/bin"},
		{"windows", map[string]string{"LOCALAPPDATA": `C:\Users\alex\AppData\Local`}, `C:\Users\alex\AppData\Local\Programs\aigw\bin`},
	}
	for _, tt := range tests {
		got, err := platform.UserBinDirFor(tt.goos, tt.env)
		if err != nil || filepath.Clean(got) != filepath.Clean(tt.want) {
			t.Errorf("UserBinDirFor(%s) = %q, %v; want %q", tt.goos, got, err, tt.want)
		}
	}
}

func TestUserBinDirRefusesMissingHome(t *testing.T) {
	if _, err := platform.UserBinDirFor("linux", map[string]string{}); err == nil {
		t.Fatal("expected missing HOME to fail")
	}
}

func TestUserBinDirFallsBackFromLocalAppDataToAppData(t *testing.T) {
	got, err := platform.UserBinDirFor("windows", map[string]string{"APPDATA": `C:\Users\alex\AppData\Roaming`})
	want := `C:\Users\alex\AppData\Roaming\Programs\aigw\bin`
	if err != nil || filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("UserBinDirFor(windows) = %q, %v; want %q", got, err, want)
	}
}

func TestUserBinDirRefusesMissingWindowsEnvOrUnsupportedOS(t *testing.T) {
	if _, err := platform.UserBinDirFor("windows", map[string]string{}); err == nil {
		t.Fatal("windows without LOCALAPPDATA or APPDATA unexpectedly admitted")
	}
	if _, err := platform.UserBinDirFor("plan9", map[string]string{}); err == nil {
		t.Fatal("unsupported operating system unexpectedly admitted")
	}
}

func TestWindowsJoinPreservesRootedAndEmptyInputs(t *testing.T) {
	got, err := platform.ConfigPathFor("windows", map[string]string{"APPDATA": `\\server\share`})
	if err != nil || got != `\server\share\aigw\config.toml` {
		t.Fatalf("rooted Windows config path = %q, %v", got, err)
	}
	got, err = platform.ConfigPathFor("windows", map[string]string{"APPDATA": `\\`})
	if err != nil || got != `\aigw\config.toml` {
		t.Fatalf("root-only Windows config path = %q, %v", got, err)
	}
}

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
			got, err := platform.ClaudeSettingsPathFor(test.goos, test.env)
			if err != nil || filepath.Clean(got) != filepath.Clean(test.want) {
				t.Fatalf("ClaudeSettingsPathFor() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
	for _, goos := range []string{"darwin", "linux", "windows", "plan9"} {
		if _, err := platform.ClaudeSettingsPathFor(goos, map[string]string{}); err == nil {
			t.Fatalf("ClaudeSettingsPathFor(%q) accepted missing home", goos)
		}
	}
}
