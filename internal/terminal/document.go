package terminal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// documentHeader heads every command's JSON document: the version of that
// command's document shape and whether the command did what it was asked.
type documentHeader struct {
	SchemaVersion int  `json:"schema_version"`
	OK            bool `json:"ok"`
}

// WriteDocument writes document as one indented JSON object headed by its
// command's schema version and outcome, followed by document's own fields in
// their order. document must encode as an object that names neither
// schema_version nor ok.
func WriteDocument(out io.Writer, version int, ok bool, document any) error {
	body, err := json.Marshal(document)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return fmt.Errorf("a JSON document must be an object: %w", err)
	}
	if fields == nil {
		return errors.New("a JSON document must be an object, not null")
	}
	for _, name := range []string{"schema_version", "ok"} {
		if _, found := fields[name]; found {
			return fmt.Errorf("a JSON document must not name its own %s", name)
		}
	}
	head, err := json.Marshal(documentHeader{SchemaVersion: version, OK: ok})
	if err != nil {
		return err
	}
	if len(fields) > 0 {
		head = append(head[:len(head)-1], ',')
		head = append(head, body[1:]...)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, head, "", "  "); err != nil {
		return err
	}
	indented.WriteByte('\n')
	_, err = out.Write(indented.Bytes())
	return err
}
