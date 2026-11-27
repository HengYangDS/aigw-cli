package domain

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestValidateRejectsNonCanonicalSchemaVersion(t *testing.T) {
	cfg := validConfig()
	cfg.Version = 1
	err := cfg.Validate()
	var versionErr *UnsupportedConfigVersionError
	if !errors.As(err, &versionErr) {
		t.Fatalf("version 1 validation error = %v", err)
	}
	if versionErr.Version != 1 || versionErr.ExpectedVersion != ConfigVersion {
		t.Fatalf("version error = %#v", versionErr)
	}
	if !strings.Contains(err.Error(), "unsupported config version 1") {
		t.Fatalf("version error text = %q", err)
	}
}

// The refusal of a file of the schema that this one replaced names the edits
// that bring it here: for a configuration, the version that ConfigVersion
// sets and one edit for each client field that the schema renamed or
// removed; for a team file, which holds no clients, the version and the keys
// of its recommendations, and no client field. Any other version names none.
func TestTheReplacedSchemaIsRefusedWithItsEdits(t *testing.T) {
	// The edits are written for the schema just before this one: a schema that
	// moves ConfigVersion on writes its own edits and moves this version too.
	replaced := previousConfigVersion
	if replaced != ConfigVersion-1 {
		t.Fatalf("the edits are written for version %d, but version %d precedes the current schema", replaced, ConfigVersion-1)
	}
	version := "`version = " + strconv.Itoa(ConfigVersion) + "`"
	configuration := &UnsupportedConfigVersionError{Version: replaced, ExpectedVersion: ConfigVersion}
	team := &UnsupportedTeamFileVersionError{Version: replaced}
	for _, test := range []struct {
		name   string
		edits  []string
		keys   [][]string
		shown  []string
		absent []string
	}{
		{
			name: "configuration", edits: configurationEdits(), shown: []string{configuration.Edits(), configuration.Error()},
			keys: [][]string{{version}, {"`executable`", "`program`", "`extensions`"}, {"`authentication`", "`model_provider`"}, {"`forwarding`", "--via <root>"}},
		},
		{
			name: "team file", edits: teamFileEdits(), shown: []string{team.Edits()}, absent: []string{"`executable`", "`forwarding`", "--via"},
			keys: [][]string{{version}, {"`authentication`", "`model_provider`", "recommendation"}},
		},
	} {
		if len(test.edits) != len(test.keys) {
			t.Errorf("%s: %d edits %q, want %d", test.name, len(test.edits), test.edits, len(test.keys))
			continue
		}
		for index, edit := range test.edits {
			for _, key := range test.keys[index] {
				if !strings.Contains(edit, key) {
					t.Errorf("%s: the edit %q does not name %s", test.name, edit, key)
				}
			}
			for _, shown := range test.shown {
				if !strings.Contains(shown, edit) {
					t.Errorf("%s: %q does not name the edit %q", test.name, shown, edit)
				}
			}
		}
		for _, field := range test.absent {
			if strings.Contains(test.shown[0], field) {
				t.Errorf("%s: %q names %s", test.name, test.shown[0], field)
			}
		}
	}
	for _, other := range []int{1, replaced - 1, ConfigVersion, ConfigVersion + 1} {
		if edits := (&UnsupportedConfigVersionError{Version: other, ExpectedVersion: ConfigVersion}).Edits(); edits != "" {
			t.Errorf("version %d names the edits %q", other, edits)
		}
		if edits := (&UnsupportedTeamFileVersionError{Version: other}).Edits(); edits != "" {
			t.Errorf("team file version %d names the edits %q", other, edits)
		}
	}
}

func TestNewConfigUsesCurrentSchema(t *testing.T) {
	if got := NewConfig(testAdmission()).Version; got != ConfigVersion {
		t.Fatalf("new config version = %d, want %d", got, ConfigVersion)
	}
}
