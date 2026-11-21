package codex

import "testing"

// Codex's root selection lines are TOML, so every spelling that TOML gives a
// string names the same model: a basic string, a literal string, no spaces
// around the sign, a trailing comment, and a number sign inside the string. A
// line that assigns another key, or a value that is no string, names none.
// The tenth review found a pick written as a literal string read as "no
// choice", because a second reader decoded the value as JSON.
func TestASelectionLineIsReadAsTOML(t *testing.T) {
	for _, row := range []struct {
		line, key, value string
		assigned         bool
	}{
		{`model = "gpt-terra"`, "model", "gpt-terra", true},
		{`model = 'gpt-terra'`, "model", "gpt-terra", true},
		{`model="gpt-terra"`, "model", "gpt-terra", true},
		{`model = "gpt-terra" # mine`, "model", "gpt-terra", true},
		{`model = 'gpt-terra' # managed by AIGW`, "model", "gpt-terra", true},
		{`model = "team#gpt-terra"`, "model", "team#gpt-terra", true},
		{`model = "gpt-terra"`, "model", "gpt-terra", true},
		{`model = ""`, "model", "", true},
		{`model = 3`, "model", "", false},
		{`model_provider = "aigw"`, "model", "", false},
		{`model = "gpt-terra`, "model", "", false},
		{``, "model", "", false},
	} {
		value, assigned := selectionValue(row.line, row.key)
		if value != row.value || assigned != row.assigned {
			t.Errorf("selectionValue(%q, %q) = %q, %t; want %q, %t", row.line, row.key, value, assigned, row.value, row.assigned)
		}
		if has := selectionHasValue(row.line, row.key, row.value); has != row.assigned {
			t.Errorf("selectionHasValue(%q, %q, %q) = %t; want %t", row.line, row.key, row.value, has, row.assigned)
		}
	}
}
