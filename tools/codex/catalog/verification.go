package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"aigw-cli/internal/codex"
	"aigw-cli/internal/codex/catalog"

	"github.com/rogpeppe/go-internal/robustio"
)

// modelProbe records direct evidence from a Codex model catalog. The
// digest covers the complete model entry after removing only its slug, so an
// alias proves semantic identity without depending on a private config export,
// prompt layout, or a hand-maintained list of model fields.
type modelProbe struct {
	Model          string `json:"model"`
	Present        bool   `json:"present"`
	MetadataSHA256 string `json:"metadata_sha256,omitempty"`
}

// verificationResult separates four claims: the installed client knows
// the base slug, does not already know the provider-prefixed alias, loads the
// generated catalog, and leaves an unrelated unknown id absent.
type verificationResult struct {
	ClientVersion string     `json:"client_version"`
	ClientSHA256  string     `json:"client_sha256"`
	Model         string     `json:"model"`
	BaseSlug      string     `json:"base_slug"`
	Reference     modelProbe `json:"reference"`
	Unadapted     modelProbe `json:"unadapted"`
	Adapted       modelProbe `json:"adapted"`
	Unknown       modelProbe `json:"unknown"`
}

const unknownProbeModel = "aigw-model-catalog-verification-no-such-model"

// Check reports whether the direct catalog observations prove the projection.
func (v verificationResult) Check() error {
	if !v.Reference.Present {
		return fmt.Errorf("base model %q is absent from the bundled catalog", v.BaseSlug)
	}
	if v.Unadapted.Present {
		return fmt.Errorf("%q already exists in the bundled catalog, so no AIGW alias is required", v.Model)
	}
	if !v.Adapted.Present {
		return fmt.Errorf("%q is absent from the effective catalog after loading the generated catalog", v.Model)
	}
	if v.Adapted.MetadataSHA256 != v.Reference.MetadataSHA256 {
		return fmt.Errorf("%q metadata digest %s differs from %q metadata digest %s", v.Model, v.Adapted.MetadataSHA256, v.BaseSlug, v.Reference.MetadataSHA256)
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
// read nor changed and no model request is sent.
func verifyCatalog(executable, model string) (verification verificationResult, result error) {
	client, bundled, err := codex.ReadBundledCatalog(executable)
	if err != nil {
		return verificationResult{}, err
	}
	verification = verificationResult{
		ClientVersion: client.Version,
		ClientSHA256:  client.SHA256,
		Model:         model,
	}
	bundledDocument, err := catalog.Parse(bundled)
	if err != nil {
		return verification, err
	}
	projected, base := bundledDocument.Project(model)
	if projected == nil {
		return verification, fmt.Errorf("no unique Codex model matches %q, so AIGW projects no catalog for it", model)
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
	verification.Reference = catalogProbe(bundledDocument, verification.BaseSlug)
	verification.Unadapted = catalogProbe(bundledDocument, model)
	verification.Adapted = catalogProbe(effectiveDocument, model)
	verification.Unknown = catalogProbe(effectiveDocument, unknownProbeModel)
	return verification, nil
}

func probeCodexCatalog(executable, catalogPath string) (catalog.Document, error) {
	output, err := codex.ReadEffectiveCatalog(executable, catalogPath)
	if err != nil {
		return catalog.Document{}, err
	}
	document, err := catalog.Parse(output)
	if err != nil {
		return catalog.Document{}, fmt.Errorf("read Codex effective model catalog: %w", err)
	}
	return document, nil
}

func catalogProbe(document catalog.Document, model string) modelProbe {
	probe := modelProbe{Model: model}
	metadata := document.Model(model)
	if metadata == nil {
		return probe
	}
	delete(metadata, "slug")
	canonical, _ := json.Marshal(metadata)
	digest := sha256.Sum256(canonical)
	probe.Present = true
	probe.MetadataSHA256 = hex.EncodeToString(digest[:])
	return probe
}
