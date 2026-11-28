// Package modelcatalog preserves Codex-owned model metadata while listing the
// models that AIGW lists for a client, each with its declared context window
// and an alias for a model whose family the installed client's table holds,
// and hiding every other entry from Codex's picker.
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
	visibilityField  = "visibility"
	// listVisibility is the one visibility that shows an entry in Codex's
	// picker; hideVisibility is the one that AIGW gives an entry it does not list.
	listVisibility = "list"
	hideVisibility = "hide"
)

// Listing is one model that the table lists: its ID, its family, the name
// that an alias of it shows (empty for the family's own), and its context
// window in tokens (zero for the window that its entry already carries).
type Listing struct {
	Model, Family, Name string
	Window              int
}

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

// Project returns the client's table that lists models, in their order, and
// hides every other entry that the client lists. A model is listed by the
// entry that describes it (DescribedBy): its own, or an alias of its family's,
// named model and appended. The listed entry carries the model's window as
// its context_window and its max_context_window, where that window is above
// zero. An alias is shown as the model's name, or as the family's name when
// that is empty; a name that another entry already shows, hidden or not and
// ignoring case, is followed by the model ID in parentheses, and a model with
// no name left to tell it apart gets no alias. A model that the table
// describes by neither entry gets none either: metadata comes only from the
// model's own entry or its family's, never from a prefix or suffix of its ID.
// An entry that the client hides stays hidden, and so does an alias of it.
// Every other field of every entry, and the order of the client's entries,
// stay the client's. Project returns nothing for no models.
func (d Document) Project(models []Listing) []byte {
	if len(models) == 0 {
		return nil
	}
	table, listed := slices.Clone(d.models), map[string]bool{}
	for _, model := range models {
		base := d.DescribedBy(model.Model, model.Family)
		if base == "" {
			continue
		}
		entry := d.Model(base)
		if model.Window > 0 {
			entry = withWindow(entry, model.Window)
		}
		if base == model.Model {
			table[d.slugs[base]], listed[base] = entry, true
			continue
		}
		name := model.Name
		if name == "" {
			_ = json.Unmarshal(entry[displayNameField], &name)
		}
		if name != "" {
			if len(showing(table, name, "")) > 0 {
				name += " (" + model.Model + ")"
			}
			if len(showing(table, name, "")) > 0 {
				continue
			}
			entry[displayNameField], _ = json.Marshal(name)
		}
		entry[slugField], _ = json.Marshal(model.Model)
		table, listed[model.Model] = append(table, entry), true
	}
	for index, entry := range table {
		if !listed[slugOf(entry)] && visibility(entry) == listVisibility {
			hidden := maps.Clone(entry)
			hidden[visibilityField], _ = json.Marshal(hideVisibility)
			table[index] = hidden
		}
	}
	return d.withModels(table)
}

// WithWindows returns the table in which the own entry of each of models
// carries the model's window above zero in both window fields, and nothing else
// changes; nothing when the table holds no entry of any of their IDs.
func (d Document) WithWindows(models []Listing) []byte {
	table, found := slices.Clone(d.models), false
	for _, model := range models {
		if index, present := d.slugs[model.Model]; present && model.Window > 0 {
			table[index], found = withWindow(d.Model(model.Model), model.Window), true
		}
	}
	if !found {
		return nil
	}
	return d.withModels(table)
}

// Listed returns the slugs of the entries that the table shows in Codex's
// picker, in table order.
func (d Document) Listed() []string {
	var slugs []string
	for _, entry := range d.models {
		if visibility(entry) == listVisibility {
			slugs = append(slugs, slugOf(entry))
		}
	}
	return slugs
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

// SharingDisplayName returns the slugs of the other entries whose display
// name is slug's, ignoring case, in table order.
func (d Document) SharingDisplayName(slug string) []string {
	var name string
	if index, present := d.slugs[slug]; !present || json.Unmarshal(d.models[index][displayNameField], &name) != nil || name == "" {
		return nil
	}
	return showing(d.models, name, slug)
}

// showing returns the slugs of the entries of models but except whose display
// name is name, ignoring case.
func showing(models []map[string]json.RawMessage, name, except string) []string {
	var slugs []string
	for _, entry := range models {
		var shown string
		if slug := slugOf(entry); slug != except && json.Unmarshal(entry[displayNameField], &shown) == nil && strings.EqualFold(shown, name) {
			slugs = append(slugs, slug)
		}
	}
	return slugs
}

// slugOf is entry's slug, which Parse admits only as a string.
func slugOf(entry map[string]json.RawMessage) string {
	var slug string
	_ = json.Unmarshal(entry[slugField], &slug)
	return slug
}

// visibility is entry's visibility, or empty where it has none as a string.
func visibility(entry map[string]json.RawMessage) string {
	var value string
	_ = json.Unmarshal(entry[visibilityField], &value)
	return value
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
