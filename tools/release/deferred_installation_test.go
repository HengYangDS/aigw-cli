package main

import (
	claudedesktop "aigw-cli/internal/claude/desktop"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/secrets"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func runDeferredClientInstallation(t *testing.T, artifact, endpoint string) {
	t.Helper()
	for _, clientID := range configuration.AdmittedClientIDs() {
		t.Run(clientID, func(t *testing.T) {
			journey := newNativeJourney(t, artifact, endpoint, false)
			manifest := fmt.Sprintf(`version = 6

[recommendations.claude]
profile = 'native-system-keyring-probe-claude'

[recommendations.claude-desktop]
profile = 'native-system-keyring-probe-claude'

[recommendations.codex]
profile = 'native-system-keyring-probe-codex'

[recommendations.hermes]
profile = 'native-system-keyring-probe-claude'
protocol = 'anthropic'

[accounts.native-system-keyring-probe]
label = 'Native System Keyring Probe'

[accounts.native-system-keyring-probe.endpoints]
anthropic = %[1]q
openai_responses = %[1]q

[profiles.native-system-keyring-probe-claude]
label = 'Native System Keyring Probe Claude'
account = 'native-system-keyring-probe'
model = 'claude-test'
protocols = ['anthropic']

[profiles.native-system-keyring-probe-codex]
label = 'Native System Keyring Probe Codex'
account = 'native-system-keyring-probe'
model = 'gpt-test'
protocols = ['openai_responses']
`, journey.endpoint)
			if err := os.WriteFile(journey.manifest, []byte(manifest), 0o600); err != nil {
				t.Fatal(err)
			}
			journey.setEnvironment(secrets.EnvironmentKey("native-system-keyring-probe"), "native-journey-token")
			journey.run("setup", "--from", journey.manifest)

			before, err := configuration.NewStore(journey.config).Load()
			if err != nil {
				t.Fatal(err)
			}
			beforeProjections := make(map[string]map[string]string)
			for _, candidate := range configuration.AdmittedClientIDs() {
				beforeProjections[candidate] = journey.clientProjectionSnapshot(candidate)
			}
			if len(beforeProjections[clientID]) != 0 {
				t.Skipf("%s is already installed on this host; clean hosted runners exercise deferred activation", clientID)
			}

			journey.installClientFixture(clientID)
			journey.run("sync")

			after, err := configuration.NewStore(journey.config).Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := after.Clients[clientID]
			if !binding.Enabled || binding.Executable == "" || clientID != configuration.ClientClaude && len(binding.Targets) == 0 {
				t.Fatalf("%s binding was not activated: %#v", clientID, binding)
			}
			journey.requireClientProjection(clientID)
			for _, candidate := range configuration.AdmittedClientIDs() {
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
			journey.uninstallAndRequireOwnedFilesAbsent()
			for _, candidate := range configuration.AdmittedClientIDs() {
				journey.requireNoClientProjection(candidate)
			}
		})
	}
}

func (j *journeyFixture) requireNoClientProjection(clientID string) {
	j.testing.Helper()
	for _, path := range j.clientProjectionPaths(clientID) {
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
	if clientID == configuration.ClientCodex {
		model = "gpt-test"
	}
	primary := paths[0]
	if clientID == configuration.ClientClaudeDesktop {
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
	case configuration.ClientClaude:
		return []string{j.settings, j.settings + ".aigw-state.json"}
	case configuration.ClientCodex:
		path := filepath.Join(home, ".codex", "config.toml")
		return []string{path, path + ".aigw-state.json"}
	case configuration.ClientHermes:
		path := filepath.Join(home, ".hermes", "config.yaml")
		return []string{path, path + ".aigw-state.json"}
	case configuration.ClientClaudeDesktop:
		paths, err := platform.PathsFor(runtime.GOOS, environmentValues(j.environment))
		if err != nil {
			j.testing.Fatal(err)
		}
		desktopPaths := claudedesktop.PathsForLibrary(paths.ClaudeDesktopLibrary)
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
