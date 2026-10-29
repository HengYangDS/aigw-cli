package main

import (
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/claudedesktop"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/client/hermes"
	builtinclients "aigw-cli/internal/clients"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestNativeClientFixtureMatchesClaudeDesktopDiscovery(t *testing.T) {
	root := t.TempDir()
	clientBin := filepath.Join(root, "client bin")
	if err := os.MkdirAll(clientBin, 0o755); err != nil {
		t.Fatal(err)
	}
	journey := &journeyFixture{testing: t, root: root, clientBin: clientBin}
	journey.installClientFixture(claudedesktop.ID)
	discovered := claudedesktop.New().Discover(discovery.System{
		GOOS: runtime.GOOS, Home: filepath.Join(root, "home"),
		Env: map[string]string{"XDG_CONFIG_HOME": filepath.Join(root, "config"), "LOCALAPPDATA": filepath.Join(root, "localappdata")}, Path: clientBin,
	}).Executable(claudedesktop.ID)
	if runtime.GOOS == "linux" {
		if discovered != "" {
			t.Fatalf("Claude Desktop fixture was discovered on unsupported Linux host: %q", discovered)
		}
		return
	}
	if discovered == "" || !strings.HasPrefix(filepath.Clean(discovered), filepath.Clean(root)+string(filepath.Separator)) {
		t.Fatalf("Claude Desktop fixture discovery = %q, want an executable owned by %s", discovered, root)
	}
}

func deferredJourneyClientIDs() []string {
	clients := slices.Clone(builtinclients.Admission().IDs())
	if runtime.GOOS != "linux" {
		return clients
	}
	return slices.DeleteFunc(clients, func(client string) bool {
		return client == claudedesktop.ID
	})
}

func runDeferredClientInstallation(t *testing.T, artifact func(*testing.T) string, endpoint string) {
	t.Helper()
	for _, clientID := range deferredJourneyClientIDs() {
		t.Run(clientID, func(t *testing.T) {
			journey := newNativeJourney(t, artifact(t), endpoint, false)
			manifest := fmt.Sprintf(`version = 7

[recommendations.claude]
[recommendations.claude.primary]
route = 'native-system-keyring-probe-claude'

[recommendations.claude-desktop]
[recommendations.claude-desktop.primary]
route = 'native-system-keyring-probe-claude'

[recommendations.codex]
[recommendations.codex.primary]
route = 'native-system-keyring-probe-codex'

[recommendations.hermes]
[recommendations.hermes.primary]
route = 'native-system-keyring-probe-claude'
protocol = 'anthropic'

[accounts.native-system-keyring-probe]
label = 'Native System Keyring Probe'

[accounts.native-system-keyring-probe.endpoints]
anthropic = %[1]q
openai_responses = %[1]q

[models.claude-test]
label = 'Claude Test'

[models.gpt-test]
label = 'GPT Test'

[routes.native-system-keyring-probe-claude]
label = 'Native System Keyring Probe Claude'
account = 'native-system-keyring-probe'
model = 'claude-test'
upstream_model = 'claude-test'
interfaces = { anthropic = [] }

[routes.native-system-keyring-probe-codex]
label = 'Native System Keyring Probe Codex'
account = 'native-system-keyring-probe'
model = 'gpt-test'
upstream_model = 'gpt-test'
interfaces = { openai_responses = [] }
`, journey.endpoint)
			if err := os.WriteFile(journey.manifest, []byte(manifest), 0o600); err != nil {
				t.Fatal(err)
			}
			journey.setEnvironment(secret.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
			journey.run("setup", "--from", journey.manifest)

			before, err := store.NewStore(journey.config, builtinclients.Admission()).Load()
			if err != nil {
				t.Fatal(err)
			}
			beforeProjections := make(map[string]map[string]string)
			for _, candidate := range builtinclients.Admission().IDs() {
				beforeProjections[candidate] = journey.clientProjectionSnapshot(candidate)
			}
			if len(beforeProjections[clientID]) != 0 {
				t.Skipf("%s is already installed on this host; clean hosted runners exercise deferred activation", clientID)
			}

			userPath, userSentinel := journey.seedUnownedClientConfiguration(clientID)
			journey.installClientFixture(clientID)
			journey.run("sync")

			after, err := store.NewStore(journey.config, builtinclients.Admission()).Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := after.Clients[clientID]
			if !binding.Enabled || binding.Executable == "" || clientID != claude.ID && len(binding.Targets) == 0 {
				t.Fatalf("%s binding was not activated: %#v", clientID, binding)
			}
			journey.requireClientProjection(clientID)
			journey.requireUnownedClientConfiguration(userPath, userSentinel, "sync")
			for _, candidate := range builtinclients.Admission().IDs() {
				if candidate == clientID {
					continue
				}
				if !reflect.DeepEqual(before.Clients[candidate], after.Clients[candidate]) {
					t.Fatalf("sync for %s changed %s binding", clientID, candidate)
				}
				if projection := journey.clientProjectionSnapshot(candidate); !reflect.DeepEqual(beforeProjections[candidate], projection) {
					t.Fatalf("sync for %s changed %s projection", clientID, candidate)
				}
			}
			journey.requireDeferredWithdrawal(clientID, userPath, userSentinel)
		})
	}
}

func (j *journeyFixture) seedUnownedClientConfiguration(clientID string) (string, string) {
	j.testing.Helper()
	paths := j.clientProjectionPaths(clientID)
	path := paths[0]
	const sentinel = "user-owned-before-aigw"
	content := ""
	switch clientID {
	case claude.ID:
		content = fmt.Sprintf(`{"theme":%q}`, sentinel)
	case claudedesktop.ID:
		path = paths[1]
		content = fmt.Sprintf(`{"theme":%q}`, sentinel)
	case codex.ID:
		content = fmt.Sprintf("user_setting = %q\n", sentinel)
	case hermes.ID:
		content = fmt.Sprintf("terminal:\n  backend: %s\n", sentinel)
	default:
		j.testing.Fatalf("unsupported client %q", clientID)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		j.testing.Fatal(err)
	}
	return path, sentinel
}

func (j *journeyFixture) requireUnownedClientConfiguration(path, sentinel, operation string) {
	j.testing.Helper()
	if !strings.Contains(string(readFile(j.testing, path)), sentinel) {
		j.testing.Fatalf("%s replaced user-owned configuration at %s", operation, path)
	}
}

func (j *journeyFixture) requireDeferredWithdrawal(clientID, userPath, userSentinel string) {
	j.testing.Helper()
	j.uninstallWithAndRequireInstallationRemoved(j.source)
	if _, err := os.Stat(j.config + ".verified.json"); !os.IsNotExist(err) {
		j.testing.Fatalf("uninstall retained verified checkpoint: %v", err)
	}
	for _, candidate := range builtinclients.Admission().IDs() {
		if candidate == clientID {
			j.requireNoClientProjection(candidate, userPath)
			j.requireUnownedClientConfiguration(userPath, userSentinel, "uninstall")
			continue
		}
		j.requireNoClientProjection(candidate)
	}
}

func (j *journeyFixture) requireNoClientProjection(clientID string, preservedPaths ...string) {
	j.testing.Helper()
	for _, path := range j.clientProjectionPaths(clientID) {
		if slices.Contains(preservedPaths, path) {
			continue
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			j.testing.Fatalf("%s projection unexpectedly exists at %s: %v", clientID, path, err)
		}
	}
}

func (j *journeyFixture) requireClientProjection(clientID string) {
	j.testing.Helper()
	paths := j.clientProjectionPaths(clientID)
	for _, path := range paths {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			j.testing.Fatalf("%s projection missing at %s: %v", clientID, path, err)
		}
	}
	model := "claude-test"
	if clientID == codex.ID {
		model = "gpt-test"
	}
	primary := paths[0]
	if clientID == claudedesktop.ID {
		primary = paths[2]
	}
	requireFileContains(j.testing, primary, j.endpoint, model)
}

func (j *journeyFixture) clientProjectionSnapshot(clientID string) map[string]string {
	j.testing.Helper()
	snapshot := make(map[string]string)
	for _, path := range j.clientProjectionPaths(clientID) {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			j.testing.Fatalf("read %s projection %s: %v", clientID, path, err)
		}
		snapshot[path] = string(data)
	}
	return snapshot
}

func (j *journeyFixture) clientProjectionPaths(clientID string) []string {
	j.testing.Helper()
	home := filepath.Dir(filepath.Dir(j.settings))
	switch clientID {
	case claude.ID:
		return []string{j.settings, j.settings + ".aigw-state.json"}
	case codex.ID:
		path := filepath.Join(home, ".codex", "config.toml")
		return []string{path, path + ".aigw-state.json"}
	case hermes.ID:
		path := filepath.Join(environmentValues(j.environment)["HERMES_HOME"], "config.yaml")
		return []string{path, path + ".aigw-state.json"}
	case claudedesktop.ID:
		library, err := claudedesktop.LibraryPathFor(runtime.GOOS, environmentValues(j.environment))
		if err != nil {
			j.testing.Fatal(err)
		}
		desktopPaths := claudedesktop.PathsForLibrary(library)
		return []string{
			desktopPaths.StandardConfig,
			desktopPaths.ThirdPartyConfig,
			desktopPaths.Profile,
			desktopPaths.Metadata,
			desktopPaths.State,
		}
	default:
		j.testing.Fatalf("unsupported client %q", clientID)
		return nil
	}
}
