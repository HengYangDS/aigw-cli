package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ModelCatalogProbe is what one client invocation reveals about the metadata it
// selected for one model. The numbers are read from the input the client says it
// would send, so they describe the client's own resolution rather than any
// AIGW-side assumption. Placeholder metadata yields different instructions and
// drops the session features a model's own metadata enables, which is what makes
// these three numbers able to separate the states.
type ModelCatalogProbe struct {
	Model        string `json:"model"`
	Instructions int    `json:"instructions"`
	Items        int    `json:"items"`
	MultiAgent   int    `json:"multi_agent"`
}

func (p ModelCatalogProbe) String() string {
	return fmt.Sprintf("%d instruction bytes, %d input items, %d multi-agent mentions", p.Instructions, p.Items, p.MultiAgent)
}

func (p ModelCatalogProbe) same(other ModelCatalogProbe) bool {
	return p.Instructions == other.Instructions && p.Items == other.Items && p.MultiAgent == other.MultiAgent
}

// ModelCatalogVerification is the evidence that an installed client resolves a
// provider-prefixed model id exactly as it resolves the slug that id wraps, and
// that a model the client genuinely does not know still falls back.
type ModelCatalogVerification struct {
	ClientVersion string `json:"client_version"`
	ClientSHA256  string `json:"client_sha256"`
	Model         string `json:"model"`
	BaseSlug      string `json:"base_slug"`
	// Reference is the base slug read through the client's own bundled table:
	// the behavior a bare-name profile already gets.
	Reference ModelCatalogProbe `json:"reference"`
	// Unadapted is the prefixed id without a catalog: the reported defect.
	Unadapted ModelCatalogProbe `json:"unadapted"`
	// Adapted is the prefixed id through the generated catalog: the claim.
	Adapted ModelCatalogProbe `json:"adapted"`
	// Unknown is a model the client cannot know, asked with the same generated
	// catalog in place, so the alias set is shown not to answer for ids it was
	// never given.
	Unknown ModelCatalogProbe `json:"unknown"`
}

// unknownProbeModel is an id no client can describe. It is asked for alongside
// the real one so a silenced warning cannot pass as a fixed one.
const unknownProbeModel = "aigw-model-catalog-verification-no-such-model"

// Check reports whether the measurements support the claim. It is separate from
// the measurement so a caller can present the numbers either way.
func (v ModelCatalogVerification) Check() error {
	if v.Unadapted.same(v.Reference) {
		return fmt.Errorf("%q already resolves like %q without a catalog, so this measurement cannot show what a catalog changes", v.Model, v.BaseSlug)
	}
	if !v.Adapted.same(v.Reference) {
		return fmt.Errorf("%q resolved %s through the generated catalog, but %q resolves %s", v.Model, v.Adapted, v.BaseSlug, v.Reference)
	}
	if !v.Unknown.same(v.Unadapted) {
		return fmt.Errorf("%q resolved %s through the generated catalog instead of the client's fallback %s", unknownProbeModel, v.Unknown, v.Unadapted)
	}
	return nil
}

// VerifyModelCatalog measures how an installed client resolves one
// provider-prefixed model id, with and without the catalog AIGW would project
// for it. It writes nothing outside temporary directories, reads throwaway
// client homes, and never sends anything to a model, so it costs no model
// request.
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
	var document codexCatalogDocument
	if err := json.Unmarshal(bundled, &document); err != nil {
		return verification, fmt.Errorf("parse Codex bundled model catalog: %w", err)
	}
	slugs, err := codexCatalogSlugs(document)
	if err != nil {
		return verification, err
	}
	namespace, ok := codexCatalogNamespace(model, slugs)
	if !ok {
		return verification, fmt.Errorf("no unique Codex model matches %q, so AIGW projects no catalog for it", model)
	}
	verification.BaseSlug = strings.TrimPrefix(model, namespace+".")
	catalog, err := buildCodexCatalog(bundled, model)
	if err != nil {
		return verification, err
	}
	if catalog == nil {
		return verification, fmt.Errorf("AIGW projects no catalog for %q", model)
	}
	directory, err := os.MkdirTemp("", "aigw-codex-catalog-verify-")
	if err != nil {
		return verification, fmt.Errorf("create Codex verification directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(directory) }()
	catalogPath := filepath.Join(directory, "model-catalog.json")
	if err := os.WriteFile(catalogPath, catalog, 0o600); err != nil {
		return verification, fmt.Errorf("write Codex verification catalog: %w", err)
	}
	for _, probe := range []struct {
		into    *ModelCatalogProbe
		model   string
		catalog string
	}{
		{&verification.Reference, verification.BaseSlug, ""},
		{&verification.Unadapted, model, ""},
		{&verification.Adapted, model, catalogPath},
		{&verification.Unknown, unknownProbeModel, catalogPath},
	} {
		measured, err := probeCodexModel(executable, probe.model, probe.catalog)
		if err != nil {
			return verification, err
		}
		*probe.into = measured
	}
	return verification, nil
}

// resolvedInstructions matches the base instructions in a configuration the
// client exported for one invocation. The value spans lines, so the match starts
// at a line boundary and runs to its closing delimiter; TOML admits either
// multiline delimiter and the client picks whichever its content allows.
var resolvedInstructions = regexp.MustCompile(`(?ms)^instructions = ("""|''')(.*?)("""|''')`)

// probeCodexModel asks the client which input it would send for one model, and
// which configuration it resolved to produce it.
func probeCodexModel(executable, model, catalogPath string) (ModelCatalogProbe, error) {
	exportDir, err := os.MkdirTemp("", "aigw-codex-catalog-export-")
	if err != nil {
		return ModelCatalogProbe{}, fmt.Errorf("create Codex export directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(exportDir) }()
	args := []string{
		"debug", "prompt-input", "hi",
		"-c", "model=" + model,
		"-c", "debug.config_lockfile.export_dir=" + exportDir,
	}
	if catalogPath != "" {
		args = append(args, "-c", "model_catalog_json="+catalogPath)
	}
	output, err := runCodexReadOnly(executable, args...)
	if err != nil {
		return ModelCatalogProbe{}, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(output, &items); err != nil {
		return ModelCatalogProbe{}, fmt.Errorf("parse Codex input for %q: %w", model, err)
	}
	instructions, err := resolvedInstructionLength(exportDir)
	if err != nil {
		return ModelCatalogProbe{}, fmt.Errorf("read resolved configuration for %q: %w", model, err)
	}
	return ModelCatalogProbe{
		Model:        model,
		Instructions: instructions,
		Items:        len(items),
		MultiAgent:   strings.Count(string(output), "multi_agent"),
	}, nil
}

func resolvedInstructionLength(exportDir string) (int, error) {
	entries, err := os.ReadDir(exportDir)
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(exportDir, entry.Name()))
		if err != nil {
			return 0, err
		}
		match := resolvedInstructions.FindSubmatch(data)
		if match == nil {
			return 0, fmt.Errorf("exported configuration declares no instructions")
		}
		return len(match[2]), nil
	}
	return 0, fmt.Errorf("the client exported no configuration")
}
