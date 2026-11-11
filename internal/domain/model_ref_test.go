package domain

import (
	"errors"
	"strings"
	"testing"
)

// TestParseModelRefSplitsAtTheFirstSlash holds the model argument's grammar:
// the provider is the text before the first slash and the rest is the
// provider's own model ID, which may contain a slash; an argument without a
// slash, an empty part and an invalid provider are refused.
func TestParseModelRefSplitsAtTheFirstSlash(t *testing.T) {
	for text, want := range map[string][2]string{
		"aihubmix/openai/gpt-oss": {"aihubmix", "openai/gpt-oss"},
		"dmxapi/gpt-6.1-sol":      {"dmxapi", "gpt-6.1-sol"},
		"ucloud/MiniMax-M3":       {"ucloud", "MiniMax-M3"},
		"team/a b":                {"team", "a b"},
	} {
		ref, err := ParseModelRef(text)
		if err != nil || ref.Provider() != want[0] || ref.ID() != want[1] || ref.String() != text {
			t.Fatalf("ParseModelRef(%q) = %q %q %q, %v; want %q", text, ref.Provider(), ref.ID(), ref.String(), err, want)
		}
	}
	if _, err := ParseModelRef("gpt-6.1-sol"); !errors.Is(err, ErrModelRefWithoutProvider) || !strings.Contains(err.Error(), `"gpt-6.1-sol" has no provider`) {
		t.Fatalf("an argument without a slash = %v", err)
	}
	for _, text := range []string{"/gpt", "dmxapi/", "/"} {
		if _, err := ParseModelRef(text); err == nil || errors.Is(err, ErrModelRefWithoutProvider) || !strings.Contains(err.Error(), "empty provider or model ID") {
			t.Fatalf("ParseModelRef(%q) = %v, want an empty-part refusal", text, err)
		}
	}
	for _, text := range []string{"bad provider/model", "-team/model", "té/model"} {
		if _, err := ParseModelRef(text); err == nil || !strings.Contains(err.Error(), "invalid provider") {
			t.Fatalf("ParseModelRef(%q) = %v, want an invalid-provider refusal", text, err)
		}
	}
}

// TestModelRefTextRoundTrips writes a reference as provider/model and reads
// it back, the zero reference as nothing, and refuses text without a
// provider.
func TestModelRefTextRoundTrips(t *testing.T) {
	ref := NewModelRef("aihubmix", "openai/gpt-oss")
	text, err := ref.MarshalText()
	if err != nil || string(text) != "aihubmix/openai/gpt-oss" {
		t.Fatalf("MarshalText = %q, %v", text, err)
	}
	var read ModelRef
	if err := read.UnmarshalText(text); err != nil || read != ref {
		t.Fatalf("UnmarshalText = %v, %v; want %v", read, err, ref)
	}
	if zero, err := (ModelRef{}).MarshalText(); err != nil || len(zero) != 0 || (ModelRef{}).String() != "" {
		t.Fatalf("the zero reference writes %q, %v", zero, err)
	}
	if err := read.UnmarshalText(nil); err != nil || !read.IsZero() {
		t.Fatalf("nothing reads as %v, %v", read, err)
	}
	if err := read.UnmarshalText([]byte("gpt-6.1-sol")); !errors.Is(err, ErrModelRefWithoutProvider) {
		t.Fatalf("text without a provider = %v", err)
	}
}

// TestAddedModelFamily takes an existing family whose ID equals the model ID
// ignoring case, and otherwise makes the model its own family, which needs no
// family entry and is never derived from a prefix.
func TestAddedModelFamily(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Families["gpt-6.1-sol"] = Family{Label: "GPT-6.1 Sol"}
	for id, want := range map[string]string{"GPT-6.1-Sol": "gpt-6.1-sol", "gpt-6.1-sol": "gpt-6.1-sol", "gpt-6.1-sol-cdx": "", "LongCat-2.5": ""} {
		if got := cfg.AddedModelFamily(id); got != want {
			t.Fatalf("AddedModelFamily(%q) = %q, want %q", id, got, want)
		}
	}
	if len(cfg.Families) != 1 {
		t.Fatalf("a model of its own family added a family: %v", cfg.Families)
	}
	own := Model{Provider: "dmxapi", ID: "LongCat-2.5"}
	if own.FamilyID() != "LongCat-2.5" {
		t.Fatalf("own family = %q", own.FamilyID())
	}
}
