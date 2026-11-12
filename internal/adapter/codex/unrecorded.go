package codex

import (
	"fmt"
	"path/filepath"
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/platform"

	"github.com/pelletier/go-toml/v2"
)

// refuseUnrecordedProvider refuses, before any write, a Codex file that holds
// the provider table AIGW projects while AIGW keeps no record of it. AIGW
// cannot prove that table is its own, so it neither adopts it nor writes a
// second one, which Codex would refuse to read. The record restored from a
// copy proves the table; AIGW's table taken out lets a first projection begin
// again.
func refuseUnrecordedProvider(config, state platform.FileSnapshot, provider, configPath, statePath string) error {
	if !config.Exists || state.Exists {
		return nil
	}
	ranges, err := codexProviderRanges(string(config.Data), provider)
	if err != nil || len(ranges) == 0 {
		return nil
	}
	record := filepath.Base(statePath)
	return adapter.NewConflictError(
		fmt.Sprintf("Codex config already has a [model_providers.%s] table, but AIGW's record of it, %s, is missing; refusing to adopt it or write a second one", provider, record),
		adapter.ThenRun(fmt.Sprintf("Restore %s in its folder from a copy of it, or take the [model_providers.%s] table and the lines marked \"# managed by AIGW\" out of %s", record, provider, filepath.Base(configPath))),
		nil,
	)
}

// requireParsableCodexConfig refuses rendered Codex settings that the TOML
// parser rejects, so a write never leaves a file that Codex cannot read.
func requireParsableCodexConfig(text string) error {
	var document map[string]any
	if err := toml.Unmarshal([]byte(text), &document); err != nil {
		return fmt.Errorf("refusing to write Codex config that does not parse as TOML: %w", err)
	}
	return nil
}

// refuseRepeatedProvider refuses a Codex file that declares the provider table
// AIGW projects more than once, which Codex cannot read. AIGW 0.6.1 and
// earlier wrote one when AIGW's record was missing. Only one of the tables is
// AIGW's, so the operator keeps the right one by hand.
func refuseRepeatedProvider(text string, ranges [][2]int, provider string) error {
	headers := 0
	for _, span := range ranges {
		header, _, _ := strings.Cut(text[span[0]:span[1]], "\n")
		if strings.TrimSpace(header) == "[model_providers."+provider+"]" {
			headers++
		}
	}
	if headers < 2 {
		return nil
	}
	return adapter.NewConflictError(
		fmt.Sprintf("Codex config declares [model_providers.%s] %d times, so Codex cannot read it", provider, headers),
		adapter.ThenRun(fmt.Sprintf("Keep one [model_providers.%s] table in config.toml, the one between AIGW's markers, and remove the others", provider)),
		nil,
	)
}
