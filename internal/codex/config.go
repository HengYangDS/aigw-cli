// Package codex owns Codex Home configuration projection, inspection,
// reconciliation, and native authentication plans. It never owns conversations
// or Codex Desktop-only settings.
package codex

import (
	"aigw-cli/internal/process"
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

var modelProviderLine = regexp.MustCompile(`(?m)^[ \t]*model_provider[ \t]*=.*$`)
var modelLine = regexp.MustCompile(`(?m)^[ \t]*model[ \t]*=.*$`)
var modelCatalogLine = regexp.MustCompile(`(?m)^[ \t]*model_catalog_json[ \t]*=.*$`)

type codexState struct {
	OriginalProvider       string          `json:"original_provider,omitempty"`
	OriginalModel          string          `json:"original_model,omitempty"`
	ManagedBlockHash       string          `json:"managed_block_hash"`
	OriginalScheduler      map[string]*int `json:"original_scheduler,omitempty"`
	ProjectedSchedulerHash string          `json:"projected_scheduler_hash,omitempty"`
	CatalogState           string          `json:"catalog_state,omitempty"`
	CatalogHash            string          `json:"catalog_hash,omitempty"`
	CatalogClientVersion   string          `json:"catalog_client_version,omitempty"`
	CatalogClientSHA256    string          `json:"catalog_client_sha256,omitempty"`
	ProjectedProvider      string          `json:"projected_provider,omitempty"`
	ProjectionMode         string          `json:"projection_mode,omitempty"`
	WriterID               string          `json:"writer_id,omitempty"`
	TransactionID          string          `json:"transaction_id,omitempty"`
}

// ProjectionPlan is a non-secret, read-only rendering of one target's
// proposed configuration projection. It never includes configuration content,
// credentials, or state bodies.
type ProjectionPlan struct {
	Target string `json:"target"`
	Action string `json:"action"`
}

func LoginPlan(executable, codexHome, token string) (process.Plan, error) {
	if executable == "" {
		return process.Plan{}, fmt.Errorf("Codex executable is not configured")
	}
	if token == "" {
		return process.Plan{}, fmt.Errorf("Codex token is empty")
	}
	env := os.Environ()
	if codexHome != "" {
		env = removeEnvironment(env, "CODEX_HOME")
		env = append(env, "CODEX_HOME="+codexHome)
	}
	return process.Plan{
		Executable: executable,
		Args:       []string{"login", "--with-api-key"},
		Env:        env,
		Stdin:      token + "\n",
	}, nil
}

func SyncConfig(path string, runtime configuration.Runtime) error {
	return SyncConfigs([]string{path}, runtime)
}

// PlanConfigs performs every projection read and conflict check without
// writing. It is the dry-run boundary used by the CLI and callers that need
// evidence before mutation.
func PlanConfigs(paths []string, runtime configuration.Runtime) ([]ProjectionPlan, error) {
	return PlanReconciliation(nil, codexHomeTargets(paths), runtime)
}

// SyncConfigs is an all-target transaction. It prepares every target
// before the first write; a later conflict therefore cannot leave an earlier
// profile half-synchronized. If any atomic write fails, every configuration and
// sidecar returns to its byte-exact pre-state, including an absent sidecar.
func SyncConfigs(paths []string, runtime configuration.Runtime) error {
	_, err := ReconcileConfigs(nil, codexHomeTargets(paths), runtime)
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
	if codexRuntimeProvider(runtime) != configuration.ModelProviderAIGW && runtime.CredentialCommand == "" {
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
	if isManagedSelection(modelProviderLine.FindString(text), "model_provider", provider) {
		if model := runtime.Model; model != "" {
			if !isManagedSelection(modelLine.FindString(text), "model", strings.ReplaceAll(model, "\"", "'")) {
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
		if !codexSchedulerHashMatches(state.ProjectedSchedulerHash, text) {
			return fmt.Errorf("Codex config conflict: AIGW-managed scheduler keys changed; refusing to overwrite user edits")
		}
		return validateCodexCatalog(path, text, state)
	}
	return fmt.Errorf("Codex config provider selection does not match AIGW")
}

// validateCodexCatalog verifies the model catalog AIGW owns for one target. A
// withdrawn catalog is reported rather than passed over: routing still works, so
// nothing else would notice that the client has silently returned to fallback
// metadata for a provider-prefixed model.
func validateCodexCatalog(path, text string, state codexState) error {
	if state.CatalogState == catalogStateStale {
		return fmt.Errorf("Codex model catalog is stale: it was copied from Codex %q and no longer matches the installed client; run aigw sync", state.CatalogClientVersion)
	}
	line := modelCatalogLine.FindString(text)
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

func DisableConfig(path string) error {
	_, err := ReconcileConfigs(codexHomeTargets([]string{path}), nil, configuration.Runtime{})
	return err
}

func codexUserConfig(configSnapshot, stateSnapshot transaction.FileSnapshot, runtime configuration.Runtime, expectedBlock string) (string, codexState, error) {
	if stateSnapshot.Exists {
		var state codexState
		if err := json.Unmarshal(stateSnapshot.Data, &state); err != nil {
			return "", codexState{}, fmt.Errorf("parse Codex adapter state: %w", err)
		}
		currentText := string(configSnapshot.Data)
		base, err := removeCodexProjection(currentText, state)
		if err != nil {
			if repaired, ok := completeExactTruncatedCodexProjection(currentText, state, runtime, expectedBlock); ok {
				state.ManagedBlockHash = hashText(expectedBlock)
				base, err = removeCodexProjection(repaired, state)
			}
		}
		if err != nil {
			return "", codexState{}, err
		}
		base, err = restoreCodexScheduler(base, state.OriginalScheduler)
		if err != nil {
			return "", codexState{}, err
		}
		state.OriginalScheduler, err = backfillCodexScheduler(state.OriginalScheduler, base)
		if err != nil {
			return "", codexState{}, err
		}
		return base, state, nil
	}
	text := string(configSnapshot.Data)
	originalProvider := modelProviderLine.FindString(text)
	originalModel := modelLine.FindString(text)
	scheduler, err := captureCodexScheduler(text)
	if err != nil {
		return "", codexState{}, err
	}
	return text, codexState{OriginalProvider: originalProvider, OriginalModel: originalModel, OriginalScheduler: scheduler}, nil
}

// completeExactTruncatedCodexProjection admits only the known interrupted
// projection shape: the current runtime's complete owned block with its final
// ownership marker omitted. It returns an in-memory completion so the caller's
// normal atomic projection transaction remains the sole write path.
func completeExactTruncatedCodexProjection(current string, state codexState, runtime configuration.Runtime, expectedBlock string) (string, bool) {
	provider := codexStateProvider(state)
	if !isManagedSelection(modelProviderLine.FindString(current), "model_provider", provider) {
		return "", false
	}
	if runtime.Model != "" && !isManagedSelection(modelLine.FindString(current), "model", strings.ReplaceAll(runtime.Model, "\"", "'")) {
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
	if codexRuntimeProvider(runtime) != configuration.ModelProviderAIGW {
		if runtime.CredentialCommand == "" {
			return "", fmt.Errorf("profile %q native Codex provider requires a credential command", runtime.ProfileID)
		}
		if !filepath.IsAbs(runtime.CredentialCommand) {
			return "", fmt.Errorf("profile %q native Codex provider credential command must be absolute", runtime.ProfileID)
		}
	}
	return runtime.Endpoint, nil
}

func projectCodex(original, block, model, catalogPath string) (string, error) {
	return projectCodexForProvider(original, block, model, catalogPath, configuration.ModelProviderAIGW)
}

func projectCodexForProvider(original, block, model, catalogPath, provider string) (string, error) {
	base, err := projectCodexScheduler(original)
	if err != nil {
		return "", err
	}
	base = strings.TrimRight(base, "\r\n")
	selection := fmt.Sprintf(`model_provider = "%s" # managed by AIGW`, provider)
	if modelProviderLine.MatchString(base) {
		base = modelProviderLine.ReplaceAllString(base, selection)
	} else {
		base = selection + "\n" + base
	}
	if model != "" {
		selection := fmt.Sprintf("model = \"%s\" # managed by AIGW", strings.ReplaceAll(model, `"`, `'`))
		if modelLine.MatchString(base) {
			base = modelLine.ReplaceAllString(base, selection)
		} else {
			base = selection + "\n" + base
		}
	}
	// The catalog reference is projected only when AIGW owns a catalog for this
	// target. A stale reference is worse than none: the client refuses to start
	// when model_catalog_json names a file it cannot read.
	if catalogPath != "" {
		quoted, err := codexTOMLString(catalogPath)
		if err != nil {
			return "", err
		}
		selection := "model_catalog_json = " + quoted + " # managed by AIGW"
		if modelCatalogLine.MatchString(base) {
			base = modelCatalogLine.ReplaceAllString(base, selection)
		} else {
			base = selection + "\n" + base
		}
	} else {
		base = removeManagedCodexModelCatalog(base)
	}
	// A managed projection owns the provider block, not the incidental number
	// of blank lines before its ownership marker.  Keep the separator canonical
	// so a client formatter that folds adjacent blank lines cannot cause every
	// subsequent dry-run to report a needless update.
	return base + "\n" + codexBegin + "\n" + block, nil
}

func codexManagedBlock(runtime configuration.Runtime, endpoint string) string {
	provider := codexRuntimeProvider(runtime)
	if provider != configuration.ModelProviderAIGW {
		return codexProviderTable(provider) + "\n" +
			fmt.Sprintf("base_url = \"%s\"\n", endpoint) +
			"wire_api = \"responses\"\n\n" +
			"[model_providers." + provider + ".auth]\n" +
			fmt.Sprintf("command = %s\n", strconv.Quote(runtime.CredentialCommand)) +
			"args = [\"credential\", \"codex\"]\n" +
			codexEnd + "\n"
	}
	name := "AIGW: " + runtime.ProfileLabel
	name = strings.ReplaceAll(name, `"`, `'`)
	return "[model_providers.aigw]\n" +
		fmt.Sprintf("name = \"%s\"\n", name) +
		fmt.Sprintf("base_url = \"%s\"\n", endpoint) +
		"wire_api = \"responses\"\n" +
		"requires_openai_auth = true\n" +
		codexEnd + "\n"
}

func removeCodexProjection(current string, state codexState) (string, error) {
	provider := codexStateProvider(state)
	if !modelProviderLine.MatchString(current) || !isManagedSelection(modelProviderLine.FindString(current), "model_provider", provider) {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed model_provider selection changed; refusing to overwrite user edits")
	}
	block, err := codexManagedBlockForProviderIn(current, provider)
	if err != nil {
		return "", err
	}
	if !managedBlockHashMatches(state.ManagedBlockHash, block) {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed provider block changed; refusing to overwrite user edits")
	}
	if !codexSchedulerHashMatches(state.ProjectedSchedulerHash, current) {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed scheduler keys changed; refusing to overwrite user edits")
	}
	providerStart := strings.Index(current, codexProviderTable(provider))
	providerEnd := providerStart + len(block)
	base := strings.TrimRight(current[:providerStart]+current[providerEnd:], "\r\n")
	base = removeCodexBeginMarker(base)
	base = strings.TrimRight(base, "\r\n")
	if state.OriginalProvider != "" {
		base = modelProviderLine.ReplaceAllString(base, state.OriginalProvider)
	} else {
		base = modelProviderLine.ReplaceAllString(base, "")
		base = strings.TrimLeft(base, "\r\n")
	}
	if state.OriginalModel != "" {
		base = restoreModelSelection(base, state.OriginalModel)
	} else {
		base = removeManagedModelSelection(base)
	}
	// The catalog reference is AIGW's own line and its file is AIGW's own
	// artifact, so a restore removes the reference here and the transaction
	// removes the file. A user-authored model_catalog_json was never projected
	// and is therefore not matched.
	base = removeManagedCodexModelCatalog(base)
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
	return isManagedAssignment(line, key, `"`+value+`"`)
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
	marker := strings.Index(current, codexBegin)
	if marker < 0 {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed provider block is missing")
	}
	providerRel := strings.Index(current[marker:], codexProviderTable(provider))
	if providerRel < 0 {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed provider table is missing")
	}
	start := marker + providerRel
	endRel := strings.Index(current[start:], codexEnd)
	if endRel < 0 {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed provider block is incomplete")
	}
	end := start + endRel + len(codexEnd)
	if end < len(current) && current[end] == '\r' {
		end++
	}
	if end < len(current) && current[end] == '\n' {
		end++
	}
	return current[start:end], nil
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

func restoreModelSelection(base, originalModel string) string {
	if originalModel != "" {
		if modelLine.MatchString(base) {
			return modelLine.ReplaceAllString(base, originalModel)
		}
		return originalModel + "\n" + base
	}
	return removeManagedModelSelection(base)
}

func removeManagedModelSelection(base string) string {
	return removeManagedCodexLine(base, "model")
}

func removeManagedCodexModelCatalog(base string) string {
	return removeManagedCodexLine(base, "model_catalog_json")
}

// removeManagedCodexLine drops the top-level assignment AIGW owns for one key.
// The key is matched exactly, because model and model_catalog_json are separate
// settings with separate owners and a prefix match would let a restore of one
// silently discard the other.
func removeManagedCodexLine(base, key string) string {
	pattern := regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(key) + `[ \t]*=.*#[ \t]*managed by AIGW[ \t]*(?:\r?\n|$)`)
	trimmed := pattern.ReplaceAllString(base, "")
	// Removing the document's final line also removes the newline that separated
	// it, so a document that did not end in a newline still does not.
	if !strings.HasSuffix(base, "\n") {
		return strings.TrimSuffix(trimmed, "\n")
	}
	return trimmed
}

func codexStatePath(path string) string { return path + ".aigw-state.json" }

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func removeEnvironment(env []string, keys ...string) []string {
	remove := map[string]bool{}
	for _, key := range keys {
		remove[key] = true
	}
	out := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if remove[key] || strings.HasPrefix(key, "AIGW_TOKEN_") {
			continue
		}
		out = append(out, entry)
	}
	return out
}
