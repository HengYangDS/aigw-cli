package platform_test

import (
	"path/filepath"
	"testing"

	"aigw-cli/internal/platform"
)

func TestConfigPathUsesPlatformConvention(t *testing.T) {
	tests := []struct {
		goos string
		env  map[string]string
		want string
	}{
		{"darwin", map[string]string{"HOME": "/Users/alex"}, "/Users/alex/Library/Application Support/aigw/configuration.toml"},
		{"linux", map[string]string{"HOME": "/home/alex"}, "/home/alex/.config/aigw/configuration.toml"},
		{"linux", map[string]string{"HOME": "/home/alex", "XDG_CONFIG_HOME": "/cfg"}, "/cfg/aigw/configuration.toml"},
		{"windows", map[string]string{"APPDATA": `C:\Users\alex\AppData\Roaming`}, `C:\Users\alex\AppData\Roaming\aigw\configuration.toml`},
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

func TestLauncherDirUsesAIGWOwnedDataBoundary(t *testing.T) {
	tests := []struct {
		goos string
		env  map[string]string
		want string
	}{
		{"darwin", map[string]string{"HOME": "/Users/alex"}, "/Users/alex/Library/Application Support/aigw/bin"},
		{"linux", map[string]string{"HOME": "/home/alex"}, "/home/alex/.local/share/aigw/bin"},
		{"linux", map[string]string{"HOME": "/home/alex", "XDG_DATA_HOME": "/data"}, "/data/aigw/bin"},
		{"windows", map[string]string{"LOCALAPPDATA": `C:\Users\alex\AppData\Local`}, `C:\Users\alex\AppData\Local\Programs\aigw\bin`},
	}
	for _, tt := range tests {
		got, err := platform.LauncherDirFor(tt.goos, tt.env)
		if err != nil || filepath.Clean(got) != filepath.Clean(tt.want) {
			t.Errorf("LauncherDirFor(%s) = %q, %v; want %q", tt.goos, got, err, tt.want)
		}
	}
}

func TestLauncherDirRejectsUnsupportedOS(t *testing.T) {
	if _, err := platform.LauncherDirFor("plan9", map[string]string{}); err == nil {
		t.Fatal("unsupported operating system unexpectedly admitted")
	}
}

func TestDefaultLauncherDirectoryFallsBackAndHonorsOverride(t *testing.T) {
	if got, err := platform.DefaultLauncherDirFor("darwin", map[string]string{"AIGW_LAUNCHER_DIR": "  /custom/shims  "}, "/opt/bin/aigw"); err != nil || got != "/custom/shims" {
		t.Fatalf("override = %q, %v", got, err)
	}
	if got, err := platform.DefaultLauncherDirFor("darwin", map[string]string{}, "/opt/bin/aigw"); err != nil || got != "/opt/bin" {
		t.Fatalf("fallback = %q, %v", got, err)
	}
}

func TestExecutableDirectoryUsesTargetPathConvention(t *testing.T) {
	tests := []struct{ goos, executable, want string }{
		{goos: "linux", executable: "/opt/aigw/bin/aigw", want: "/opt/aigw/bin"},
		{goos: "windows", executable: `C:\Program Files\AIGW\aigw.exe`, want: `C:\Program Files\AIGW`},
		{goos: "windows", executable: `C:\aigw.exe`, want: `C:\`},
		{goos: "windows", executable: `/aigw.exe`, want: `/`},
		{goos: "windows", executable: `aigw.exe`, want: `.`},
		{goos: "windows", executable: ``, want: ``},
		{goos: "windows", executable: `C:\AIGW\\`, want: `C:\`},
	}
	for _, test := range tests {
		if got := platform.ExecutableDirFor(test.goos, test.executable); got != test.want {
			t.Errorf("ExecutableDirFor(%q, %q) = %q, want %q", test.goos, test.executable, got, test.want)
		}
	}
}

func TestWindowsJoinPreservesLeadingSeparator(t *testing.T) {
	got, err := platform.ConfigPathFor("windows", map[string]string{"APPDATA": `\`})
	want := `\aigw\configuration.toml`
	if err != nil || got != want {
		t.Fatalf("ConfigPathFor(windows) with root APPDATA = %q, %v; want %q", got, err, want)
	}
}
