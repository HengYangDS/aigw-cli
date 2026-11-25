// Package modelcatalog preserves Codex-owned model metadata while giving the
// selected model's entry its declared context window, and adding an alias for
// a model whose family the installed client's table holds.
package modelcatalog

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// The fields of an entry that this package reads and writes. Every other field
// is the client's and is carried as it is.
const (
	slugField        = "slug"
	displayNameField = "display_name"
	windowField      = "context_window"
	ceilingField     = "max_context_window"
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
		raw, present := entry[slugField]
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

// Project returns the client's complete table in which the entry that
// describes model carries window, in tokens, as its context_window and as its
// max_context_window, together with the slug of the client's entry that this
// entry's metadata comes from. A model that the table holds keeps its own
// entry, and that slug is model. Any other model gets an alias of family's
// entry, named model, and that slug is family. The two window fields are
// written whether or not the client's entry had them; every other field of the
// entry, every other entry and their order stay the client's. Metadata comes
// only from the model's own entry or its family's, never from a prefix or
// suffix of the model ID. An alias is shown as name, or as the family's name
// when name is empty; a name that another entry already shows, ignoring case,
// is followed by the model ID in parentheses, so that no two entries of the
// table share a display name.
//
// Project returns nothing, so that the client keeps its own table: for a model
// that the table does not hold and whose family is empty or not in the table
// either; for an alias that has no name left to tell it apart; and for a
// window that is not above zero, which is a caller's error, because every
// resolved selection has one above zero.
func (d Document) Project(model, family, name string, window int) ([]byte, string) {
	base := d.DescribedBy(model, family)
	if base == "" || window <= 0 {
		return nil, ""
	}
	entry, models := withWindow(d.Model(base), window), slices.Clone(d.models)
	if base == model {
		models[d.slugs[model]] = entry
		return d.withModels(models), base
	}
	if name == "" {
		_ = json.Unmarshal(entry[displayNameField], &name)
	}
	if name != "" {
		if d.shows(name) {
			name += " (" + model + ")"
		}
		if d.shows(name) {
			return nil, ""
		}
		entry[displayNameField], _ = json.Marshal(name)
	}
	entry[slugField], _ = json.Marshal(model)
	return d.withModels(append(models, entry)), base
}

// DescribedBy returns the slug of the entry of the table that describes model,
// of family: model, where the table holds an entry of that ID, which is the
// one that Codex reads for it; else family, where the table holds that entry,
// from which Project makes the model's alias; and else nothing. Nothing
// describes a model without an ID, and no prefix or suffix of an ID is looked
// up.
func (d Document) DescribedBy(model, family string) string {
	if model == "" {
		return ""
	}
	for _, slug := range [...]string{model, family} {
		if _, present := d.slugs[slug]; present {
			return slug
		}
	}
	return ""
}

// ProjectedFields returns the fields that Project gives the entry which
// describes a model, and which that entry so does not take from its base: the
// two window fields, and for an alias also its slug and its display name.
func ProjectedFields(alias bool) []string {
	if alias {
		return []string{slugField, displayNameField, windowField, ceilingField}
	}
	return []string{windowField, ceilingField}
}

// withWindow sets the two window fields of entry, a copy that the caller owns.
func withWindow(entry map[string]json.RawMessage, window int) map[string]json.RawMessage {
	for _, field := range ProjectedFields(false) {
		entry[field], _ = json.Marshal(window)
	}
	return entry
}

// ContextWindow returns the context window and its ceiling, in tokens, that
// slug's entry carries as context_window and max_context_window. A field that
// is absent or is not a whole number above zero, and either field of an entry
// that the table does not hold, is zero.
func (d Document) ContextWindow(slug string) (window, ceiling int) {
	entry := d.Model(slug)
	read := func(field string) int {
		var tokens int
		if json.Unmarshal(entry[field], &tokens) != nil || tokens < 0 {
			return 0
		}
		return tokens
	}
	return read(windowField), read(ceilingField)
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
	if index, present := d.slugs[slug]; !present || json.Unmarshal(d.models[index][displayNameField], &name) != nil || name == "" {
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
		_ = json.Unmarshal(entry[slugField], &slug)
		if slug != except && json.Unmarshal(entry[displayNameField], &shown) == nil && strings.EqualFold(shown, name) {
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
