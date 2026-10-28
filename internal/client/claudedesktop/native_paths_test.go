package claudedesktop

import (
	"path/filepath"
	"reflect"
	"testing"

	"aigw-cli/internal/client"
	"aigw-cli/internal/discovery"
)

func TestClaudeDesktopLibraryUsesTheOfficialPerUserScope(t *testing.T) {
	for _, test := range []struct {
		name string
		goos string
		env  map[string]string
		want string
	}{
		{name: "macOS", goos: "darwin", env: map[string]string{"HOME": "/Users/alex"}, want: "/Users/alex/Library/Application Support/Claude-3p/configLibrary"},
		{name: "Linux XDG", goos: "linux", env: map[string]string{"XDG_CONFIG_HOME": "/cfg"}, want: "/cfg/Claude-3p/configLibrary"},
		{name: "Linux home", goos: "linux", env: map[string]string{"HOME": "/home/alex"}, want: "/home/alex/.config/Claude-3p/configLibrary"},
		{name: "Windows", goos: "windows", env: map[string]string{"LOCALAPPDATA": `C:\Users\alex\AppData\Local`}, want: `C:\Users\alex\AppData\Local\Claude-3p\configLibrary`},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := LibraryPathFor(test.goos, test.env)
			if err != nil || filepath.Clean(got) != filepath.Clean(test.want) {
				t.Fatalf("LibraryPathFor() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
	for _, goos := range []string{"darwin", "linux", "windows", "plan9"} {
		if _, err := LibraryPathFor(goos, map[string]string{}); err == nil {
			t.Fatalf("LibraryPathFor(%q) accepted missing platform input", goos)
		}
	}
}

func TestClaudeDesktopApplicationPathsUseOfficialNativeLocations(t *testing.T) {
	tests := []struct {
		name string
		goos string
		env  map[string]string
		want []string
	}{
		{name: "macOS", goos: "darwin", env: map[string]string{"HOME": "/Users/alex"}, want: []string{"/Users/alex/Applications/Claude.app/Contents/MacOS/Claude", "/Applications/Claude.app/Contents/MacOS/Claude"}},
		{name: "macOS without home", goos: "darwin", env: map[string]string{}, want: []string{"/Applications/Claude.app/Contents/MacOS/Claude"}},
		{name: "Linux", goos: "linux", env: map[string]string{"HOME": "/home/alex"}, want: nil},
		{name: "Windows", goos: "windows", env: map[string]string{"LOCALAPPDATA": `C:\Users\alex\AppData\Local`}, want: []string{`C:\Users\alex\AppData\Local\AnthropicClaude\claude.exe`}},
		{name: "Windows without local app data", goos: "windows", env: map[string]string{}, want: nil},
		{name: "unsupported", goos: "plan9", env: map[string]string{}, want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ApplicationPathsFor(test.goos, test.env); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("ApplicationPathsFor() = %#v, want %#v", got, test.want)
			}
		})
	}
}

// AIGW does not start on a host where it cannot derive Claude Desktop's
// configuration library, and it names the same missing input as the library.
func TestAdapterRefusesAHostWithoutTheLibraryInputs(t *testing.T) {
	validator, validates := New().(client.HostValidator)
	if !validates {
		t.Fatal("the Claude Desktop adapter does not validate the host")
	}
	if err := validator.ValidateHost(discovery.Host{GOOS: "windows", Env: map[string]string{"LOCALAPPDATA": `C:\Users\alex\AppData\Local`}}); err != nil {
		t.Fatalf("ValidateHost() of a complete Windows host = %v", err)
	}
	// Every platform misses its own inputs here, while other platforms' inputs are present.
	foreign := map[string]string{"APPDATA": `C:\Users\alex\AppData\Roaming`, "USERPROFILE": `C:\Users\alex`}
	for goos, want := range map[string]string{
		"darwin":  "HOME is not set",
		"linux":   "HOME and XDG_CONFIG_HOME are not set",
		"windows": "LOCALAPPDATA is not set",
		"plan9":   `unsupported operating system "plan9"`,
	} {
		if err := validator.ValidateHost(discovery.Host{GOOS: goos, Home: "/home/alex", Env: foreign}); err == nil || err.Error() != want {
			t.Errorf("ValidateHost(%s) = %v, want %q", goos, err, want)
		}
	}
}
