package codex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"aigw-cli/internal/transaction"
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
)

// codexClient identifies one installed Codex build by the two facts AIGW can
// verify locally. A bundled catalog copy replaces the client's own table instead
// of merging with it, so a copy is only ever reusable for the exact build it was
// taken from.
type codexClient struct {
	Version string
	SHA256  string
}

func (c codexClient) known() bool { return c.Version != "" && c.SHA256 != "" }

// same reports whether two identities describe the same build. An unknown
// identity never matches, so an unreadable client cannot authorize reuse.
func (c codexClient) same(other codexClient) bool { return c.known() && c == other }

// codexBundledCatalog reads the installed client's own bundled model catalog.
// It is a package seam so tests can supply a catalog without an installed
// client; production reads the client itself.
var codexBundledCatalog = readCodexBundledCatalog

// codexCatalogPlan is the catalog decision for one target: the path the
// configuration should reference, the bytes AIGW should own, the client identity
// the bytes came from, and the ownership state recorded in the sidecar.
type codexCatalogPlan struct {
	path   string
	data   []byte
	client codexClient
	state  string
}

func codexCatalogPath(configPath string) string { return configPath + ".aigw-model-catalog.json" }

func targetCodexCatalogPath(target TargetRef) string { return codexCatalogPath(target.Path) }

// codexCatalogProjection decides what AIGW owns for one target without writing
// anything. It withholds a catalog whenever it cannot prove the adaptation is
// both needed and correct, so an unrecognized model keeps the client's own
// fallback and its warning instead of being silenced by a looser match.
func codexCatalogProjection(target TargetRef, model, base string, state codexState, before transaction.FileSnapshot) codexCatalogPlan {
	// A user-authored model_catalog_json is the user's own client policy. AIGW
	// replaces the bundled table wholesale, so adopting that key here would
	// silently drop models the user added.
	if model == "" || modelCatalogLine.MatchString(base) {
		return codexCatalogPlan{}
	}
	live, bundled, err := codexBundledCatalog(target.Executable)
	if err == nil {
		data, buildErr := buildCodexCatalog(bundled, model)
		if buildErr == nil {
			if data == nil {
				return codexCatalogPlan{}
			}
			return codexCatalogPlan{path: targetCodexCatalogPath(target), data: data, client: live, state: catalogStateProjected}
		}
	}
	// Regeneration failed. A previous copy is reusable only while it still
	// describes the installed build: after an upgrade the old snapshot would
	// override the client's newer bundled table, which is a worse failure than
	// the fallback it was meant to prevent.
	recorded := codexClient{Version: state.CatalogClientVersion, SHA256: state.CatalogClientSHA256}
	if state.CatalogHash != "" && live.same(recorded) && before.Exists && hashBytes(before.Data) == state.CatalogHash {
		return codexCatalogPlan{path: targetCodexCatalogPath(target), data: before.Data, client: recorded, state: catalogStateProjected}
	}
	if state.CatalogHash == "" && state.CatalogState == "" {
		// AIGW never owned a catalog here, so nothing was lost and there is
		// nothing to report; the client keeps the behavior it already had.
		return codexCatalogPlan{}
	}
	return codexCatalogPlan{client: recorded, state: catalogStateStale}
}

// codexCatalogDesiredSnapshot converts a catalog decision into the desired file
// state. AIGW removes a catalog only while the bytes on disk are still the ones
// it recorded writing, so a file it does not own survives untouched. For the
// same reason a file at the managed path that AIGW cannot prove is its own is
// not written over either: an unrecognized file there is a conflict to report,
// not a file to adopt.
func codexCatalogDesiredSnapshot(plan codexCatalogPlan, before transaction.FileSnapshot, ownedHash string) (transaction.FileSnapshot, error) {
	owned := before.Exists && ownedHash != "" && hashBytes(before.Data) == ownedHash
	if plan.data != nil {
		if before.Exists && !owned {
			return transaction.FileSnapshot{}, fmt.Errorf(
				"Codex config conflict: %s exists and is not the model catalog AIGW recorded writing; refusing to overwrite or re-permission it",
				plan.path,
			)
		}
		// The catalog carries the resolved account's model metadata, so its mode
		// is part of what AIGW owns rather than a user preference: a mode that
		// drifted wider converges back instead of being carried forward.
		return desiredCodexSnapshot(plan.data, ownerOnlyCatalogMode(before)), nil
	}
	if owned {
		return transaction.FileSnapshot{}, nil
	}
	return before, nil
}

// ownerOnlyCatalogMode is the mode an AIGW-owned catalog converges to. Windows
// cannot express owner-only in a file mode — Go reports 0666 for any writable
// file there — so on that platform the mode already on disk is left alone
// instead of being fought with on every sync.
func ownerOnlyCatalogMode(before transaction.FileSnapshot) os.FileMode {
	if runtime.GOOS == "windows" && before.Exists {
		return before.Mode
	}
	return os.FileMode(0o600)
}

// catalogModeIsEnforceable reports whether the platform represents owner-only
// permissions in a file mode, and therefore whether a mode read back from disk
// can be held to the contract.
func catalogModeIsEnforceable() bool {
	return runtime.GOOS != "windows"
}

func applyCodexCatalogState(state *codexState, plan codexCatalogPlan) {
	state.CatalogState = plan.state
	state.CatalogClientVersion = plan.client.Version
	state.CatalogClientSHA256 = plan.client.SHA256
	state.CatalogHash = ""
	if plan.data != nil {
		state.CatalogHash = hashBytes(plan.data)
	}
}

// codexCatalogDocument is the client's catalog document shape. Models are kept
// as raw members so every field of a cloned entry survives exactly, including
// fields a future client adds that AIGW knows nothing about.
type codexCatalogDocument struct {
	Models []map[string]json.RawMessage `json:"models"`
}

// buildCodexCatalog returns the catalog AIGW should own for one model id, or nil
// when the client already resolves that id by itself. The result is the client's
// complete bundled table plus aliases: a partial table would replace the
// client's own and push every model AIGW did not name back onto fallback.
func buildCodexCatalog(bundled []byte, model string) ([]byte, error) {
	var document codexCatalogDocument
	if err := json.Unmarshal(bundled, &document); err != nil {
		return nil, fmt.Errorf("parse Codex bundled model catalog: %w", err)
	}
	if len(document.Models) == 0 {
		return nil, fmt.Errorf("Codex bundled model catalog is empty")
	}
	slugs, err := codexCatalogSlugs(document)
	if err != nil {
		return nil, err
	}
	namespace, ok := codexCatalogNamespace(model, slugs)
	if !ok {
		return nil, nil
	}
	// Mirror the whole bundled table under the proven namespace. The provider
	// prefix belongs to the account, not to one model, so every model that
	// account can select resolves without AIGW keeping a list of model names.
	names := make([]string, 0, len(slugs))
	for slug := range slugs {
		names = append(names, slug)
	}
	sort.Strings(names)
	aliases := make([]map[string]json.RawMessage, 0, len(names))
	for _, slug := range names {
		alias := namespace + "." + slug
		if _, present := slugs[alias]; present {
			continue
		}
		encoded, err := json.Marshal(alias)
		if err != nil {
			return nil, fmt.Errorf("encode Codex model alias %q: %w", alias, err)
		}
		entry := make(map[string]json.RawMessage, len(document.Models[slugs[slug]]))
		for key, value := range document.Models[slugs[slug]] {
			entry[key] = value
		}
		entry["slug"] = encoded
		aliases = append(aliases, entry)
	}
	if len(aliases) == 0 {
		return nil, nil
	}
	document.Models = append(document.Models, aliases...)
	data, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode Codex model catalog: %w", err)
	}
	return append(data, '\n'), nil
}

func codexCatalogSlugs(document codexCatalogDocument) (map[string]int, error) {
	slugs := make(map[string]int, len(document.Models))
	for index, entry := range document.Models {
		raw, present := entry["slug"]
		if !present {
			return nil, fmt.Errorf("Codex bundled model catalog entry %d has no slug", index)
		}
		var slug string
		if err := json.Unmarshal(raw, &slug); err != nil {
			return nil, fmt.Errorf("parse Codex bundled model catalog entry %d slug: %w", index, err)
		}
		if slug == "" {
			return nil, fmt.Errorf("Codex bundled model catalog entry %d has an empty slug", index)
		}
		if _, duplicate := slugs[slug]; duplicate {
			return nil, fmt.Errorf("Codex bundled model catalog declares slug %q twice", slug)
		}
		slugs[slug] = index
	}
	return slugs, nil
}

// codexCatalogNamespace splits a provider-prefixed model id into the namespace
// the provider adds and the client slug it wraps. Every dot-separated suffix is
// matched exactly against the client's own slugs and exactly one match is
// required, so neither a multi-level namespace nor a slug that itself contains
// dots can be mapped by accident. An id the client already knows needs no alias.
func codexCatalogNamespace(model string, slugs map[string]int) (string, bool) {
	if _, present := slugs[model]; present {
		return "", false
	}
	namespace := ""
	matches := 0
	for index := 0; index < len(model); index++ {
		if model[index] != '.' || index == 0 {
			continue
		}
		suffix := model[index+1:]
		if suffix == "" {
			continue
		}
		if _, present := slugs[suffix]; !present {
			continue
		}
		namespace = model[:index]
		matches++
	}
	if matches != 1 {
		return "", false
	}
	return namespace, true
}

// readCodexBundledCatalog reads the installed client's identity and its bundled
// catalog. The identity is returned even when the catalog read fails, because
// deciding whether a previous copy may be reused requires knowing which build is
// installed now.
func readCodexBundledCatalog(executable string) (codexClient, []byte, error) {
	if strings.TrimSpace(executable) == "" {
		return codexClient{}, nil, fmt.Errorf("Codex executable is not configured")
	}
	sum, err := codexFileSHA256(executable)
	if err != nil {
		return codexClient{}, nil, err
	}
	version, err := runCodexReadOnly(executable, "--version")
	if err != nil {
		return codexClient{}, nil, err
	}
	client := codexClient{Version: strings.TrimSpace(string(version)), SHA256: sum}
	if client.Version == "" {
		return codexClient{}, nil, fmt.Errorf("Codex reported no version")
	}
	catalog, err := runCodexReadOnly(executable, "debug", "models", "--bundled")
	if err != nil {
		return client, nil, err
	}
	return client, catalog, nil
}

// runCodexReadOnly runs one read-only client command against a throwaway Codex
// home. The empty home is what makes the result trustworthy: the bundled table
// cannot be read through a configuration AIGW itself projected, so regeneration
// can never feed on its own previous output.
func runCodexReadOnly(executable string, args ...string) ([]byte, error) {
	home, err := os.MkdirTemp("", "aigw-codex-catalog-")
	if err != nil {
		return nil, fmt.Errorf("create Codex probe home: %w", err)
	}
	defer func() { _ = os.RemoveAll(home) }()
	ctx, cancel := context.WithTimeout(context.Background(), codexCatalogTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, args...)
	command.Env = append(removeEnvironment(os.Environ(), "CODEX_HOME"), "CODEX_HOME="+home)
	stdout := &bytes.Buffer{}
	command.Stdout = stdout
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("run %s %s: %w", filepath.Base(executable), strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

func codexFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read Codex executable: %w", err)
	}
	defer func() { _ = file.Close() }()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", fmt.Errorf("hash Codex executable: %w", err)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// codexTOMLString quotes a value as a TOML basic string. A control character
// cannot be expressed there, so it is refused rather than written out as a
// document the client would reject.
func codexTOMLString(value string) (string, error) {
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return "", fmt.Errorf("Codex model catalog path contains a control character")
		}
	}
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`, nil
}
