package adapters

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

const (
	codexSelection = `model_provider = "aigw" # managed by AIGW`
	codexBegin     = "# >>> AIGW managed provider >>>"
	codexEnd       = "# <<< AIGW managed provider <<<"
)

var modelProviderLine = regexp.MustCompile(`(?m)^[ \t]*model_provider[ \t]*=.*$`)
var modelLine = regexp.MustCompile(`(?m)^[ \t]*model[ \t]*=.*$`)

type codexState struct {
	OriginalProvider string `json:"original_provider,omitempty"`
	OriginalModel    string `json:"original_model,omitempty"`
	ManagedBlockHash string `json:"managed_block_hash"`
}

// CodexProjectionPlan is a non-secret, read-only rendering of one target's
// proposed configuration projection. It never includes configuration content,
// credentials, or state bodies.
type CodexProjectionPlan struct {
	Target string `json:"target"`
	Action string `json:"action"`
}

type codexTargetSnapshot struct {
	path       string
	config     []byte
	configMode os.FileMode
	state      []byte
	stateMode  os.FileMode
	stateExist bool
}

type preparedCodexProjection struct {
	target   string
	config   []byte
	state    []byte
	action   string
	snapshot codexTargetSnapshot
}

// writeFileAtomic is an injected seam for deterministic commit/rollback tests.
// Production code uses transaction.WriteFileAtomic.
var writeFileAtomic = transaction.WriteFileAtomic

func CodexLoginPlan(executable, codexHome, token string) (ProcessPlan, error) {
	if executable == "" {
		return ProcessPlan{}, fmt.Errorf("Codex executable is not configured")
	}
	if token == "" {
		return ProcessPlan{}, fmt.Errorf("Codex token is empty")
	}
	env := os.Environ()
	if codexHome != "" {
		env = removeEnvironment(env, "CODEX_HOME")
		env = append(env, "CODEX_HOME="+codexHome)
	}
	return ProcessPlan{
		Executable: executable,
		Args:       []string{"login", "--with-api-key"},
		Env:        env,
		Stdin:      token + "\n",
	}, nil
}

func SyncCodexConfig(path string, runtime domain.Runtime) error {
	return SyncCodexConfigs([]string{path}, runtime)
}

// PlanCodexConfigs performs every projection read and conflict check without
// writing. It is the dry-run boundary used by the CLI and callers that need
// evidence before mutation.
func PlanCodexConfigs(paths []string, runtime domain.Runtime) ([]CodexProjectionPlan, error) {
	prepared, err := prepareCodexProjections(paths, runtime)
	if err != nil {
		return nil, err
	}
	plans := make([]CodexProjectionPlan, 0, len(prepared))
	for _, projection := range prepared {
		plans = append(plans, CodexProjectionPlan{Target: projection.target, Action: projection.action})
	}
	return plans, nil
}

// SyncCodexConfigs is an all-target transaction. It prepares every target
// before the first write; a later conflict therefore cannot leave an earlier
// profile half-synchronized. If any atomic write fails, every configuration and
// sidecar returns to its byte-exact pre-state, including an absent sidecar.
func SyncCodexConfigs(paths []string, runtime domain.Runtime) error {
	prepared, err := prepareCodexProjections(paths, runtime)
	if err != nil {
		return err
	}
	committed := make([]preparedCodexProjection, 0, len(prepared))
	for _, projection := range prepared {
		if projection.action == "already-converged" {
			continue
		}
		committed = append(committed, projection)
		if err := writePreparedCodexProjection(projection); err != nil {
			rollbackErr := rollbackCodexProjections(committed)
			if rollbackErr != nil {
				return fmt.Errorf("commit Codex projection %s: %w; rollback also failed: %v", projection.target, err, rollbackErr)
			}
			return fmt.Errorf("commit Codex projection %s: %w; all targets rolled back", projection.target, err)
		}
	}
	return nil
}

func prepareCodexProjections(paths []string, runtime domain.Runtime) ([]preparedCodexProjection, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	endpoint, err := codexEndpoint(runtime)
	if err != nil {
		return nil, err
	}
	block := codexManagedBlock(runtime.ProfileLabel, endpoint)
	seen := make(map[string]struct{}, len(paths))
	prepared := make([]preparedCodexProjection, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			return nil, fmt.Errorf("Codex config target is empty")
		}
		if _, duplicate := seen[path]; duplicate {
			return nil, fmt.Errorf("Codex config target %s is duplicated", path)
		}
		seen[path] = struct{}{}
		projection, err := prepareCodexProjection(path, runtime, block)
		if err != nil {
			return nil, fmt.Errorf("prepare Codex projection %s: %w", path, err)
		}
		prepared = append(prepared, projection)
	}
	return prepared, nil
}

func prepareCodexProjection(path string, runtime domain.Runtime, block string) (preparedCodexProjection, error) {
	snapshot, err := readCodexTargetSnapshot(path)
	if err != nil {
		return preparedCodexProjection{}, err
	}
	base, state, err := codexUserConfig(path, runtime, block)
	if err != nil {
		return preparedCodexProjection{}, err
	}
	projected := projectCodex(base, block, runtime.Model)
	state.ManagedBlockHash = hashText(block)
	stateData, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return preparedCodexProjection{}, fmt.Errorf("encode Codex adapter state: %w", err)
	}
	stateData = append(stateData, '\n')
	action := "update"
	if string(snapshot.config) == projected && snapshot.stateExist && string(snapshot.state) == string(stateData) {
		action = "already-converged"
	} else if isExactTruncatedCodexProjection(string(snapshot.config), snapshot.state, runtime, block) {
		action = "repair-truncated"
	} else if !snapshot.stateExist {
		action = "initial-project"
	}
	return preparedCodexProjection{
		target:   path,
		config:   []byte(projected),
		state:    stateData,
		action:   action,
		snapshot: snapshot,
	}, nil
}

func isExactTruncatedCodexProjection(current string, stateData []byte, runtime domain.Runtime, block string) bool {
	var state codexState
	if len(stateData) == 0 || json.Unmarshal(stateData, &state) != nil {
		return false
	}
	_, ok := completeExactTruncatedCodexProjection(current, state, runtime, block)
	return ok
}

func readCodexTargetSnapshot(path string) (codexTargetSnapshot, error) {
	config, err := os.ReadFile(path)
	if err != nil {
		return codexTargetSnapshot{}, fmt.Errorf("read Codex config: %w", err)
	}
	configMode := os.FileMode(0o600)
	if info, statErr := os.Stat(path); statErr == nil {
		configMode = info.Mode().Perm()
	} else {
		return codexTargetSnapshot{}, fmt.Errorf("inspect Codex config: %w", statErr)
	}
	snapshot := codexTargetSnapshot{path: path, config: config, configMode: configMode}
	statePath := codexStatePath(path)
	state, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return snapshot, nil
	}
	if err != nil {
		return codexTargetSnapshot{}, fmt.Errorf("read Codex adapter state: %w", err)
	}
	snapshot.state = state
	snapshot.stateExist = true
	if info, statErr := os.Stat(statePath); statErr == nil {
		snapshot.stateMode = info.Mode().Perm()
	} else {
		return codexTargetSnapshot{}, fmt.Errorf("inspect Codex adapter state: %w", statErr)
	}
	return snapshot, nil
}

func writePreparedCodexProjection(projection preparedCodexProjection) error {
	if err := writeFileAtomic(projection.target, projection.config, projection.snapshot.configMode); err != nil {
		return err
	}
	if err := writeFileAtomic(codexStatePath(projection.target), projection.state, 0o600); err != nil {
		return err
	}
	return nil
}

func rollbackCodexProjections(projections []preparedCodexProjection) error {
	var failures []string
	for index := len(projections) - 1; index >= 0; index-- {
		projection := projections[index]
		if err := writeFileAtomic(projection.target, projection.snapshot.config, projection.snapshot.configMode); err != nil {
			failures = append(failures, fmt.Sprintf("restore %s: %v", projection.target, err))
		}
		statePath := codexStatePath(projection.target)
		if projection.snapshot.stateExist {
			if err := writeFileAtomic(statePath, projection.snapshot.state, projection.snapshot.stateMode); err != nil {
				failures = append(failures, fmt.Sprintf("restore %s: %v", statePath, err))
			}
		} else if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
			failures = append(failures, fmt.Sprintf("remove %s: %v", statePath, err))
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

// ValidateCodexConfig verifies that a Codex target still matches the resolved
// AIGW profile. It never changes the target; callers can safely use it for
// diagnostics before offering an explicit sync.
func ValidateCodexConfig(path string, runtime domain.Runtime) error {
	endpoint, err := codexEndpoint(runtime)
	if err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read Codex config: %w", err)
	}
	text := string(current)
	expectedBlock := codexManagedBlock(runtime.ProfileLabel, endpoint)
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
		return nil
	}
	return fmt.Errorf("Codex config provider selection does not match AIGW")
}

func DisableCodexConfig(path string) error {
	stateData, err := os.ReadFile(codexStatePath(path))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read Codex adapter state: %w", err)
	}
	var state codexState
	if err := json.Unmarshal(stateData, &state); err != nil {
		return fmt.Errorf("parse Codex adapter state: %w", err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read Codex config: %w", err)
	}
	base, err := removeCodexProjection(string(current), state)
	if err != nil {
		return err
	}
	if err := transaction.WriteFileAtomic(path, []byte(base), 0o600); err != nil {
		return err
	}
	if err := os.Remove(codexStatePath(path)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove Codex adapter state: %w", err)
	}
	return nil
}

func codexUserConfig(path string, runtime domain.Runtime, expectedBlock string) (string, codexState, error) {
	stateData, err := os.ReadFile(codexStatePath(path))
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
	return string(data), codexState{OriginalProvider: originalProvider, OriginalModel: originalModel}, nil
}

// completeExactTruncatedCodexProjection admits only the known interrupted
// projection shape: the current runtime's complete owned block with its final
// ownership marker omitted. It returns an in-memory completion so the caller's
// normal atomic projection transaction remains the sole write path.
func completeExactTruncatedCodexProjection(current string, state codexState, runtime domain.Runtime, expectedBlock string) (string, bool) {
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

func codexEndpoint(runtime domain.Runtime) (string, error) {
	if runtime.Endpoint == "" {
		return "", fmt.Errorf("profile %q has no Codex endpoint", runtime.ProfileID)
	}
	return runtime.Endpoint, nil
}

func projectCodex(original, block, model string) string {
	base := strings.TrimRight(original, "\r\n")
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
	return base + "\n" + codexBegin + "\n" + block
}

func codexManagedBlock(label, endpoint string) string {
	label = strings.ReplaceAll(label, `"`, `'`)
	return "[model_providers.aigw]\n" +
		fmt.Sprintf("name = \"AIGW: %s\"\n", label) +
		fmt.Sprintf("base_url = \"%s\"\n", endpoint) +
		"wire_api = \"responses\"\n" +
		"requires_openai_auth = true\n" +
		codexEnd + "\n"
}

func removeCodexProjection(current string, state codexState) (string, error) {
	legacyState := state.OriginalProvider == "" && state.OriginalModel == ""
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
	providerStart := strings.Index(current, "[model_providers.aigw]")
	providerEnd := providerStart + len(block)
	base := strings.TrimRight(current[:providerStart]+current[providerEnd:], "\r\n")
	base = removeCodexBeginMarker(base)
	base = strings.TrimRight(base, "\r\n")
	if legacyState {
		base = removeManagedModelSelection(base)
		base = modelProviderLine.ReplaceAllString(base, "")
		base = strings.TrimLeft(base, "\r\n")
		return base + "\n", nil
	}
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
	return base + "\n", nil
}

// isManagedSelection accepts harmless formatter changes such as the padded
// top-level assignments written by JetBrains. Values and the ownership marker
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
