package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

type miseTask struct {
	Name string   `json:"name"`
	Run  []string `json:"run"`
}

type miseConfiguration struct {
	Settings struct {
		LegacyVersionFile      *bool `toml:"legacy_version_file"`
		NotFoundSystemFallback *bool `toml:"not_found_system_fallback"`
	} `toml:"settings"`
}

func TestMiseConfigurationRejectsAmbientToolFallbacks(t *testing.T) {
	root := repositoryRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var configuration miseConfiguration
	if err := toml.Unmarshal(content, &configuration); err != nil {
		t.Fatal(err)
	}
	for name, setting := range map[string]*bool{
		"legacy_version_file":       configuration.Settings.LegacyVersionFile,
		"not_found_system_fallback": configuration.Settings.NotFoundSystemFallback,
	} {
		if setting == nil || *setting {
			t.Errorf("mise setting %s must be explicitly false", name)
		}
	}
}

func TestMiseTasksDelegateToCanonicalOwners(t *testing.T) {
	root := repositoryRoot(t)
	command := exec.Command("mise", "-C", root, "tasks", "ls", "--json")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("list mise tasks: %v", err)
	}
	var listed []miseTask
	if err := json.Unmarshal(output, &listed); err != nil {
		t.Fatalf("decode mise tasks: %v", err)
	}
	got := make(map[string][]string, len(listed))
	for _, task := range listed {
		got[task.Name] = task.Run
	}
	want := map[string][]string{
		"bootstrap": {"npm ci --ignore-scripts"},
		"check":     {"go run ./tools/ci source"},
		"native":    {"go run ./tools/ci native"},
		"release":   {"go run ./tools/release build dist"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mise tasks = %#v, want %#v", got, want)
	}
}
