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
	Original      string `json:"original"`
	ProjectedHash string `json:"projected_hash"`
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
	original, err := originalCodexConfig(path)
	if err != nil {
		return err
	}
	projected := projectCodex(original, profile.Label, endpoint)
	state := codexState{Original: original, ProjectedHash: hashText(projected)}
	stateData, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Codex adapter state: %w", err)
	}
	if err := transaction.WriteFileAtomic(path, []byte(projected), 0o600); err != nil {
		return err
	}
	if err := transaction.WriteFileAtomic(codexStatePath(path), append(stateData, '\n'), 0o600); err != nil {
		_ = transaction.WriteFileAtomic(path, []byte(original), 0o600)
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
	if hashText(string(current)) != state.ProjectedHash {
		return fmt.Errorf("Codex config conflict: managed projection changed after AIGW wrote it; refusing to overwrite user edits")
	}
	if err := transaction.WriteFileAtomic(path, []byte(state.Original), 0o600); err != nil {
		return err
	}
	if err := os.Remove(codexStatePath(path)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove Codex adapter state: %w", err)
	}
	return nil
}

func originalCodexConfig(path string) (string, error) {
	stateData, err := os.ReadFile(codexStatePath(path))
	if err == nil {
		var state codexState
		if err := json.Unmarshal(stateData, &state); err != nil {
			return "", fmt.Errorf("parse Codex adapter state: %w", err)
		}
		current, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read Codex config: %w", err)
		}
		if hashText(string(current)) != state.ProjectedHash {
			return "", fmt.Errorf("Codex config conflict: managed projection changed after AIGW wrote it; refusing to sync")
		}
		return state.Original, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("read Codex adapter state: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read Codex config: %w", err)
	}
	return string(data), nil
}

func projectCodex(original, label, endpoint string) string {
	base := strings.TrimRight(original, "\n")
	if modelProviderLine.MatchString(base) {
		base = modelProviderLine.ReplaceAllString(base, codexSelection)
	} else {
		base = codexSelection + "\n" + base
	}
	label = strings.ReplaceAll(label, `"`, `'`)
	return base + "\n\n" + codexBegin + "\n" +
		"[model_providers.aigw]\n" +
		fmt.Sprintf("name = \"AIGW: %s\"\n", label) +
		fmt.Sprintf("base_url = \"%s\"\n", endpoint) +
		"wire_api = \"responses\"\n" +
		"requires_openai_auth = true\n" + codexEnd + "\n"
}

func codexStatePath(path string) string { return path + ".aigw-state.json" }

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
