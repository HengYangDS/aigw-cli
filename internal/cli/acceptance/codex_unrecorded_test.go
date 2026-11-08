package cli_test

import (
	"os"
	"regexp"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"

	"github.com/pelletier/go-toml/v2"
)

// With Codex's record deleted, sync wrote a second [model_providers.aigw]
// table, which Codex cannot read. Every surface now names the one recovery,
// the file keeps parsing, and either branch of the fix lets sync and the local
// checks pass.
func TestCodexWithoutItsRecordIsRefusedAndKeepsParsing(t *testing.T) {
	const recovery = "Restore config.toml.aigw-state.json in its folder from a copy of it, or take the [model_providers.aigw] table and the lines marked \"# managed by AIGW\" out of config.toml, then run `aigw sync`"
	for name, fix := range map[string]func(*testing.T, projectedClient, []byte, []byte){
		"the record restored": func(t *testing.T, client projectedClient, _, record []byte) {
			t.Helper()
			writeFile(t, client.record, record, 0o600)
		},
		"AIGW's table taken out": func(t *testing.T, client projectedClient, config, _ []byte) {
			t.Helper()
			table := regexp.MustCompile(`(?s)# >>> AIGW managed provider >>>.*?# <<< AIGW managed provider <<<\n?`)
			stripped := regexp.MustCompile(`(?m)^.*# managed by AIGW\n`).ReplaceAll(table.ReplaceAll(config, nil), nil)
			writeFile(t, client.file, stripped, 0o600)
		},
	} {
		t.Run(name, func(t *testing.T) {
			client := projectedClients[codex.ID](t)
			config, record := readFile(t, client.file), readFile(t, client.record)
			if err := os.Remove(client.record); err != nil {
				t.Fatal(err)
			}
			assertRecoveryOnEverySurface(t, client.app, client.output, codex.ID, recovery, client.directory(), client.surfaces())
			var document map[string]any
			if err := toml.Unmarshal(readFile(t, client.file), &document); err != nil {
				t.Fatalf("the refused file does not parse: %v", err)
			}
			fix(t, client, config, record)
			syncAndCheckPass(t, client.app, client.output, codex.ID)
			if err := toml.Unmarshal(readFile(t, client.file), &document); err != nil {
				t.Fatalf("the synchronized file does not parse: %v", err)
			}
			if err := cli.Execute(client.app, []string{"client", "disable", codex.ID}); err != nil {
				t.Fatalf("withdrawal after the recovery: %v\n%s", err, client.output)
			}
		})
	}
}
