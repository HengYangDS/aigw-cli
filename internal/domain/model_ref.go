package domain

import (
	"crypto/sha256"
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

// String returns the reference as `provider/model`.
func (ref ModelRef) String() string { return ref.provider + "/" + ref.id }

// Ref names model as `provider/model`.
func (model Model) Ref() ModelRef {
	return ModelRef{provider: model.Provider, id: model.UpstreamModelID()}
}

// ResolveModelRef returns the key of the configured model that ref names: the
// one model of ref's provider whose provider model ID equals ref's. It is
// the one lookup from a reference to a stored model, so the store key can
// change without its callers.
func (c *Config) ResolveModelRef(ref ModelRef) (string, bool) {
	for _, key := range c.ModelIDs() {
		if model := c.Models[key]; model.Provider == ref.provider && model.UpstreamModelID() == ref.id {
			return key, true
		}
	}
	return "", false
}

// ModelKey derives the key under which a model that ref names is stored. It
// is deterministic in the provider and model ID and never appears in output:
// a readable part of both, then a digest of the reference itself, so that two
// IDs that read alike still get distinct keys within the identifier rule.
func ModelKey(ref ModelRef) string {
	return identifierFor(ref.provider+"-"+ref.id, ref.String())
}

// FamilyID derives the ID of the family that a model forms by itself: its
// model ID when that is a valid identifier, otherwise a readable part of it
// and a digest of the whole ID.
func FamilyID(id string) string {
	if ValidIdentifier(id) {
		return id
	}
	return identifierFor(id, id)
}

// identifierFor makes a valid identifier from readable and a digest of
// identity: the letters, digits, dots, hyphens and underscores of readable,
// any other run of characters as one hyphen, shortened to leave room for the
// digest's first twelve hexadecimal digits.
func identifierFor(readable, identity string) string {
	var slug strings.Builder
	for _, character := range readable {
		switch {
		case character < 0x80 && (character == '.' || character == '-' || character == '_' ||
			'0' <= character && character <= '9' || 'a' <= character && character <= 'z' || 'A' <= character && character <= 'Z'):
			slug.WriteRune(character)
		case !strings.HasSuffix(slug.String(), "-"):
			slug.WriteByte('-')
		}
	}
	const digestLength, maximumLength = 12, 64
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))[:digestLength]
	prefix := strings.Trim(slug.String(), ".-_")
	prefix = strings.TrimRight(prefix[:min(len(prefix), maximumLength-digestLength-1)], ".-_")
	if prefix == "" {
		return "m" + digest
	}
	return prefix + "-" + digest
}
