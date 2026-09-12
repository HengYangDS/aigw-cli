// Package codex owns Codex Home configuration projection, inspection,
// and reconciliation. It never owns conversations
// or Codex Desktop-only settings.
package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
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
	ProjectionMode               string          `json:"projection_mode,omitempty"`
	WriterID                     string          `json:"writer_id,omitempty"`
	TransactionID                string          `json:"transaction_id,omitempty"`
}

// ProjectionPlan is a non-secret, read-only rendering of one target's
// proposed configuration projection. It never includes configuration content,
// credentials, or state bodies.
type ProjectionPlan struct {
	Target string `json:"target"`
	Action string `json:"action"`
}

// SyncConfig reconciles one Codex configuration target to the resolved runtime.
func SyncConfig(path string, runtime configuration.Runtime) error {
	_, err := ReconcileConfigs(nil, codexHomeTargets([]string{path}), runtime)
	return err
}

func isExactTruncatedCodexProjection(current string, stateData []byte, runtime configuration.Runtime, block string) bool {
	var state codexState
	if len(stateData) == 0 || json.Unmarshal(stateData, &state) != nil {
		return false
	}
	_, ok := completeExactTruncatedCodexProjection(current, state, runtime, block)
	return ok
}

// ValidateConfig verifies that a Codex target still matches the resolved
// AIGW profile. It never changes the target; callers can safely use it for
// diagnostics before offering an explicit sync.
func ValidateConfig(path string, runtime configuration.Runtime) error {
	if runtime.RequiresAccountToken() && runtime.CredentialCommand == "" {
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
	stateData, err := os.ReadFile(codexStatePath(path))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Codex config AIGW state is missing")
		}
		return fmt.Errorf("read Codex adapter state: %w", err)
	}
	var state codexState
	if err := json.Unmarshal(stateData, &state); err != nil {
		return fmt.Errorf("parse Codex adapter state: %w", err)
	}
	if err := validateCodexStateAttribution(state); err != nil {
		return err
	}
	provider := codexStateProvider(state)
	providerLine, err := codexSelectionLine(text, "model_provider")
	if err != nil {
		return err
	}
	if !isManagedSelection(providerLine, "model_provider", provider) {
		return fmt.Errorf("Codex config provider selection does not match AIGW")
	}
	if model := runtime.Model; model != "" {
		modelLine, err := codexSelectionLine(text, "model")
		if err != nil {
			return err
		}
		if !isManagedSelection(modelLine, "model", model) {
			return fmt.Errorf("Codex config model selection does not match profile %q", runtime.ProfileID)
		}
	}
	actualBlock, err := codexManagedBlockForProviderIn(text, provider)
	if err != nil {
		return err
	}
	if hashText(actualBlock) != hashText(expectedBlock) {
		return fmt.Errorf("Codex config provider block does not match profile %q", runtime.ProfileID)
	}
	if !managedBlockHashMatches(state.ManagedBlockHash, actualBlock) {
		return fmt.Errorf("Codex config AIGW state does not match profile %q", runtime.ProfileID)
	}
	if err := validateCodexScheduler(text); err != nil {
		return err
	}
	if err := validateCodexSchedulerOwnership(state, text); err != nil {
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
	line, err := codexSelectionLine(text, "model_catalog_json")
	if err != nil {
		return err
	}
	if state.CatalogHash == "" {
		if line != "" && strings.Contains(line, "# managed by AIGW") {
			return fmt.Errorf("Codex config references an AIGW-managed model catalog that AIGW does not own")
		}
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
	quoted, err := codexTOMLString(catalogPath)
	if err != nil {
		return err
	}
	if !isManagedAssignment(line, "model_catalog_json", quoted) {
		return fmt.Errorf("Codex config model catalog selection does not match AIGW")
	}
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
	if hashBytes(data) != state.CatalogHash {
		return fmt.Errorf("Codex config conflict: AIGW-managed model catalog changed; refusing to overwrite user edits")
	}
	return nil
}

// DisableConfig removes AIGW's owned projection from one Codex configuration target.
func DisableConfig(path string) error {
	_, err := ReconcileConfigs(codexHomeTargets([]string{path}), nil, configuration.Runtime{})
	return err
}

func codexUserConfig(configSnapshot, stateSnapshot transaction.FileSnapshot, runtime configuration.Runtime, expectedBlock string) (string, codexState, error) {
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
	base, err := removeCodexProjection(text, state)
	if err != nil {
		if repaired, ok := completeExactTruncatedCodexProjection(text, state, runtime, expectedBlock); ok {
			state.ManagedBlockHash = hashText(expectedBlock)
			base, err = removeCodexProjection(repaired, state)
		}
	}
	if err != nil {
		return "", codexState{}, err
	}
	return base, state, nil
}

// completeExactTruncatedCodexProjection admits only the known interrupted
// projection shape: the current runtime's complete owned block with its final
// ownership marker omitted. It returns an in-memory completion so the caller's
// normal atomic projection transaction remains the sole write path.
func completeExactTruncatedCodexProjection(current string, state codexState, runtime configuration.Runtime, expectedBlock string) (string, bool) {
	provider := codexStateProvider(state)
	providerLine, err := codexSelectionLine(current, "model_provider")
	if err != nil || !isManagedSelection(providerLine, "model_provider", provider) {
		return "", false
	}
	modelLine, err := codexSelectionLine(current, "model")
	if err != nil || runtime.Model != "" && !isManagedSelection(modelLine, "model", runtime.Model) {
		return "", false
	}
	marker := strings.Index(current, codexBegin)
	if marker < 0 {
		return "", false
	}
	providerRel := strings.Index(current[marker:], codexProviderTable(provider))
	if providerRel < 0 {
		return "", false
	}
	start := marker + providerRel
	if strings.Contains(current[start:], codexEnd) {
		return "", false
	}
	truncated := strings.TrimSuffix(expectedBlock, codexEnd+"\n")
	remaining := current[start:]
	if !strings.HasPrefix(remaining, truncated) || state.ManagedBlockHash == "" {
		return "", false
	}
	tail := remaining[len(truncated):]
	if nextTable := regexp.MustCompile(`(?m)^\[[^\r\n]+\]`).FindStringIndex(tail); nextTable != nil {
		if strings.TrimSpace(tail[:nextTable[0]]) != "" {
			return "", false
		}
		return current[:start] + expectedBlock + tail[nextTable[0]:], true
	}
	if strings.TrimSpace(tail) != "" {
		return "", false
	}
	return current[:start] + expectedBlock, true
}

func codexEndpoint(runtime configuration.Runtime) (string, error) {
	if runtime.Endpoint == "" {
		return "", fmt.Errorf("profile %q has no Codex endpoint", runtime.ProfileID)
	}
	if runtime.RequiresAccountToken() {
		if runtime.CredentialCommand == "" {
			return "", fmt.Errorf("profile %q account-token Codex provider requires a credential command", runtime.ProfileID)
		}
		if !filepath.IsAbs(runtime.CredentialCommand) {
			return "", fmt.Errorf("profile %q account-token Codex provider credential command must be absolute", runtime.ProfileID)
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

func codexManagedBlock(runtime configuration.Runtime, endpoint string) string {
	provider := codexRuntimeProvider(runtime)
	block := codexProviderTable(provider) + "\n"
	if provider == configuration.ModelProviderAIGW {
		block += fmt.Sprintf("name = %s\n", strconv.Quote("AIGW: "+runtime.ProfileLabel))
	}
	block += fmt.Sprintf("base_url = %s\n", strconv.Quote(endpoint)) + "wire_api = \"responses\"\n"
	if runtime.RequiresAccountToken() {
		block += "\n[model_providers." + provider + ".auth]\n" +
			fmt.Sprintf("command = %s\n", strconv.Quote(runtime.CredentialCommand)) +
			fmt.Sprintf("args = [\"credential\", \"codex\", %s]\n", strconv.Quote(runtime.CredentialProjectionFingerprint(configuration.ClientCodex)))
	}
	return block + codexEnd + "\n"
}

func removeCodexProjection(current string, state codexState) (string, error) {
	provider := codexStateProvider(state)
	providerLine, err := codexSelectionLine(current, "model_provider")
	if err != nil {
		return "", err
	}
	if !isManagedSelection(providerLine, "model_provider", provider) {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed model_provider selection changed; refusing to overwrite user edits")
	}
	providerStart, providerEnd, err := codexManagedBlockBoundsForProviderIn(current, provider)
	if err != nil {
		return "", err
	}
	block := current[providerStart:providerEnd]
	if !managedBlockHashMatches(state.ManagedBlockHash, block) {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed provider block changed; refusing to overwrite user edits")
	}
	if err := validateCodexSchedulerOwnership(state, current); err != nil {
		return "", err
	}
	base := strings.TrimRight(current[:providerStart]+current[providerEnd:], "\r\n")
	base = removeCodexBeginMarker(base)
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

// isManagedSelection accepts harmless formatter changes such as the padded
// top-level assignments written by the client. Values and the ownership marker
// must still match exactly, so a semantic edit remains a conflict.
func isManagedSelection(line, key, value string) bool {
	encoded, err := codexTOMLString(value)
	return err == nil && isManagedAssignment(line, key, encoded)
}

// isManagedAssignment is the same check for a value that is already rendered as
// a TOML string, which a path must be: it may contain characters that require
// escaping and so cannot be compared as a bare literal.
func isManagedAssignment(line, key, encoded string) bool {
	pattern := `^[ \t]*` + regexp.QuoteMeta(key) + `[ \t]*=[ \t]*` + regexp.QuoteMeta(encoded) + `[ \t]*#[ \t]*managed by AIGW[ \t]*$`
	return regexp.MustCompile(pattern).MatchString(line)
}

func codexManagedBlockIn(current string) (string, error) {
	return codexManagedBlockForProviderIn(current, configuration.ModelProviderAIGW)
}

func codexManagedBlockForProviderIn(current, provider string) (string, error) {
	start, end, err := codexManagedBlockBoundsForProviderIn(current, provider)
	if err != nil {
		return "", err
	}
	return current[start:end], nil
}

func codexManagedBlockBoundsForProviderIn(current, provider string) (int, int, error) {
	marker := strings.Index(current, codexBegin)
	if marker < 0 {
		return 0, 0, fmt.Errorf("Codex config conflict: AIGW-managed provider block is missing")
	}
	providerRel := strings.Index(current[marker:], codexProviderTable(provider))
	if providerRel < 0 {
		return 0, 0, fmt.Errorf("Codex config conflict: AIGW-managed provider table is missing")
	}
	start := marker + providerRel
	endRel := strings.Index(current[start:], codexEnd)
	if endRel < 0 {
		return 0, 0, fmt.Errorf("Codex config conflict: AIGW-managed provider block is incomplete")
	}
	end := start + endRel + len(codexEnd)
	if end < len(current) && current[end] == '\r' {
		end++
	}
	if end < len(current) && current[end] == '\n' {
		end++
	}
	return start, end, nil
}

func codexRuntimeProvider(runtime configuration.Runtime) string {
	if runtime.ModelProvider == "" {
		return configuration.ModelProviderAIGW
	}
	return runtime.ModelProvider
}

func codexStateProvider(state codexState) string {
	if state.ProjectedProvider == "" {
		return configuration.ModelProviderAIGW
	}
	return state.ProjectedProvider
}

func codexProviderTable(provider string) string {
	return "[model_providers." + provider + "]"
}

func managedBlockHashMatches(stateHash, block string) bool {
	return stateHash == hashText(block)
}

func removeCodexBeginMarker(text string) string {
	for _, marker := range []string{codexBegin + "\r\n", codexBegin + "\n", codexBegin} {
		if strings.Contains(text, marker) {
			return strings.Replace(text, marker, "", 1)
		}
	}
	return text
}

func restoreModelSelection(base, originalModel string) (string, error) {
	if originalModel != "" {
		return setCodexSelection(base, "model", originalModel)
	}
	return removeManagedCodexLine(base, "model")
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

func codexStatePath(path string) string { return path + ".aigw-state.json" }

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
