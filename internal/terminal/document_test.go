package terminal

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"

	"aigw-cli/internal/store"

	"github.com/pelletier/go-toml/v2"
)

func TestWriteDocumentHeadsTheObject(t *testing.T) {
	var out bytes.Buffer
	document := struct {
		Name  string   `json:"name"`
		Items []string `json:"items"`
	}{Name: "a <b>", Items: []string{}}
	if err := WriteDocument(&out, 3, false, document); err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"schema_version\": 3,\n  \"ok\": false,\n  \"name\": \"a \\u003cb\\u003e\",\n  \"items\": []\n}\n"; out.String() != want {
		t.Fatalf("document = %q, want %q", out.String(), want)
	}
	var encoded bytes.Buffer
	if err := WriteJSON(&encoded, document); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out.String(), strings.TrimPrefix(encoded.String(), "{\n")) {
		t.Fatalf("the header changed the document's own fields:\n%s\n%s", out.String(), encoded.String())
	}
	out.Reset()
	if err := WriteDocument(&out, 1, true, struct{}{}); err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"schema_version\": 1,\n  \"ok\": true\n}\n"; out.String() != want {
		t.Fatalf("empty document = %q, want %q", out.String(), want)
	}
	for name, refused := range map[string]any{
		"an array":           []string{"a"},
		"null":               nil,
		"its own ok":         map[string]bool{"ok": true},
		"its own version":    map[string]int{"schema_version": 1},
		"a value that fails": map[string]any{"f": func() {}},
	} {
		if err := WriteDocument(io.Discard, 1, true, refused); err == nil {
			t.Errorf("WriteDocument accepted %s", name)
		}
	}
}

func TestFailureDocumentNamesTheConfigurationStage(t *testing.T) {
	var decodeErr *toml.DecodeError
	if err := toml.Unmarshal([]byte("version = 7\n[accounts.team\n"), &map[string]any{}); !errors.As(err, &decodeErr) {
		t.Fatalf("fixture is not a TOML decode error: %v", err)
	}
	line, column := decodeErr.Position()
	for _, test := range []struct {
		name         string
		err          error
		stage        string
		line, column int
	}{
		{"read", &store.LoadError{Phase: store.LoadPhaseRead, Err: &os.PathError{Op: "open", Err: fs.ErrPermission}}, "configuration_read", 0, 0},
		{"parse", &store.LoadError{Phase: store.LoadPhaseParse, Err: decodeErr}, "configuration_parse", line, column},
		{"validate", &store.LoadError{Phase: store.LoadPhaseValidate, Err: errors.New("unsupported version")}, "configuration_validate", 0, 0},
		{"another error", errors.New("something else failed"), "", 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			renderer := NewRenderer(&out, false)
			RenderError(renderer, test.err, true)
			if err := renderer.Err(); err != nil {
				t.Fatal(err)
			}
			var failure struct {
				SchemaVersion *int    `json:"schema_version"`
				OK            *bool   `json:"ok"`
				Error         string  `json:"error"`
				Stage         *string `json:"stage"`
				Line          int     `json:"line"`
				Column        int     `json:"column"`
			}
			if err := json.Unmarshal(out.Bytes(), &failure); err != nil {
				t.Fatalf("failure is not one JSON document: %v\n%s", err, out.String())
			}
			if failure.SchemaVersion == nil || *failure.SchemaVersion != failureSchemaVersion || failure.OK == nil || *failure.OK || failure.Error == "" {
				t.Fatalf("failure header = %s", out.String())
			}
			stage := ""
			if failure.Stage != nil {
				stage = *failure.Stage
			}
			if stage != test.stage || failure.Line != test.line || failure.Column != test.column {
				t.Fatalf("stage = %q at %d:%d, want %q at %d:%d", stage, failure.Line, failure.Column, test.stage, test.line, test.column)
			}
			if test.stage == "" && strings.Contains(out.String(), "\"stage\"") {
				t.Fatalf("an unrelated failure names a stage: %s", out.String())
			}
		})
	}
}
