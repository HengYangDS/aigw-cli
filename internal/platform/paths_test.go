package platform_test

import (
	"path/filepath"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/platform"
)

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

func TestShimDirUsesAIGWOwnedDataBoundary(t *testing.T) {
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
		got, err := platform.ShimDirFor(tt.goos, tt.env)
		if err != nil || filepath.Clean(got) != filepath.Clean(tt.want) {
			t.Errorf("ShimDirFor(%s) = %q, %v; want %q", tt.goos, got, err, tt.want)
		}
	}
}

func TestShimDirRejectsUnsupportedOS(t *testing.T) {
	if _, err := platform.ShimDirFor("plan9", map[string]string{}); err == nil {
		t.Fatal("unsupported operating system unexpectedly admitted")
	}
}

func TestWindowsJoinPreservesLeadingSeparator(t *testing.T) {
	got, err := platform.ConfigPathFor("windows", map[string]string{"APPDATA": `\`})
	want := `\aigw\config.toml`
	if err != nil || got != want {
		t.Fatalf("ConfigPathFor(windows) with root APPDATA = %q, %v; want %q", got, err, want)
	}
}

func TestAirLogDirUsesMacOSHomeBoundary(t *testing.T) {
	got, err := platform.AirLogDirFor("darwin", map[string]string{"HOME": "/Users/alex"})
	if err != nil {
		t.Fatal(err)
	}
	want := "/Users/alex/Library/Logs/JetBrains/Air"
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("AirLogDirFor() = %q, want %q", got, want)
	}
}

func TestAirLogDirRejectsUnsupportedOrMissingHome(t *testing.T) {
	if _, err := platform.AirLogDirFor("linux", map[string]string{"HOME": "/home/alex"}); err == nil {
		t.Fatal("non-macOS Air log path unexpectedly admitted")
	}
	if _, err := platform.AirLogDirFor("darwin", map[string]string{}); err == nil {
		t.Fatal("missing HOME unexpectedly admitted")
	}
}
