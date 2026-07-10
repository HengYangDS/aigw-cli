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

var modelProviderLine = regexp.MustCompile(`(?m)^model_provider\s*=.*$`)

type codexState struct {
	OriginalProvider string `json:"original_provider,omitempty"`
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
	block := codexManagedBlock(profile.Label, endpoint, profile.ModelFor(domain.ClientCodex))
	projected := projectCodex(base, block)
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
		base, err := removeCodexProjection(string(current), state)
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
	return string(data), codexState{OriginalProvider: originalProvider}, nil
}

func projectCodex(original, block string) string {
	base := strings.TrimRight(original, "\r\n")
	if modelProviderLine.MatchString(base) {
		base = modelProviderLine.ReplaceAllString(base, codexSelection)
	} else {
		base = codexSelection + "\n" + base
	}
	return base + "\n\n" + block
}

func codexManagedBlock(label, endpoint, model string) string {
	label = strings.ReplaceAll(label, `"`, `'`)
	block := codexBegin + "\n" +
		"[model_providers.aigw]\n" +
		fmt.Sprintf("name = \"AIGW: %s\"\n", label) +
		fmt.Sprintf("base_url = \"%s\"\n", endpoint) +
		"wire_api = \"responses\"\n" +
		"requires_openai_auth = true\n"
	if model != "" {
		block += fmt.Sprintf("model = \"%s\"\n", strings.ReplaceAll(model, `"`, `'`))
	}
	return block + codexEnd + "\n"
}

func removeCodexProjection(current string, state codexState) (string, error) {
	if !modelProviderLine.MatchString(current) || modelProviderLine.FindString(current) != codexSelection {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed model_provider selection changed; refusing to overwrite user edits")
	}
	start := strings.Index(current, codexBegin)
	if start < 0 {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed provider block is missing")
	}
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
	block := current[start:end]
	if hashText(block) != state.ManagedBlockHash {
		return "", fmt.Errorf("Codex config conflict: AIGW-managed provider block changed; refusing to overwrite user edits")
	}
	base := strings.TrimRight(current[:start]+current[end:], "\r\n")
	if state.OriginalProvider != "" {
		base = strings.Replace(base, codexSelection, state.OriginalProvider, 1)
	} else {
		base = strings.TrimPrefix(base, codexSelection+"\n")
		base = strings.TrimPrefix(base, codexSelection+"\r\n")
		base = strings.TrimPrefix(base, codexSelection)
	}
	return base + "\n", nil
}

func codexStatePath(path string) string { return path + ".aigw-state.json" }

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
