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
	"regexp"
	"strings"

	configuration "aigw-cli/internal/configuration"
)

const (
	codexSelection = `model_provider = "aigw" # managed by AIGW`
	codexBegin     = "# >>> AIGW managed provider >>>"
	codexEnd       = "# <<< AIGW managed provider <<<"
	// Codex owns scheduling; AIGW projects the bounded client policy while the
	// selected endpoint remains an ordinary provider concern. The current Codex
	// schema calls this a per-session limit; do not replace it with the retired
	// max_threads spelling or a proxy-side session limiter.
	codexSessionConcurrency = 16
	codexAgentDepth         = 1
)

var modelProviderLine = regexp.MustCompile(`(?m)^[ \t]*model_provider[ \t]*=.*$`)
var modelLine = regexp.MustCompile(`(?m)^[ \t]*model[ \t]*=.*$`)

type codexState struct {
	OriginalProvider       string          `json:"original_provider,omitempty"`
	OriginalModel          string          `json:"original_model,omitempty"`
	ManagedBlockHash       string          `json:"managed_block_hash"`
	OriginalScheduler      map[string]*int `json:"original_scheduler,omitempty"`
	ProjectedSchedulerHash string          `json:"projected_scheduler_hash,omitempty"`
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
	if err := validateCodexStateAttribution(state, ProjectionFullSelection); err != nil {
		return err
	}
	if isManagedSelection(modelProviderLine.FindString(text), "model_provider", "aigw") {
		if model := runtime.Model; model != "" {
			if !isManagedSelection(modelLine.FindString(text), "model", strings.ReplaceAll(model, "\"", "'")) {
				return fmt.Errorf("Codex config model selection does not match profile %q", runtime.ProfileID)
			}
		}
		actualBlock, err := codexManagedBlockIn(text)
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
		if state.ProjectedSchedulerHash != "" && state.ProjectedSchedulerHash != codexSchedulerHash(text) {
			return fmt.Errorf("Codex config conflict: AIGW-managed scheduler keys changed; refusing to overwrite user edits")
		}
		return nil
	}
	return fmt.Errorf("Codex config provider selection does not match AIGW")
}

func DisableConfig(path string) error {
	_, err := ReconcileConfigs(codexHomeTargets([]string{path}), nil, configuration.Runtime{})
	return err
}

func codexUserConfigAt(path, statePath string, runtime configuration.Runtime, expectedBlock string) (string, codexState, error) {
	stateData, err := os.ReadFile(statePath)
	if err == nil {
		var state codexState
		if err := json.Unmarshal(stateData, &state); err != nil {
			return "", codexState{}, fmt.Errorf("parse Codex adapter state: %w", err)
		}
		current, err := os.ReadFile(path)
		if err != nil {
			return "", codexState{}, fmt.Errorf("read Codex config: %w", err)
		}
		currentText := string(current)
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
		return base, state, nil
	}
	if !os.IsNotExist(err) {
		return "", codexState{}, fmt.Errorf("read Codex adapter state: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", codexState{}, fmt.Errorf("read Codex config: %w", err)
	}
	originalProvider := modelProviderLine.FindString(string(data))
	originalModel := modelLine.FindString(string(data))
	scheduler, err := captureCodexScheduler(string(data))
	if err != nil {
		return "", codexState{}, err
	}
	return string(data), codexState{OriginalProvider: originalProvider, OriginalModel: originalModel, OriginalScheduler: scheduler}, nil
}

// completeExactTruncatedCodexProjection admits only the known interrupted
// projection shape: the current runtime's complete owned block with its final
// ownership marker omitted. It returns an in-memory completion so the caller's
// normal atomic projection transaction remains the sole write path.
func completeExactTruncatedCodexProjection(current string, state codexState, runtime configuration.Runtime, expectedBlock string) (string, bool) {
	if !isManagedSelection(modelProviderLine.FindString(current), "model_provider", "aigw") {
		return "", false
	}
	if runtime.Model != "" && !isManagedSelection(modelLine.FindString(current), "model", strings.ReplaceAll(runtime.Model, "\"", "'")) {
		return "", false
	}
	marker := strings.Index(current, codexBegin)
	if marker < 0 {
		return "", false
	}
	providerRel := strings.Index(current[marker:], "[model_providers.aigw]")
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
	return runtime.Endpoint, nil
}

func projectCodex(original, block, model string) (string, error) {
	base, err := projectCodexScheduler(original)
	if err != nil {
		return "", err
	}
	base = strings.TrimRight(base, "\r\n")
	if modelProviderLine.MatchString(base) {
		base = modelProviderLine.ReplaceAllString(base, codexSelection)
	} else {
		base = codexSelection + "\n" + base
	}
	if model != "" {
		selection := fmt.Sprintf("model = \"%s\" # managed by AIGW", strings.ReplaceAll(model, `"`, `'`))
		if modelLine.MatchString(base) {
			base = modelLine.ReplaceAllString(base, selection)
		} else {
			base = selection + "\n" + base
		}
	}
	// A managed projection owns the provider block, not the incidental number
	// of blank lines before its ownership marker.  Keep the separator canonical
	// so a client formatter that folds adjacent blank lines cannot cause every
	// subsequent dry-run to report a needless update.
	return base + "\n" + codexBegin + "\n" + block, nil
}

func codexManagedBlock(runtime configuration.Runtime, endpoint string) string {
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
	if !modelProviderLine.MatchString(current) || !isManagedSelection(modelProviderLine.FindString(current), "model_provider", "aigw") {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed model_provider selection changed; refusing to overwrite user edits")
	}
	block, err := codexManagedBlockIn(current)
	if err != nil {
		return "", err
	}
	if !managedBlockHashMatches(state.ManagedBlockHash, block) {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed provider block changed; refusing to overwrite user edits")
	}
	if state.ProjectedSchedulerHash != "" && state.ProjectedSchedulerHash != codexSchedulerHash(current) {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed scheduler keys changed; refusing to overwrite user edits")
	}
	providerStart := strings.Index(current, "[model_providers.aigw]")
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
	pattern := `^[ \t]*` + regexp.QuoteMeta(key) + `[ \t]*=[ \t]*"` + regexp.QuoteMeta(value) + `"[ \t]*#[ \t]*managed by AIGW[ \t]*$`
	return regexp.MustCompile(pattern).MatchString(line)
}

func codexManagedBlockIn(current string) (string, error) {
	marker := strings.Index(current, codexBegin)
	if marker < 0 {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed provider block is missing")
	}
	providerRel := strings.Index(current[marker:], "[model_providers.aigw]")
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
	lines := strings.Split(base, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if strings.Contains(line, "# managed by AIGW") && strings.HasPrefix(strings.TrimSpace(line), "model") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func removeManagedModelSelection(base string) string {
	lines := strings.Split(base, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if strings.Contains(line, "# managed by AIGW") && strings.HasPrefix(strings.TrimSpace(line), "model") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
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
