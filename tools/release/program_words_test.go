package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"

	"aigw-cli/internal/platform"
)

// The journeys run the commands of programs that differ by release: a
// journey that starts from a published release keeps that release's words,
// which these helpers spell.

// tokenOwner is the flag with which program's team setup names the provider
// whose Token it reads from standard input: --provider from 0.7.0, and
// --account in a published release before it, so that a journey that starts
// from a published release keeps that release's words.
func (j *journeyFixture) tokenOwner(program string) string {
	j.testing.Helper()
	probe := exec.CommandContext(context.WithoutCancel(j.testing.Context()), program, "setup", "--help")
	probe.Env = j.environment
	help, _ := probe.Output()
	if bytes.Contains(help, []byte("--provider")) {
		return "--provider"
	}
	return "--account"
}

// repairs reports whether program still has `aigw repair` and the top-level
// `aigw rollback`, which 0.7.0 replaced with `aigw sync` and `aigw config
// rollback`, so that a journey that starts from a published release keeps
// that release's commands.
func (j *journeyFixture) repairs(program string) bool {
	j.testing.Helper()
	probe := exec.CommandContext(context.WithoutCancel(j.testing.Context()), program, "repair", "--help")
	probe.Env = j.environment
	return probe.Run() == nil
}

// configRollback spells `aigw config rollback` with words in program's words.
func (j *journeyFixture) configRollback(program string, words ...string) []string {
	j.testing.Helper()
	if j.repairs(program) {
		return append([]string{"rollback"}, words...)
	}
	return append([]string{"config", "rollback"}, words...)
}

// localCheck spells program's check of local readiness as JSON: `aigw check
// --depth local` from 0.7.0, and `aigw doctor` in a published release before
// it, so that a journey that starts from a published release keeps that
// release's commands.
func (j *journeyFixture) localCheck(program string) []string {
	j.testing.Helper()
	probe := exec.CommandContext(context.WithoutCancel(j.testing.Context()), program, "doctor", "--help")
	probe.Env = j.environment
	if probe.Run() == nil {
		return []string{"doctor", "--json"}
	}
	return []string{"check", "--depth", "local", "--json"}
}

// rotation spells the rotation of provider's Token from standard input in
// program's words: `aigw provider rotate` from 0.7.0, and `aigw rotate` in a
// published release before it, so that a journey that starts from a
// published release keeps that release's commands.
func (j *journeyFixture) rotation(program, provider string) []string {
	j.testing.Helper()
	probe := exec.CommandContext(context.WithoutCancel(j.testing.Context()), program, "provider", "--help")
	probe.Env = j.environment
	if probe.Run() == nil {
		return []string{"provider", "rotate", provider, "--token-stdin"}
	}
	return []string{"rotate", provider, "--token-stdin"}
}

// lifecycle spells a command of program's own lifecycle in program's words.
// words are the spelling after `aigw self` from 0.7.0, which a program with
// `aigw self` runs; a published release before it runs the top-level command
// that 0.7.0 replaced, with update's --rollback for rollback, installation for
// status and --candidate for --archive, so that a journey that starts from a
// published release keeps that release's commands.
func (j *journeyFixture) lifecycle(program string, words ...string) []string {
	j.testing.Helper()
	probe := exec.CommandContext(context.WithoutCancel(j.testing.Context()), program, "self", "--help")
	probe.Env = j.environment
	if probe.Run() == nil {
		return append([]string{"self"}, words...)
	}
	switch words[0] {
	case "rollback":
		return append([]string{"update", "--rollback"}, words[1:]...)
	case "status":
		return append([]string{"installation"}, words[1:]...)
	}
	spelled := slices.Clone(words)
	for index, word := range spelled {
		if word == "--archive" {
			spelled[index] = "--candidate"
		}
	}
	return spelled
}

func (j *journeyFixture) requireRepairPreservesUserSettings(previousTheme, theme string) {
	j.testing.Helper()
	settings := map[string]json.RawMessage{}
	if err := json.Unmarshal(readFile(j.testing, j.settings), &settings); err != nil {
		j.testing.Fatal(err)
	}
	var observedTheme string
	if data, present := settings["theme"]; present {
		if err := json.Unmarshal(data, &observedTheme); err != nil {
			j.testing.Fatal(err)
		}
	}
	if observedTheme != previousTheme {
		j.testing.Fatalf("program update changed the user theme: got=%q want=%q", observedTheme, previousTheme)
	}
	themeData, err := json.Marshal(theme)
	if err != nil {
		j.testing.Fatal(err)
	}
	settings["theme"] = themeData
	userSettings, err := json.Marshal(settings)
	if err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(j.settings, userSettings, 0o600); err != nil {
		j.testing.Fatal(err)
	}
	// A published release repairs; from 0.7.0 sync does what repair did.
	repair := []string{"sync"}
	if j.repairs(j.binary) {
		repair = []string{"repair"}
	}
	j.run(repair...)
	if got := readFile(j.testing, j.settings); !bytes.Equal(got, userSettings) {
		j.testing.Fatalf("%s changed the externally edited settings: got=%s want=%s", repair[0], got, userSettings)
	}
	paths := []string{j.config, j.config + ".bak", verifiedSnapshot(j.config), j.settings, j.settings + ".aigw-state.json"}
	codexConfig := filepath.Join(j.root, "home", ".codex", "config.toml")
	paths = append(paths, codexConfig, codexConfig+".aigw-state.json", codexConfig+".aigw-model-catalog.json")
	before := make(map[string]platform.FileSnapshot, len(paths))
	for _, path := range paths {
		content, err := platform.CaptureFileSnapshot(path)
		if err != nil {
			j.testing.Fatal(err)
		}
		before[path] = content
	}
	for range 2 {
		var result struct {
			ConfigurationAction string `json:"configuration_action"`
			NextAction          string `json:"next_action"`
		}
		if err := json.Unmarshal(j.run(append(repair, "--json")...), &result); err != nil {
			j.testing.Fatal(err)
		}
		// sync's document names no configuration action; repair's did.
		if (repair[0] == "repair" && result.ConfigurationAction != "already_converged") || result.NextAction != "aigw check" {
			j.testing.Fatalf("repeated %s is not terminal: %#v", repair[0], result)
		}
		for _, path := range paths {
			after, err := platform.CaptureFileSnapshot(path)
			if err != nil {
				j.testing.Fatal(err)
			}
			if !reflect.DeepEqual(after, before[path]) {
				j.testing.Fatalf("repeated repair changed %s", path)
			}
		}
	}
}
