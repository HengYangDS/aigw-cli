package configuration

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func prepareVerificationDependencyFixture(t *testing.T, root string) map[string][]byte {
	t.Helper()
	installed := filepath.Join(root, "installs", "selected")
	for _, path := range []string{filepath.Join(root, "tools"), filepath.Join(installed, "environments"), filepath.Join(installed, "pm-runtime")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	config := "plugins: {enabled: [root-member], disabled: [unused], token: public-private-fixture}\nmemory: {provider: native-memory, api_key: public-private-fixture}\nmodel: {default: operator-model}\n"
	original := map[string][]byte{
		filepath.Join(root, "config.yaml"):                                                      []byte(config),
		filepath.Join(root, "plugins", "root-member", "plugin.yaml"):                            []byte("name: root-member\n"),
		filepath.Join(installed, "facts.json"):                                                  []byte("{\"generation\":\"selected\"}\n"),
		filepath.Join(installed, ".install.lock"):                                               {},
		filepath.Join(installed, "boot-record.json"):                                            []byte("operator boot\n"),
		filepath.Join(root, "installs", "metadata"):                                             []byte("not an installation\n"),
		filepath.Join(root, "profiles", "research", "config.yaml"):                              []byte("plugins: {enabled: [profile-member]}\nprivate: preserved\n"),
		filepath.Join(root, "profiles", "research", "plugins", "profile-member", "plugin.yaml"): []byte("name: profile-member\n"),
		filepath.Join(root, "profiles", "retired", "config.yaml"):                               []byte("plugins: {enabled: [retired-member]}\n"),
		filepath.Join(root, "profiles", ".deleted", "retired"):                                  {},
		filepath.Join(root, "profiles", "default", "config.yaml"):                               []byte("plugins: {enabled: [default-alias]}\n"),
		filepath.Join(root, "profiles", "Invalid", "config.yaml"):                               []byte("plugins: {enabled: [invalid-name]}\n"),
		filepath.Join(root, "profiles", "plain", "config.yaml"):                                 []byte("private: preserved\n"),
	}
	for path, data := range original {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return original
}

func TestVerificationHomePreservesInstalledProfileDependencyUnion(t *testing.T) {
	root, temporary := t.TempDir(), t.TempDir()
	t.Setenv("TMPDIR", temporary)
	if runtime.GOOS == "windows" {
		home, cleanup, err := CreateVerificationHome(filepath.Join(root, "config.yaml"))
		if err == nil || home != "" || cleanup != nil || !strings.Contains(err.Error(), "no client process was started") {
			t.Fatalf("unqualified Windows verification = %s, %v", home, err)
		}
		if entries, err := os.ReadDir(temporary); err != nil || len(entries) != 0 {
			t.Fatalf("refusal created runtime state: %v, %v", entries, err)
		}
		return
	}
	original := prepareVerificationDependencyFixture(t, root)
	home, cleanup, err := CreateVerificationHome(filepath.Join(root, "profiles", "research", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	})
	for _, name := range []string{"tools", "installs/selected/facts.json", "installs/selected/environments", "installs/selected/pm-runtime", "installs/selected/.install.lock"} {
		source, err := os.Stat(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		borrowed, err := os.Stat(filepath.Join(home, filepath.FromSlash(name)))
		if err != nil || !os.SameFile(source, borrowed) {
			t.Fatalf("dependency identity changed for %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, "installs", "selected", "boot-record.json"), []byte("private boot\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profiles, err := filepath.Glob(filepath.Join(home, "profiles", "*", "config.yaml"))
	if err != nil || len(profiles) != 2 {
		t.Fatalf("effective dependency profile union = %v, %v", profiles, err)
	}
	var selected []string
	for _, path := range profiles {
		selected = append(selected, observeVerificationDependencyProfile(t, root, path)...)
	}
	slices.Sort(selected)
	if !slices.Equal(selected, []string{"profile-member", "root-member"}) {
		t.Fatalf("dependency union includes an inactive profile: %v", selected)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(temporary); err != nil || len(entries) != 0 {
		t.Fatalf("verification cleanup left runtime state: %v, %v", entries, err)
	}
	for path, want := range original {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("verification changed native state %s: %v", path, err)
		}
	}
}

func observeVerificationDependencyProfile(t *testing.T, root, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || bytes.Contains(data, []byte("public-private-fixture")) || bytes.Contains(data, []byte("operator-model")) || bytes.Contains(data, []byte("private:")) {
		t.Fatalf("dependency profile retained operator configuration: %v", err)
	}
	var config struct {
		Plugins struct {
			Enabled  []string `yaml:"enabled"`
			Disabled []string `yaml:"disabled"`
		} `yaml:"plugins"`
		Memory struct {
			Provider string `yaml:"provider"`
		} `yaml:"memory"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(config.Plugins.Enabled, "root-member") && (!slices.Equal(config.Plugins.Disabled, []string{"unused"}) || config.Memory.Provider != "native-memory") {
		t.Fatal("native dependency selectors changed")
	}
	member := config.Plugins.Enabled[0]
	borrowed, err := os.Stat(filepath.Join(filepath.Dir(path), "plugins", member, "plugin.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "plugins", member, "plugin.yaml")
	if member == "profile-member" {
		source = filepath.Join(root, "profiles", "research", "plugins", member, "plugin.yaml")
	}
	info, err := os.Stat(source)
	if err != nil || !os.SameFile(info, borrowed) {
		t.Fatalf("plugin dependency identity changed: %v", err)
	}
	return config.Plugins.Enabled
}

func TestVerificationHomeRequiresThePhysicalSelectionLock(t *testing.T) {
	root, temporary := t.TempDir(), t.TempDir()
	t.Setenv("TMPDIR", temporary)
	installed := filepath.Join(root, "installs", "selected")
	if err := os.MkdirAll(installed, 0o700); err != nil {
		t.Fatal(err)
	}
	facts := []byte("{\"generation\":\"selected\"}\n")
	if err := os.WriteFile(filepath.Join(installed, "facts.json"), facts, 0o600); err != nil {
		t.Fatal(err)
	}
	home, cleanup, err := CreateVerificationHome(filepath.Join(root, "config.yaml"))
	want := "selection lock is unavailable"
	if runtime.GOOS == "windows" {
		want = "no client process was started"
	}
	if err == nil || !strings.Contains(err.Error(), want) || home != "" || cleanup != nil {
		t.Fatalf("unqualified dependency selection was borrowed: %s, %v", home, err)
	}
	if entries, err := os.ReadDir(temporary); err != nil || len(entries) != 0 {
		t.Fatalf("failed preparation left runtime state: %v, %v", entries, err)
	}
	if data, err := os.ReadFile(filepath.Join(installed, "facts.json")); err != nil || !bytes.Equal(data, facts) {
		t.Fatalf("refusal changed the native dependency selection: %v", err)
	}
}
