package adapters

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

func SyncCodexConfig(path string, profile domain.Profile) error {
	endpoint, err := profile.EndpointFor(domain.ClientCodex)
	if err != nil {
		return err
	}
	base, state, err := codexUserConfig(path)
	if err != nil {
		return err
	}
	model := profile.ModelFor(domain.ClientCodex)
	block := codexManagedBlock(profile.Label, endpoint)
	projected := projectCodex(base, block, model)
	state.ManagedBlockHash = hashText(block)
	stateData, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Codex adapter state: %w", err)
	}
	if err := transaction.WriteFileAtomic(path, []byte(projected), 0o600); err != nil {
		return err
	}
	if err := transaction.WriteFileAtomic(codexStatePath(path), append(stateData, '\n'), 0o600); err != nil {
		_ = transaction.WriteFileAtomic(path, []byte(base), 0o600)
		return err
	}
	return nil
}

// ValidateCodexConfig verifies that a Codex target still matches the resolved
// AIGW profile. It never changes the target; callers can safely use it for
// diagnostics before offering an explicit sync.
func ValidateCodexConfig(path string, profile domain.Profile) error {
	endpoint, err := profile.EndpointFor(domain.ClientCodex)
	if err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read Codex config: %w", err)
	}
	text := string(current)
	if !isManagedSelection(modelProviderLine.FindString(text), "model_provider", "aigw") {
		return fmt.Errorf("Codex config provider selection does not match AIGW")
	}
	if model := profile.ModelFor(domain.ClientCodex); model != "" {
		if !isManagedSelection(modelLine.FindString(text), "model", strings.ReplaceAll(model, "\"", "'")) {
			return fmt.Errorf("Codex config model selection does not match profile %q", profile.ID)
		}
	}
	expectedBlock := codexManagedBlock(profile.Label, endpoint)
	actualBlock, err := codexManagedBlockIn(text)
	if err != nil {
		return err
	}
	if hashText(actualBlock) != hashText(expectedBlock) {
		return fmt.Errorf("Codex config provider block does not match profile %q", profile.ID)
	}
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
	if !managedBlockHashMatches(state.ManagedBlockHash, actualBlock) {
		return fmt.Errorf("Codex config AIGW state does not match profile %q", profile.ID)
	}
	return nil
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

func codexUserConfig(path string) (string, codexState, error) {
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
		if state.OriginalModel == "" {
			state.OriginalModel = unmarkManagedModel(modelLine.FindString(string(current)))
		}
		base, err := removeCodexProjection(string(current), state)
		if err != nil {
			return "", codexState{}, err
		}
		if state.OriginalModel == "" {
			state.OriginalModel = modelLine.FindString(base)
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
	return base + "\n\n" + codexBegin + "\n" + block
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
	if state.OriginalProvider != "" {
		base = modelProviderLine.ReplaceAllString(base, state.OriginalProvider)
	} else {
		base = modelProviderLine.ReplaceAllString(base, "")
		base = strings.TrimLeft(base, "\r\n")
	}
	base = restoreModelSelection(base, state.OriginalModel)
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
	return stateHash == hashText(block) || stateHash == hashText(codexBegin+"\n"+block)
}

func removeCodexBeginMarker(text string) string {
	for _, marker := range []string{codexBegin + "\r\n", codexBegin + "\n", codexBegin} {
		if strings.Contains(text, marker) {
			return strings.Replace(text, marker, "", 1)
		}
	}
	return text
}

func unmarkManagedModel(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if before, _, ok := strings.Cut(line, "# managed by AIGW"); ok {
		return strings.TrimSpace(before)
	}
	return line
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

func codexStatePath(path string) string { return path + ".aigw-state.json" }

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
