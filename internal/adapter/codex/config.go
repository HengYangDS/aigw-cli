// Package codex owns Codex Home configuration projection, inspection,
// and reconciliation. It never owns conversations
// or Codex Desktop-only settings.
package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"

	"github.com/pelletier/go-toml/v2"
)

const (
	codexSelection = `model_provider = "aigw" # managed by AIGW`
	codexBegin     = "# >>> AIGW managed provider >>>"
	codexEnd       = "# <<< AIGW managed provider <<<"
	// Codex owns scheduling; AIGW projects the bounded client policy while the
	// selected endpoint remains an ordinary provider concern. Codex reads
	// [agents].max_threads as the session concurrency field and treats
	// [agents].max_concurrent_threads_per_session as its retired alias, so the two
	// cannot share a table; the feature-gated table still uses the per-session
	// spelling. Do not replace either with endpoint-side session policy.
	codexSessionConcurrency = 16
	codexAgentDepth         = 1
)

type codexState struct {
	ConfigAbsentBeforeProjection bool            `json:"config_absent_before_projection,omitempty"`
	OriginalProvider             string          `json:"original_provider,omitempty"`
	OriginalModel                string          `json:"original_model,omitempty"`
	ManagedBlockHash             string          `json:"managed_block_hash"`
	OriginalScheduler            map[string]*int `json:"original_scheduler,omitempty"`
	ProjectedSchedulerHash       string          `json:"projected_scheduler_hash,omitempty"`
	CatalogState                 string          `json:"catalog_state,omitempty"`
	CatalogHash                  string          `json:"catalog_hash,omitempty"`
	CatalogClientVersion         string          `json:"catalog_client_version,omitempty"`
	CatalogClientSHA256          string          `json:"catalog_client_sha256,omitempty"`
	ProjectedProvider            string          `json:"projected_provider,omitempty"`
	ProjectedModel               string          `json:"projected_model,omitempty"`
	ProjectionMode               string          `json:"projection_mode,omitempty"`
	WriterID                     string          `json:"writer_id,omitempty"`
	TransactionID                string          `json:"transaction_id,omitempty"`
}

// ProjectionAction identifies one closed Codex projection transition.
type ProjectionAction string

const (
	// ProjectionActionInitialProject creates AIGW-owned state for a new target.
	ProjectionActionInitialProject ProjectionAction = "initial-project"
	// ProjectionActionUpdate changes an existing AIGW-owned projection.
	ProjectionActionUpdate ProjectionAction = "update"
	// ProjectionActionAlreadyConverged reports that the target needs no change.
	ProjectionActionAlreadyConverged ProjectionAction = "already-converged"
	// ProjectionActionAlreadyRestored reports that no AIGW-owned projection remains.
	ProjectionActionAlreadyRestored ProjectionAction = "already-restored"
	// ProjectionActionRestoreExternal restores the target's pre-AIGW state.
	ProjectionActionRestoreExternal ProjectionAction = "restore-external"
	// ProjectionActionAbsent reports a dropped target that no longer exists
	// and of which AIGW keeps no record, so there is nothing to withdraw.
	ProjectionActionAbsent ProjectionAction = "absent"
)

// ProjectionPlan is a non-secret, read-only rendering of one target's
// proposed configuration projection, with the identity of the surface that
// owns the target. It never includes configuration content, credentials, or
// state bodies.
type ProjectionPlan struct {
	Target    string           `json:"target"`
	Action    ProjectionAction `json:"action"`
	SurfaceID string           `json:"-"`
	// Effects lists every file that applying the plan may write or remove: the
	// configuration, its ownership state and its model catalog.
	Effects []string `json:"-"`
}

// ChangesState reports whether applying the plan changes an owned target.
func (plan ProjectionPlan) ChangesState() bool {
	return plan.Action == ProjectionActionInitialProject ||
		plan.Action == ProjectionActionUpdate ||
		plan.Action == ProjectionActionRestoreExternal
}

// ObservedCredentialCommand reads the projected command without treating it as
// trusted. The caller must validate the intact reader and the complete owned
// projection before executing it.
func ObservedCredentialCommand(path string, runtime domain.Runtime) (string, error) {
	if !runtime.RequiresProviderToken() {
		return "", fmt.Errorf("Codex model %q does not use a Token", runtime.Model)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read Codex config: %w", err)
	}
	projection, err := codexProviderForProviderIn(string(data), codexRuntimeProvider(runtime))
	if err != nil {
		return "", err
	}
	if projection.Auth == nil {
		return "", fmt.Errorf("Codex model %q has no credential command in its client files", runtime.Model)
	}
	return projection.Auth.Command, nil
}

// ValidateConfig verifies that a Codex target still matches the resolved
// AIGW Route. It never changes the target; callers can safely use it for
// diagnostics before offering an explicit sync.
func ValidateConfig(path string, runtime domain.Runtime) error {
	return validateConfig(path, runtime, runtime.UpstreamModel)
}

// validateConfig verifies a Codex target against runtime, with shown as the
// root model that the file is to hold: runtime's own model, or Codex's own
// choice when the rest of the file is checked against the recorded Route.
func validateConfig(path string, runtime domain.Runtime, shown string) error {
	if runtime.RequiresProviderToken() && runtime.CredentialCommand == "" {
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve AIGW executable: %w", err)
		}
		runtime.CredentialCommand = executable
	}
	endpoint, err := codexEndpoint(runtime)
	if err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read Codex config: %w", err)
	}
	text := string(current)
	expectedBlock := codexManagedBlock(runtime, endpoint)
	recordPath := codexRecordPath(path)
	stateData, err := os.ReadFile(recordPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Synchronization refuses AIGW's table without its record too.
			if refusal := refuseUnrecordedProvider(platform.FileSnapshot{Exists: true, Data: current}, platform.FileSnapshot{}, codexRuntimeProvider(runtime), path, recordPath); refusal != nil {
				return refusal
			}
			return fmt.Errorf("Codex config AIGW state is missing")
		}
		return fmt.Errorf("read Codex adapter state: %w", err)
	}
	state, err := codexStateForTarget(platform.FileSnapshot{Exists: true, Data: stateData})
	if err != nil {
		return adapter.UnreadableRecord(recordPath, err)
	}
	// Recorded ownership is checked before the desired projection: an outside
	// edit is a conflict that synchronization refuses, while an owned stale
	// projection is drift that synchronization repairs.
	provider := codexStateProvider(state)
	providerLine, err := codexSelectionLine(text, "model_provider")
	if err != nil {
		return err
	}
	if !selectionHasValue(providerLine, "model_provider", provider) {
		return newConflictError("Codex config conflict: AIGW-managed model_provider selection changed; refusing to overwrite user edits", nil)
	}
	actualBlock, err := codexManagedBlockForProviderIn(text, provider)
	if err != nil {
		return err
	}
	if state.ManagedBlockHash != platform.SHA256([]byte(actualBlock)) {
		return newConflictError("Codex config conflict: AIGW-managed provider block changed; refusing to overwrite user edits", nil)
	}
	if err := validateCodexSchedulerOwnership(state, text); err != nil {
		return err
	}
	if model := shown; model != "" {
		modelLine, err := codexSelectionLine(text, "model")
		if err != nil {
			return err
		}
		if !selectionHasValue(modelLine, "model", model) {
			return fmt.Errorf("Codex config model selection does not match model %q", runtime.Model)
		}
	}
	if platform.SHA256([]byte(actualBlock)) != platform.SHA256([]byte(expectedBlock)) {
		return fmt.Errorf("Codex config provider block does not match model %q", runtime.Model)
	}
	if err := validateCodexScheduler(text); err != nil {
		return err
	}
	return validateCodexCatalog(path, text, state)
}

// validateCodexCatalog verifies the model catalog AIGW owns for one target. A
// withdrawn catalog is reported rather than passed over: routing still works, so
// nothing else would notice that the client has silently returned to fallback
// metadata for a provider-prefixed model.
func validateCodexCatalog(path, text string, state codexState) error {
	if state.CatalogState == catalogStateStale {
		return fmt.Errorf("Codex model catalog is stale: it was copied from Codex %q and no longer matches the installed client; run aigw sync", state.CatalogClientVersion)
	}
	if err := validateCodexCatalogReference(path, text, state); err != nil {
		return err
	}
	if state.CatalogHash == "" {
		return nil
	}
	// The catalog is written beside the canonical configuration path, because that
	// is the identity the projection transaction resolves targets to. Validation
	// resolves the caller's path the same way, so a symlinked configuration
	// directory does not read as a mismatch.
	canonical, err := canonicalCodexTargetPath(path)
	if err != nil {
		return err
	}
	catalogPath := codexCatalogPath(canonical)
	// The catalog's type and permissions are as much a part of what AIGW owns as
	// its bytes, so they are checked before the contents: a symlink or a widened
	// mode at the managed path is a drift worth reporting on its own.
	info, err := os.Lstat(catalogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Codex model catalog is missing")
		}
		return fmt.Errorf("read Codex model catalog: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Codex model catalog is not a regular file: %s", info.Mode())
	}
	if catalogModeIsEnforceable() && info.Mode().Perm() != 0o600 {
		return fmt.Errorf("Codex model catalog is not owner-only: %s", info.Mode().Perm())
	}
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Codex model catalog is missing")
		}
		return fmt.Errorf("read Codex model catalog: %w", err)
	}
	if platform.SHA256(data) != state.CatalogHash {
		return newConflictError("Codex config conflict: AIGW-managed model catalog changed; refusing to overwrite user edits", nil)
	}
	return nil
}

func validateCodexCatalogReference(path, text string, state codexState) error {
	line, err := codexSelectionLine(text, "model_catalog_json")
	if err != nil {
		return fmt.Errorf("parse Codex config catalog reference: %w", err)
	}
	if state.CatalogHash == "" {
		if strings.HasSuffix(strings.TrimSpace(line), "# managed by AIGW") {
			return newConflictError("Codex config references an AIGW-managed model catalog that AIGW does not own", nil)
		}
		return nil
	}
	canonical, err := canonicalCodexTargetPath(path)
	if err != nil {
		return err
	}
	catalogPath := codexCatalogPath(canonical)
	if !isManagedSelection(line, "model_catalog_json", catalogPath) {
		return newConflictError("Codex config model catalog selection does not match AIGW", nil)
	}
	return nil
}

// codexUserConfigTransition removes the recorded projection from a target
// before chosen, the root model about to be projected, replaces it.
func codexUserConfigTransition(configSnapshot, stateSnapshot platform.FileSnapshot, previous domain.Runtime, chosen string, replaceRootSelections bool) (string, codexState, error) {
	text := string(configSnapshot.Data)
	if !stateSnapshot.Exists {
		scheduler, err := captureCodexScheduler(text)
		if err != nil {
			return "", codexState{}, err
		}
		provider, err := codexSelectionLine(text, "model_provider")
		if err != nil {
			return "", codexState{}, err
		}
		model, err := codexSelectionLine(text, "model")
		if err != nil {
			return "", codexState{}, err
		}
		return text, codexState{
			ConfigAbsentBeforeProjection: !configSnapshot.Exists,
			OriginalProvider:             provider,
			OriginalModel:                model,
			OriginalScheduler:            scheduler,
		}, nil
	}
	state, err := codexStateForTarget(stateSnapshot)
	if err != nil {
		return "", codexState{}, err
	}
	base, err := removeCodexProjectionTransition(text, state, previous, chosen, replaceRootSelections)
	if err != nil {
		return "", codexState{}, err
	}
	return base, state, nil
}

func codexEndpoint(runtime domain.Runtime) (string, error) {
	if runtime.Endpoint == "" {
		return "", fmt.Errorf("Codex has no endpoint for model %q", runtime.Model)
	}
	if runtime.RequiresProviderToken() {
		if runtime.CredentialCommand == "" {
			return "", fmt.Errorf("model %q provider-token Codex provider requires a credential command", runtime.Model)
		}
		if !filepath.IsAbs(runtime.CredentialCommand) {
			return "", fmt.Errorf("model %q provider-token Codex provider credential command must be absolute", runtime.Model)
		}
	}
	return runtime.Endpoint, nil
}

func projectCodex(original, block, model, catalogPath, provider string) (string, error) {
	base, err := projectCodexScheduler(original)
	if err != nil {
		return "", err
	}
	base = strings.TrimRight(base, "\r\n")
	for _, selection := range []struct{ key, value string }{
		{"model_provider", provider}, {"model", model}, {"model_catalog_json", catalogPath},
	} {
		if selection.value == "" {
			continue
		}
		quoted, err := codexTOMLString(selection.value)
		if err != nil {
			return "", err
		}
		base, err = setCodexSelection(base, selection.key, selection.key+" = "+quoted+" # managed by AIGW")
		if err != nil {
			return "", err
		}
	}
	if catalogPath == "" {
		base, err = removeManagedCodexLine(base, "model_catalog_json")
		if err != nil {
			return "", err
		}
	}
	// A managed projection owns the provider block, not the incidental number
	// of blank lines before its ownership marker.  Keep the separator canonical
	// so a client formatter that folds adjacent blank lines cannot cause every
	// subsequent dry-run to report a needless update.
	return base + "\n" + codexBegin + "\n" + block, nil
}

func codexManagedBlock(runtime domain.Runtime, endpoint string) string {
	provider := codexRuntimeProvider(runtime)
	projection := codexProviderProjection{BaseURL: endpoint, WireAPI: "responses"}
	if runtime.RetryPolicy != nil {
		projection.RequestMaxRetries = runtime.RetryPolicy.RequestMaxRetries
		projection.StreamMaxRetries = runtime.RetryPolicy.StreamMaxRetries
	}
	if provider == modelProviderAIGW {
		name := "AIGW: " + runtime.ModelLabel
		projection.Name = &name
	}
	if runtime.RequiresProviderToken() {
		projection.Auth = &struct {
			Command string   `toml:"command"`
			Args    []string `toml:"args"`
		}{runtime.CredentialCommand, []string{"credential", "codex", runtime.CredentialProjectionFingerprint(ID)}}
	}
	return projection.render(provider)
}

// rootModelAdmitted reports whether Codex's root model line holds one of the
// models that a transition may replace: the recorded one, the one about to be
// projected, or the one that the transition starts from. An empty candidate
// admits nothing.
func rootModelAdmitted(modelLine string, candidates ...string) bool {
	return slices.ContainsFunc(candidates, func(model string) bool {
		return model != "" && selectionHasValue(modelLine, "model", model)
	})
}

// removeCodexProjectionTransition refuses a changed root model unless it already
// holds chosen, the model about to be projected, because writing that value
// cannot lose an edit, or the model of previous, the selection that the
// transition starts from: a configuration that names it has followed Codex's
// own choice, so the value is the selection and not an outside edit.
func removeCodexProjectionTransition(current string, state codexState, previous domain.Runtime, chosen string, replaceRootSelections bool) (string, error) {
	provider := codexStateProvider(state)
	block, err := codexManagedBlockForProviderIn(current, provider)
	if err != nil {
		return "", err
	}
	if state.ManagedBlockHash != platform.SHA256([]byte(block)) {
		return "", newConflictError("Codex config conflict: AIGW-managed provider block changed; refusing to overwrite user edits", nil)
	}
	providerLine, err := codexSelectionLine(current, "model_provider")
	if err != nil {
		return "", err
	}
	if !replaceRootSelections && !selectionHasValue(providerLine, "model_provider", provider) {
		return "", newConflictError("Codex config conflict: AIGW-managed model_provider selection changed; refusing to overwrite user edits", nil)
	}
	projectedModel := state.ProjectedModel
	if projectedModel == "" && previous.UpstreamModel != "" && platform.SHA256([]byte(codexManagedBlock(previous, previous.Endpoint))) == state.ManagedBlockHash {
		projectedModel = previous.UpstreamModel
	}
	modelLine, err := codexSelectionLine(current, "model")
	if err != nil {
		return "", err
	}
	if replaceRootSelections {
		// Explicit Route selection owns these root values. Provider tables,
		// scheduler state, and catalog bytes remain independently guarded.
	} else if projectedModel == "" {
		if model, _ := selectionValue(modelLine, "model"); !isManagedSelection(modelLine, "model", model) {
			return "", newConflictError("Codex config conflict: AIGW-managed model selection cannot be attributed; refusing to overwrite user edits", nil)
		}
	} else if !rootModelAdmitted(modelLine, projectedModel, chosen, previous.UpstreamModel) {
		return "", newConflictError("Codex config conflict: AIGW-managed model selection changed; refusing to overwrite user edits", nil)
	}
	if err := validateCodexSchedulerOwnership(state, current); err != nil {
		return "", err
	}
	ranges, err := codexProviderRanges(current, provider)
	if err != nil {
		return "", err
	}
	base := current
	for _, span := range slices.Backward(ranges) {
		base = base[:span[0]] + base[span[1]:]
	}
	base = removeCodexProviderMarkers(base)
	base = strings.TrimRight(base, "\r\n")
	base, err = setCodexSelection(base, "model_provider", state.OriginalProvider)
	if err != nil {
		return "", err
	}
	base, err = restoreModelSelection(base, state.OriginalModel)
	if err != nil {
		return "", err
	}
	// The catalog reference is AIGW's own line and its file is AIGW's own
	// artifact, so a restore removes the reference here and the transaction
	// removes the file. A user-authored model_catalog_json was never projected
	// and is therefore not matched.
	base, err = removeManagedCodexLine(base, "model_catalog_json")
	if err != nil {
		return "", err
	}
	base, err = restoreCodexScheduler(base+"\n", state.OriginalScheduler)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(current, "\n") {
		base = strings.Trim(base, "\n") + "\n"
	} else {
		base = strings.Trim(base, "\n")
	}
	return base, nil
}

// isManagedSelection compares the parsed TOML value while requiring the owned
// comment. Equivalent string spelling is not an ownership conflict.
func isManagedSelection(line, key, value string) bool {
	return selectionHasValue(line, key, value) && strings.HasSuffix(strings.TrimSpace(line), "# managed by AIGW")
}

// selectionValue returns the string that line assigns to key, and whether
// line is such an assignment. The line is read as TOML, the format of Codex's
// file, so every spelling of a string names the same value. It is the one
// reader of a root selection's value: a second reader that decoded the value
// as JSON read a literal string as no value, and a model chosen in Codex in
// that spelling as no choice.
func selectionValue(line, key string) (string, bool) {
	var selection map[string]any
	if err := toml.Unmarshal([]byte(line), &selection); err != nil || len(selection) != 1 {
		return "", false
	}
	selected, assigned := selection[key].(string)
	return selected, assigned
}

func selectionHasValue(line, key, value string) bool {
	selected, assigned := selectionValue(line, key)
	return assigned && selected == value
}

func codexRuntimeProvider(runtime domain.Runtime) string {
	if runtime.ModelProvider == "" {
		return modelProviderAIGW
	}
	return runtime.ModelProvider
}

func codexStateProvider(state codexState) string {
	if state.ProjectedProvider == "" {
		return modelProviderAIGW
	}
	return state.ProjectedProvider
}

func codexProviderTable(provider string) string {
	return "[model_providers." + provider + "]"
}

func restoreModelSelection(base, originalModel string) (string, error) {
	return setCodexSelection(base, "model", originalModel)
}

// removeManagedCodexLine drops the top-level assignment AIGW owns for one key.
// The key is matched exactly, because model and model_catalog_json are separate
// settings with separate owners and a prefix match would let a restore of one
// silently discard the other.
func removeManagedCodexLine(base, key string) (string, error) {
	line, err := codexSelectionLine(base, key)
	if err != nil {
		return "", err
	}
	if !regexp.MustCompile(`#[ \t]*managed by AIGW[ \t]*$`).MatchString(line) {
		return base, nil
	}
	return setCodexSelection(base, key, "")
}

func codexStatePath(path string) string { return path + adapter.StateSuffix }
