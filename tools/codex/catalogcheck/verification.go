package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/codex/modelcatalog"
	"aigw-cli/internal/platform"

	"github.com/rogpeppe/go-internal/robustio"
)

// modelProbe records direct evidence from a Codex model catalog. The digest
// covers the complete model entry after removing what the entry that describes
// a model does not take from its base: the two window fields, which carry the
// model's declared context window, and for an alias also its slug and its
// display name, which AIGW gives it so that it is not shown under its family's
// name. An entry so proves semantic identity without depending on a private
// config export, prompt layout, or a hand-maintained list of model fields.
type modelProbe struct {
	Model          string `json:"model"`
	Present        bool   `json:"present"`
	MetadataSHA256 string `json:"metadata_sha256,omitempty"`
}

// verificationResult separates the claims about the entry that describes the
// model, an alias of its family's entry or the model's own entry, which is
// the case when BaseSlug is the model: the installed client's table holds the
// base, and holds no alias already; the client loads the generated catalog;
// the entry that it rendered is the base's apart from the fields that the
// digest leaves out, and carries the declared window in both window fields;
// an alias is shown under a name no other entry has; and an unrelated unknown
// id stays absent.
type verificationResult struct {
	ClientVersion string `json:"client_version"`
	ClientSHA256  string `json:"client_sha256"`
	Model         string `json:"model"`
	BaseSlug      string `json:"base_slug"`
	// DeclaredWindow is the context window, in tokens, that the model declares.
	// ContextWindow and MaxContextWindow are the two numbers read from the
	// entry that the client rendered, zero for a field that it did not render
	// as a whole number above zero.
	DeclaredWindow   int        `json:"declared_context_window"`
	ContextWindow    int        `json:"context_window"`
	MaxContextWindow int        `json:"max_context_window"`
	Reference        modelProbe `json:"reference"`
	Unadapted        modelProbe `json:"unadapted"`
	Adapted          modelProbe `json:"adapted"`
	Unknown          modelProbe `json:"unknown"`
	// SharedName lists the entries of the effective catalog that the client
	// shows under an alias's name.
	SharedName []string `json:"shared_name"`
}

const unknownProbeModel = "aigw-model-catalog-verification-no-such-model"

// Check reports whether the direct catalog observations prove the projection.
func (v verificationResult) Check() error {
	if !v.Reference.Present {
		return fmt.Errorf("base model %q is absent from the bundled catalog", v.BaseSlug)
	}
	if v.BaseSlug != v.Model && v.Unadapted.Present {
		return fmt.Errorf("%q already exists in the bundled catalog, so no AIGW alias is required", v.Model)
	}
	if !v.Adapted.Present {
		return fmt.Errorf("%q is absent from the effective catalog after loading the generated catalog", v.Model)
	}
	if v.Adapted.MetadataSHA256 != v.Reference.MetadataSHA256 {
		return fmt.Errorf("%q metadata digest %s differs from %q metadata digest %s", v.Model, v.Adapted.MetadataSHA256, v.BaseSlug, v.Reference.MetadataSHA256)
	}
	if v.DeclaredWindow <= 0 {
		return fmt.Errorf("the declared context window %d is not above zero", v.DeclaredWindow)
	}
	if v.ContextWindow != v.DeclaredWindow || v.MaxContextWindow != v.DeclaredWindow {
		return fmt.Errorf("the effective catalog gives %q context_window %d and max_context_window %d, want the declared context window %d in both", v.Model, v.ContextWindow, v.MaxContextWindow, v.DeclaredWindow)
	}
	if len(v.SharedName) > 0 {
		return fmt.Errorf("the effective catalog shows %q under the name of %s", v.Model, strings.Join(v.SharedName, ", "))
	}
	if v.Unknown.Present {
		return fmt.Errorf("%q unexpectedly exists in the effective catalog", unknownProbeModel)
	}
	return nil
}

var removeVerificationDirectory = robustio.RemoveAll

// verifyCatalog uses only the current public `codex debug models` surface.
// Every invocation gets a throwaway CODEX_HOME, and the configured probe names
// the generated catalog explicitly, so the user's configuration is neither
// read nor changed and no model request is sent. It projects the client's own
// table as AIGW does for model, of family, shown as name, with the declared
// context window: an alias for a model that the table does not hold, and the
// model's own entry, which takes neither family nor name, for one that it holds.
func verifyCatalog(executable, model, family, name string, window int) (verification verificationResult, result error) {
	client, bundled, err := codex.ReadBundledCatalog(executable)
	if err != nil {
		return verificationResult{}, err
	}
	verification = verificationResult{
		ClientVersion:  client.Version,
		ClientSHA256:   client.SHA256,
		Model:          model,
		DeclaredWindow: window,
	}
	bundledDocument, err := modelcatalog.Parse(bundled)
	if err != nil {
		return verification, err
	}
	projected, base := bundledDocument.Project(model, family, name, window)
	if projected == nil {
		return verification, fmt.Errorf("family %q does not prove a Codex catalog entry for %q, so AIGW projects no alias for it", family, model)
	}
	verification.BaseSlug = base
	directory, err := os.MkdirTemp("", "aigw-codex-catalog-verify-")
	if err != nil {
		return verification, fmt.Errorf("create Codex verification directory: %w", err)
	}
	defer func() {
		if err := removeVerificationDirectory(directory); err != nil {
			result = errors.Join(result, fmt.Errorf("remove Codex verification directory %s: %w", directory, err))
		}
	}()
	catalogPath := filepath.Join(directory, "model-catalog.json")
	if err := os.WriteFile(catalogPath, projected, 0o600); err != nil {
		return verification, fmt.Errorf("write Codex verification catalog: %w", err)
	}
	effectiveDocument, err := probeCodexCatalog(executable, catalogPath)
	if err != nil {
		return verification, err
	}
	alias := base != model
	own := modelcatalog.ProjectedFields(alias)
	verification.Reference = catalogProbe(bundledDocument, verification.BaseSlug, own)
	verification.Unadapted = catalogProbe(bundledDocument, model, own)
	verification.Adapted = catalogProbe(effectiveDocument, model, own)
	verification.ContextWindow, verification.MaxContextWindow = effectiveDocument.ContextWindow(model)
	if alias {
		verification.SharedName = effectiveDocument.SharingDisplayName(model)
	}
	verification.Unknown = catalogProbe(effectiveDocument, unknownProbeModel, own)
	return verification, nil
}

func probeCodexCatalog(executable, catalogPath string) (modelcatalog.Document, error) {
	output, err := codex.ReadEffectiveCatalog(executable, catalogPath)
	if err != nil {
		return modelcatalog.Document{}, err
	}
	document, err := modelcatalog.Parse(output)
	if err != nil {
		return modelcatalog.Document{}, fmt.Errorf("read Codex effective model catalog: %w", err)
	}
	return document, nil
}

// catalogProbe reports whether document holds model, and digests its entry
// without the fields in own.
func catalogProbe(document modelcatalog.Document, model string, own []string) modelProbe {
	probe := modelProbe{Model: model}
	metadata := document.Model(model)
	if metadata == nil {
		return probe
	}
	for _, field := range own {
		delete(metadata, field)
	}
	canonical, _ := json.Marshal(metadata)
	probe.Present = true
	probe.MetadataSHA256 = platform.SHA256(canonical)
	return probe
}
