// Package catalog preserves Codex-owned model metadata while adding aliases
// whose base identity is uniquely established by the installed client's table.
package catalog

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

// Document is a validated client catalogue, including fields AIGW does not own.
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

// Project adds the complete bundled table under the uniquely proven namespace.
// A known, unknown or ambiguous model requires no projection and returns nil.
// The returned base identifies the selected alias without another parse or guess.
func (d Document) Project(model string) ([]byte, string) {
	namespace, ok := d.namespace(model)
	if !ok {
		return nil, ""
	}
	models := slices.Clone(d.models)
	for _, slug := range slices.Sorted(maps.Keys(d.slugs)) {
		alias := namespace + "." + slug
		if _, present := d.slugs[alias]; present {
			continue
		}
		entry := maps.Clone(d.models[d.slugs[slug]])
		entry["slug"], _ = json.Marshal(alias)
		models = append(models, entry)
	}
	fields := maps.Clone(d.fields)
	// Parse admits only valid JSON; private metadata and string aliases keep
	// serialization infallible. Observation cannot mutate this document.
	fields["models"], _ = json.Marshal(models)
	data, _ := json.Marshal(fields)
	return append(data, '\n'), model[len(namespace)+1:]
}

func (d Document) namespace(model string) (string, bool) {
	if _, present := d.slugs[model]; present {
		return "", false
	}
	namespace := ""
	matches := 0
	for index := range len(model) {
		if model[index] != '.' || index == 0 {
			continue
		}
		if _, present := d.slugs[model[index+1:]]; present {
			namespace = model[:index]
			matches++
		}
	}
	return namespace, matches == 1
}
