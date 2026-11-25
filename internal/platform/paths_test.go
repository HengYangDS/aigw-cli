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
		secrets     string
		installDir  string
		installName string
	}{
		{name: "macOS", goos: "darwin", env: map[string]string{"HOME": "/Users/alex"}, config: "/Users/alex/Library/Application Support/aigw/aigw.toml", data: "/Users/alex/Library/Application Support/aigw", secrets: "/Users/alex/Library/Application Support/aigw/secrets", installDir: "/Users/alex/.local/bin", installName: "aigw"},
		{name: "Linux", goos: "linux", env: map[string]string{"HOME": "/home/alex", "XDG_CONFIG_HOME": "/cfg", "XDG_DATA_HOME": "/data"}, config: "/cfg/aigw/aigw.toml", data: "/data/aigw", secrets: "/data/aigw/secrets", installDir: "/home/alex/.local/bin", installName: "aigw"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := platform.PathsFor(test.goos, test.env)
			if err != nil {
				t.Fatal(err)
			}
			if got.Config != test.config || got.Data != test.data || got.Secrets != test.secrets || got.InstallDir != test.installDir || got.InstallName != test.installName {
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
		{name: "installation", goos: "linux", env: map[string]string{"XDG_CONFIG_HOME": "/cfg", "XDG_DATA_HOME": "/data"}, want: "HOME is not set"},
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
		{"darwin", map[string]string{"HOME": "/Users/alex"}, "/Users/alex/Library/Application Support/aigw/aigw.toml"},
		{"linux", map[string]string{"HOME": "/home/alex"}, "/home/alex/.config/aigw/aigw.toml"},
		{"linux", map[string]string{"HOME": "/home/alex", "XDG_CONFIG_HOME": "/cfg"}, "/cfg/aigw/aigw.toml"},
	}
	for _, tt := range tests {
		got, err := platform.ConfigPathFor(tt.goos, tt.env)
		if err != nil || filepath.Clean(got) != filepath.Clean(tt.want) {
			t.Errorf("ConfigPathFor(%s) = %q, %v; want %q", tt.goos, got, err, tt.want)
		}
	}
}

func TestPlatformPathsRequireSupportedHostInputs(t *testing.T) {
	for _, operation := range []struct {
		name string
		path func(string, map[string]string) (string, error)
	}{
		{"configuration", platform.ConfigPathFor},
		{"data", platform.DataDirFor},
	} {
		t.Run(operation.name, func(t *testing.T) {
			for _, goos := range []string{"darwin", "linux", "plan9"} {
				t.Run(goos, func(t *testing.T) {
					if _, err := operation.path(goos, map[string]string{}); err == nil {
						t.Fatal("unsupported operating system or missing platform input admitted")
					}
				})
			}
		})
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
	}
	for _, tt := range tests {
		got, err := platform.DataDirFor(tt.goos, tt.env)
		if err != nil || filepath.Clean(got) != filepath.Clean(tt.want) {
			t.Errorf("DataDirFor(%s, %v) = %q, %v; want %q", tt.goos, tt.env, got, err, tt.want)
		}
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

func TestUserBinDirRefusesAnUnsupportedOS(t *testing.T) {
	if _, err := platform.UserBinDirFor("plan9", map[string]string{}); err == nil {
		t.Fatal("unsupported operating system unexpectedly admitted")
	}
}
