package codex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/codex/modelcatalog"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/platform/process"

	"github.com/pelletier/go-toml/v2"
	"github.com/rogpeppe/go-internal/robustio"
)

const (
	// catalogStateProjected records that AIGW owns a model catalog for a target.
	// catalogStateStale records that AIGW owned one, can no longer prove it
	// describes the installed client, and has therefore withdrawn it.
	catalogStateProjected = "projected"
	catalogStateStale     = "stale"

	// codexCatalogTimeout bounds the read-only client invocations AIGW uses to
	// read the bundled catalog. A client that never answers must fail the
	// generation rather than hold the projection transaction open.
	codexCatalogTimeout = 30 * time.Second
	// The client emits complete model instructions: the measured 0.153.4
	// catalog is 517,840 bytes. This finite 8 MiB data budget leaves growth
	// headroom without widening diagnostic capture for other invocations.
	codexCatalogByteLimit = 8 << 20
)

// codexBundledCatalog reads the installed client's own bundled model catalog.
// It is a package seam so tests can supply a catalog without an installed
// client; production reads the client itself.
var codexBundledCatalog = ReadBundledCatalog

var removeCatalogProbe = robustio.RemoveAll

// codexCatalogPlan is the catalog decision for one target: the path the
// configuration should reference, the bytes AIGW should own, the client identity
// the bytes came from and the configured path of that program, and the
// ownership state recorded in the sidecar.
type codexCatalogPlan struct {
	path    string
	data    []byte
	client  ExecutableIdentity
	program string
	state   string
}

func codexCatalogPath(configPath string) string { return configPath + ".aigw-model-catalog.json" }

func rebaseCopiedCodexCatalog(source, target string, config, attribution platform.FileSnapshot) (platform.FileSnapshot, error) {
	if !config.Exists {
		if attribution.Exists {
			return config, fmt.Errorf("Codex client state exists without its configuration")
		}
		return config, nil
	}
	state := codexState{}
	if attribution.Exists {
		var err error
		state, err = codexStateForTarget(attribution)
		if err != nil {
			return config, err
		}
	}
	if err := validateCodexCatalog(source, string(config.Data), state); err != nil {
		return config, err
	}
	if state.CatalogHash == "" {
		return rebaseUserCatalogReference(source, config)
	}
	canonical, err := platform.CanonicalPath(target)
	if err != nil {
		return config, err
	}
	quoted, err := codexTOMLString(codexCatalogPath(canonical))
	if err != nil {
		return config, err
	}
	projected, err := setCodexSelection(string(config.Data), "model_catalog_json", "model_catalog_json = "+quoted+" # managed by AIGW")
	if err != nil {
		return config, err
	}
	config.Data = []byte(projected)
	return config, nil
}

// rebaseUserCatalogReference keeps a user-owned catalog reachable from a copy
// in another Codex home. Codex resolves a relative model_catalog_json from the
// directory of the configuration that names it, so the copy names the same file
// by its absolute path, joined to that directory. A
// reference that starts with ~ stays as it is: the copy keeps HOME, and whether
// Codex expands it is unproved.
func rebaseUserCatalogReference(source string, config platform.FileSnapshot) (platform.FileSnapshot, error) {
	line, err := codexSelectionLine(string(config.Data), "model_catalog_json")
	if err != nil || line == "" {
		return config, err
	}
	var selection map[string]any
	if err := toml.Unmarshal([]byte(line), &selection); err != nil {
		return config, fmt.Errorf("parse Codex config catalog reference: %w", err)
	}
	reference, _ := selection["model_catalog_json"].(string)
	if reference == "" || filepath.IsAbs(reference) || strings.HasPrefix(reference, "~") {
		return config, nil
	}
	quoted, err := codexTOMLString(filepath.Join(filepath.Dir(source), reference))
	if err != nil {
		return config, err
	}
	projected, err := setCodexSelection(string(config.Data), "model_catalog_json", "model_catalog_json = "+quoted)
	if err != nil {
		return config, err
	}
	config.Data = []byte(projected)
	return config, nil
}

func validateCodexCatalogPreflight(target codexReconciliationTarget, config, state platform.FileSnapshot) error {
	if !target.desired && !state.Exists {
		return nil
	}
	attribution := codexState{}
	if state.Exists {
		var err error
		attribution, err = codexStateForTarget(state)
		if err != nil {
			// Every plan of a recorded file passes here first.
			return adapter.UnreadableRecord(targetCodexStatePath(target.ref), err)
		}
	}
	return validateCodexCatalogReference(target.ref.Path, string(config.Data), attribution)
}

// recordedCatalog is the one reader of the model table that AIGW wrote for the
// Codex file at path: the bytes that state, AIGW's record of that file, holds
// the digest of, and nothing where the record holds no table. It refuses a
// table that is missing, that is not AIGW's regular, owner-only file, or whose
// bytes are not the recorded ones.
func recordedCatalog(path string, state codexState) ([]byte, error) {
	if state.CatalogHash == "" {
		return nil, nil
	}
	// The catalog is written beside the canonical configuration path, because that
	// is the identity the projection transaction resolves targets to. Validation
	// resolves the caller's path the same way, so a symlinked configuration
	// directory does not read as a mismatch.
	canonical, err := platform.CanonicalPath(path)
	if err != nil {
		return nil, err
	}
	catalogPath := codexCatalogPath(canonical)
	// The catalog's type and permissions are as much a part of what AIGW owns as
	// its bytes, so they are checked before the contents: a symlink or a widened
	// mode at the managed path is a drift worth reporting on its own.
	info, err := os.Lstat(catalogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Codex model catalog is missing")
		}
		return nil, fmt.Errorf("read Codex model catalog: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("Codex model catalog is not a regular file: %s", info.Mode())
	}
	if info.Mode().Perm() != 0o600 {
		return nil, fmt.Errorf("Codex model catalog is not owner-only: %s", info.Mode().Perm())
	}
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Codex model catalog is missing")
		}
		return nil, fmt.Errorf("read Codex model catalog: %w", err)
	}
	if platform.SHA256(data) != state.CatalogHash {
		return nil, newConflictError("Codex config conflict: AIGW-managed model catalog changed; refusing to overwrite user edits", nil)
	}
	return data, nil
}

// noTableReason names why AIGW writes no model table for a model that runs as
// runtime, given the model_catalog_json line of the configuration that the
// projection starts from: the model runs under a model provider of Codex's own,
// or that line names a table that AIGW did not write. It is empty where AIGW
// writes a table. The projection and the window check both ask it, so the two
// cannot disagree; each reads its own text for the line and decides what a line
// that it cannot read means.
func noTableReason(runtime domain.Runtime, ownLine string) string {
	if provider := codexRuntimeProvider(runtime); provider != modelProviderAIGW {
		return "the model runs under Codex's model provider " + provider + ", for which AIGW writes no model table"
	}
	if ownLine != "" {
		return "this configuration names its own model table in model_catalog_json, which AIGW leaves alone"
	}
	return ""
}

// codexCatalogProjection decides what AIGW owns for one target without writing
// anything. For the selected model under AIGW's custom provider it pins the
// installed client's complete table, even for a known base model: otherwise
// Codex may try to decode the provider's standard /models response as its
// private metadata format. The entry that describes the model, its own or an
// alias of its family's, carries the model's declared context window; a model
// that the table describes by neither keeps the client's fallback rather than
// a guessed entry. A model under another provider gets no table of AIGW's.
func codexCatalogProjection(target TargetRef, selected domain.Runtime, base string, state codexState, before platform.FileSnapshot) codexCatalogPlan {
	// A user-authored model_catalog_json is the user's own client policy. AIGW
	// replaces the bundled table wholesale, so adopting that key here would
	// silently drop models the user added.
	selection, selectionErr := codexSelectionLine(base, "model_catalog_json")
	model := selected.UpstreamModel
	if model == "" || selectionErr != nil || noTableReason(selected, selection) != "" {
		return codexCatalogPlan{}
	}
	live, bundled, err := codexBundledCatalog(target.Executable)
	document, parseErr := modelcatalog.Parse(bundled)
	if err == nil && parseErr == nil {
		data, _ := document.Project(model, selected.FamilyID, selected.ModelName(), selected.ContextWindow)
		if data == nil {
			// Preserve Codex's native table for a model that neither its own
			// entry nor its family's describes. Never invent metadata or leave
			// Codex to query provider-specific /models.
			data = bundled
		}
		return codexCatalogPlan{path: codexCatalogPath(target.Path), data: data, client: live, program: target.Executable, state: catalogStateProjected}
	}
	// Regeneration failed. A previous copy is reusable only while it still
	// describes the installed build: after an upgrade the old snapshot would
	// override the client's newer bundled table, which is a worse failure than
	// the fallback it was meant to prevent.
	recorded := ExecutableIdentity{Version: state.CatalogClientVersion, SHA256: state.CatalogClientSHA256}
	if state.CatalogHash != "" && live.same(recorded) && before.Exists && platform.SHA256(before.Data) == state.CatalogHash {
		data := before.Data
		// The copy's own entry for the model still takes the declared window. No
		// family is given, so a copy without that entry gets no alias from a
		// table that the client did not just print, and stays as it is. An
		// unchanged window gives the bytes that the same projection wrote.
		if reused, err := modelcatalog.Parse(data); err == nil {
			if windowed, _ := reused.Project(model, "", "", selected.ContextWindow); windowed != nil {
				data = windowed
			}
		}
		return codexCatalogPlan{path: codexCatalogPath(target.Path), data: data, client: recorded, program: target.Executable, state: catalogStateProjected}
	}
	if state.CatalogHash == "" && state.CatalogState == "" {
		// AIGW never owned a catalog here, so nothing was lost and there is
		// nothing to report; the client keeps the behavior it already had.
		return codexCatalogPlan{}
	}
	return codexCatalogPlan{client: recorded, program: state.CatalogClientPath, state: catalogStateStale}
}

// codexCatalogDesiredSnapshot converts a catalog decision into the desired file
// state. AIGW removes a catalog only while the bytes on disk are still the ones
// it recorded writing, so a file it does not own survives untouched. For the
// same reason a file at the managed path that AIGW cannot prove is its own is
// not written over either: an unrecognized file there is a conflict to report,
// not a file to adopt.
func codexCatalogDesiredSnapshot(plan codexCatalogPlan, before platform.FileSnapshot, ownedHash string) (platform.FileSnapshot, error) {
	owned := before.Exists && ownedHash != "" && platform.SHA256(before.Data) == ownedHash
	if plan.data != nil {
		if before.Exists && !owned {
			return platform.FileSnapshot{}, newConflictError(fmt.Sprintf(
				"Codex config conflict: %s exists and is not the model catalog AIGW recorded writing; refusing to overwrite or re-permission it",
				plan.path,
			), nil)
		}
		// The catalog carries the resolved account's model metadata, so its mode
		// is part of what AIGW owns rather than a user preference: a mode that
		// drifted wider converges back instead of being carried forward.
		return platform.NewFileSnapshot(plan.data, 0o600), nil
	}
	if owned {
		return platform.FileSnapshot{}, nil
	}
	return before, nil
}

func applyCodexCatalogState(state *codexState, plan codexCatalogPlan) {
	state.CatalogState = plan.state
	state.CatalogClientPath = plan.program
	state.CatalogClientVersion = plan.client.Version
	state.CatalogClientSHA256 = plan.client.SHA256
	state.CatalogHash = ""
	if plan.data != nil {
		state.CatalogHash = platform.SHA256(plan.data)
	}
}

// ReadBundledCatalog reads the installed client's identity and its bundled
// catalog. The identity is returned even when the catalog read fails, because
// deciding whether a previous copy may be reused requires knowing which build is
// installed now.
func ReadBundledCatalog(executable string) (client ExecutableIdentity, data []byte, result error) {
	if strings.TrimSpace(executable) == "" {
		return ExecutableIdentity{}, nil, fmt.Errorf("Codex executable is not configured")
	}
	result = withCatalogProbe(func(ctx context.Context, home, temporary string) error {
		var err error
		client, err = IdentifyExecutable(ctx, process.Runner{}, executable, home, temporary)
		if err != nil {
			return err
		}
		data, err = runCodexReadOnly(ctx, process.Runner{StdoutLimit: codexCatalogByteLimit}, executable, home, temporary, "debug", "models", "--bundled")
		return err
	})
	return client, data, result
}

// ReadEffectiveCatalog asks the installed client to render its selected catalog
// in an isolated home. It neither reads user configuration nor sends inference.
func ReadEffectiveCatalog(executable, catalogPath string) (data []byte, result error) {
	args := []string{"debug", "models"}
	if catalogPath != "" {
		quoted, err := codexTOMLString(catalogPath)
		if err != nil {
			return nil, err
		}
		args = append(args, "-c", "model_catalog_json="+quoted)
	}
	result = withCatalogProbe(func(ctx context.Context, home, temporary string) error {
		var err error
		data, err = runCodexReadOnly(ctx, process.Runner{StdoutLimit: codexCatalogByteLimit}, executable, home, temporary, args...)
		return err
	})
	return data, result
}

func withCatalogProbe(probe func(context.Context, string, string) error) (result error) {
	home, err := os.MkdirTemp("", "aigw-codex-catalog-")
	if err != nil {
		return fmt.Errorf("create Codex probe home: %w", err)
	}
	defer func() {
		if err := removeCatalogProbe(home); err != nil {
			result = errors.Join(result, fmt.Errorf("remove Codex probe home %s: %w", home, err))
		}
	}()
	// Codex refuses aliases when its home lies below the child's temporary root.
	// Both paths stay inside one owned workspace, with temporary files below home.
	temporary := filepath.Join(home, "tmp")
	if err := os.Mkdir(temporary, 0o700); err != nil {
		return fmt.Errorf("create Codex probe temporary root: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), codexCatalogTimeout)
	defer cancel()
	return probe(ctx, home, temporary)
}
