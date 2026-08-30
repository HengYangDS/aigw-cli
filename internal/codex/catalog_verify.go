package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ModelCatalogProbe records direct evidence from a Codex model catalog. The
// digest covers the complete model entry after removing only its slug, so an
// alias proves semantic identity without depending on a private config export,
// prompt layout, or a hand-maintained list of model fields.
type ModelCatalogProbe struct {
	Model          string `json:"model"`
	Present        bool   `json:"present"`
	MetadataSHA256 string `json:"metadata_sha256,omitempty"`
}

func (p ModelCatalogProbe) String() string {
	if !p.Present {
		return "absent"
	}
	return "present, metadata sha256 " + p.MetadataSHA256
}

// ModelCatalogVerification separates four claims: the installed client knows
// the base slug, does not already know the provider-prefixed alias, loads the
// generated catalog, and leaves an unrelated unknown id absent.
type ModelCatalogVerification struct {
	ClientVersion string            `json:"client_version"`
	ClientSHA256  string            `json:"client_sha256"`
	Model         string            `json:"model"`
	BaseSlug      string            `json:"base_slug"`
	Reference     ModelCatalogProbe `json:"reference"`
	Unadapted     ModelCatalogProbe `json:"unadapted"`
	Adapted       ModelCatalogProbe `json:"adapted"`
	Unknown       ModelCatalogProbe `json:"unknown"`
}

const unknownProbeModel = "aigw-model-catalog-verification-no-such-model"

// Check reports whether the direct catalog observations prove the projection.
func (v ModelCatalogVerification) Check() error {
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

// VerifyModelCatalog uses only the current public `codex debug models` surface.
// Every invocation gets a throwaway CODEX_HOME, and the configured probe names
// the generated catalog explicitly, so the user's configuration is neither
// read nor changed and no model request is sent.
func VerifyModelCatalog(executable, model string) (ModelCatalogVerification, error) {
	client, bundled, err := codexBundledCatalog(executable)
	if err != nil {
		return ModelCatalogVerification{}, err
	}
	verification := ModelCatalogVerification{
		ClientVersion: client.Version,
		ClientSHA256:  client.SHA256,
		Model:         model,
	}
	bundledDocument, err := decodeCodexCatalog(bundled, "bundled")
	if err != nil {
		return verification, err
	}
	slugs, _ := codexCatalogSlugs(bundledDocument)
	namespace, ok := codexCatalogNamespace(model, slugs)
	if !ok {
		return verification, fmt.Errorf("no unique Codex model matches %q, so AIGW projects no catalog for it", model)
	}
	verification.BaseSlug = strings.TrimPrefix(model, namespace+".")
	catalog, _ := buildCodexCatalog(bundled, model)
	directory, err := os.MkdirTemp("", "aigw-codex-catalog-verify-")
	if err != nil {
		return verification, fmt.Errorf("create Codex verification directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(directory) }()
	catalogPath := filepath.Join(directory, "model-catalog.json")
	if err := os.WriteFile(catalogPath, catalog, 0o600); err != nil {
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

func probeCodexCatalog(executable, catalogPath string) (codexCatalogDocument, error) {
	args := []string{"debug", "models"}
	if catalogPath != "" {
		quoted, err := codexTOMLString(catalogPath)
		if err != nil {
			return codexCatalogDocument{}, err
		}
		args = append(args, "-c", "model_catalog_json="+quoted)
	}
	output, err := runCodexReadOnly(executable, args...)
	if err != nil {
		return codexCatalogDocument{}, err
	}
	return decodeCodexCatalog(output, "effective")
}

func decodeCodexCatalog(data []byte, kind string) (codexCatalogDocument, error) {
	var document codexCatalogDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return codexCatalogDocument{}, fmt.Errorf("parse Codex %s model catalog: %w", kind, err)
	}
	if _, err := codexCatalogSlugs(document); err != nil {
		return codexCatalogDocument{}, fmt.Errorf("validate Codex %s model catalog: %w", kind, err)
	}
	return document, nil
}

func catalogProbe(document codexCatalogDocument, model string) ModelCatalogProbe {
	probe := ModelCatalogProbe{Model: model}
	for _, entry := range document.Models {
		rawSlug := entry["slug"]
		var slug string
		_ = json.Unmarshal(rawSlug, &slug)
		if slug != model {
			continue
		}
		metadata := make(map[string]json.RawMessage, len(entry)-1)
		for key, value := range entry {
			if key != "slug" {
				metadata[key] = value
			}
		}
		canonical, _ := json.Marshal(metadata)
		digest := sha256.Sum256(canonical)
		probe.Present = true
		probe.MetadataSHA256 = hex.EncodeToString(digest[:])
		return probe
	}
	return probe
}
