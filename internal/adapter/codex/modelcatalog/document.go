// Package modelcatalog preserves Codex-owned model metadata while adding an
// alias for a model whose family the installed client's table holds.
package modelcatalog

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Document is a validated client catalog, including fields AIGW does not own.
type Document struct {
	fields map[string]json.RawMessage
	models []map[string]json.RawMessage
	slugs  map[string]int
}

// Parse validates model identity without discarding client-owned metadata.
func Parse(data []byte) (Document, error) {
	var document Document
	if err := json.Unmarshal(data, &document.fields); err != nil {
		return Document{}, fmt.Errorf("parse Codex model catalog: %w", err)
	}
	if err := json.Unmarshal(document.fields["models"], &document.models); err != nil {
		return Document{}, fmt.Errorf("parse Codex model catalog entries: %w", err)
	}
	if len(document.models) == 0 {
		return Document{}, fmt.Errorf("Codex model catalog is empty")
	}
	document.slugs = make(map[string]int, len(document.models))
	for index, entry := range document.models {
		raw, present := entry["slug"]
		if !present {
			return Document{}, fmt.Errorf("Codex model catalog entry %d has no slug", index)
		}
		var slug string
		if err := json.Unmarshal(raw, &slug); err != nil {
			return Document{}, fmt.Errorf("parse Codex model catalog entry %d slug: %w", index, err)
		}
		if slug == "" {
			return Document{}, fmt.Errorf("Codex model catalog entry %d has an empty slug", index)
		}
		if _, duplicate := document.slugs[slug]; duplicate {
			return Document{}, fmt.Errorf("Codex model catalog declares slug %q twice", slug)
		}
		document.slugs[slug] = index
	}
	return document, nil
}

// Model returns an independent copy of one complete model entry, or nil.
func (d Document) Model(slug string) map[string]json.RawMessage {
	index, present := d.slugs[slug]
	if !present {
		return nil
	}
	entry := make(map[string]json.RawMessage, len(d.models[index]))
	for key, value := range d.models[index] {
		entry[key] = slices.Clone(value)
	}
	return entry
}

// Project returns the client's complete table with an alias named model whose
// metadata is family's entry, and family as the alias's base. Metadata comes
// only from the model's family, never from a prefix or suffix of the model ID:
// a model that the table already holds, a model without a family, and a model
// whose family the table does not hold get nothing, so the client keeps its
// own behavior for them. The alias is shown as name, or as the family's name
// when name is empty; a name that another entry already shows, ignoring case,
// is followed by the model ID in parentheses, so that no two entries of the
// table share a display name.
func (d Document) Project(model, family, name string) ([]byte, string) {
	if model == "" || family == "" {
		return nil, ""
	}
	if _, present := d.slugs[model]; present {
		return nil, ""
	}
	entry := d.Model(family)
	if entry == nil {
		return nil, ""
	}
	if name == "" {
		_ = json.Unmarshal(entry["display_name"], &name)
	}
	if name != "" {
		if d.shows(name) {
			name += " (" + model + ")"
		}
		if d.shows(name) {
			return nil, ""
		}
		entry["display_name"], _ = json.Marshal(name)
	}
	entry["slug"], _ = json.Marshal(model)
	return d.withModels(append(slices.Clone(d.models), entry)), family
}

// shows reports whether an entry of the table has name as its display name,
// ignoring case.
func (d Document) shows(name string) bool {
	return len(d.showing(name, "")) > 0
}

// SharingDisplayName returns the slugs of the other entries whose display
// name is slug's, ignoring case, in table order.
func (d Document) SharingDisplayName(slug string) []string {
	var name string
	if index, present := d.slugs[slug]; !present || json.Unmarshal(d.models[index]["display_name"], &name) != nil || name == "" {
		return nil
	}
	return d.showing(name, slug)
}

// showing returns the slugs of the entries but except whose display name is
// name, ignoring case.
func (d Document) showing(name, except string) []string {
	var slugs []string
	for _, entry := range d.models {
		var shown, slug string
		_ = json.Unmarshal(entry["slug"], &slug)
		if slug != except && json.Unmarshal(entry["display_name"], &shown) == nil && strings.EqualFold(shown, name) {
			slugs = append(slugs, slug)
		}
	}
	return slugs
}

// withModels serializes the original document with its model table replaced.
func (d Document) withModels(models []map[string]json.RawMessage) []byte {
	fields := maps.Clone(d.fields)
	// Parse admits only valid JSON; private metadata and string aliases keep
	// serialization infallible. Observation cannot mutate this document.
	fields["models"], _ = json.Marshal(models)
	data, _ := json.Marshal(fields)
	return append(data, '\n')
}
