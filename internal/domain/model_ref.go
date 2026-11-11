package domain

import (
	"errors"
	"fmt"
	"strings"
)

// ModelRef names one provider's model as `provider/model`: the text before the
// first slash is the provider's ID, and the rest is the provider's own model
// ID, byte for byte, so a model ID may contain a slash itself.
type ModelRef struct {
	provider string
	id       string
}

// ErrModelRefWithoutProvider refuses a model argument without a slash, which
// names a model ID but no provider.
var ErrModelRefWithoutProvider = errors.New("a model is named provider/model")

// ParseModelRef splits a `provider/model` argument at its first slash. Both
// parts must be non-empty, and the provider must be a valid provider ID.
func ParseModelRef(text string) (ModelRef, error) {
	provider, id, found := strings.Cut(text, "/")
	switch {
	case !found:
		return ModelRef{}, fmt.Errorf("%q has no provider: %w", text, ErrModelRefWithoutProvider)
	case provider == "" || id == "":
		return ModelRef{}, fmt.Errorf("%q has an empty provider or model ID; a model is named provider/model", text)
	case !ValidIdentifier(provider):
		return ModelRef{}, fmt.Errorf("%q names the invalid provider %q; use letters, numbers, dots, hyphens or underscores", text, provider)
	}
	return ModelRef{provider: provider, id: id}, nil
}

// Provider returns the ID of the provider that serves the model.
func (ref ModelRef) Provider() string { return ref.provider }

// ID returns the provider's own model ID.
func (ref ModelRef) ID() string { return ref.id }

// String returns the reference as `provider/model`, and the zero reference as
// nothing.
func (ref ModelRef) String() string {
	if ref.IsZero() {
		return ""
	}
	return ref.provider + "/" + ref.id
}

// NewModelRef names the model with the provider's own model ID id, such as an
// ID that the provider's catalog lists.
func NewModelRef(provider, id string) ModelRef { return ModelRef{provider: provider, id: id} }

// IsZero reports whether ref names no model, as a client without a selection
// has.
func (ref ModelRef) IsZero() bool { return ref == ModelRef{} }

// MarshalText writes the reference as `provider/model`, and the zero
// reference as nothing.
func (ref ModelRef) MarshalText() ([]byte, error) {
	if ref.IsZero() {
		return nil, nil
	}
	return []byte(ref.String()), nil
}

// UnmarshalText reads a `provider/model` reference; nothing reads as the zero
// reference.
func (ref *ModelRef) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*ref = ModelRef{}
		return nil
	}
	parsed, err := ParseModelRef(string(text))
	if err != nil {
		return err
	}
	*ref = parsed
	return nil
}

// Ref names model as `provider/model`.
func (model Model) Ref() ModelRef {
	return ModelRef{provider: model.Provider, id: model.ID}
}

// AddedModelFamily is the family of a model with the provider model ID id
// that is added without a family: an existing family whose ID equals id,
// ignoring case, or else none, for a model that is its own family. It never
// derives a family from a prefix or another part of id, because a family's
// metadata then describes the model.
func (c *Config) AddedModelFamily(id string) string {
	for _, family := range c.FamilyIDs() {
		if strings.EqualFold(family, id) {
			return family
		}
	}
	return ""
}
