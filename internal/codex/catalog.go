package codex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"aigw-cli/internal/codex/catalog"
	"aigw-cli/internal/process"
	"aigw-cli/internal/transaction"

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
// the bytes came from, and the ownership state recorded in the sidecar.
type codexCatalogPlan struct {
	path   string
	data   []byte
	client ExecutableIdentity
	state  string
}

func codexCatalogPath(configPath string) string { return configPath + ".aigw-model-catalog.json" }

// codexCatalogProjection decides what AIGW owns for one target without writing
// anything. It withholds a catalog whenever it cannot prove the adaptation is
// both needed and correct, so an unrecognized model keeps the client's own
// fallback and its warning instead of being silenced by a looser match.
func codexCatalogProjection(target TargetRef, model, base string, state codexState, before transaction.FileSnapshot) codexCatalogPlan {
	// A user-authored model_catalog_json is the user's own client policy. AIGW
	// replaces the bundled table wholesale, so adopting that key here would
	// silently drop models the user added.
	selection, selectionErr := codexSelectionLine(base, "model_catalog_json")
	if model == "" || selectionErr != nil || selection != "" {
		return codexCatalogPlan{}
	}
	live, bundled, err := codexBundledCatalog(target.Executable)
	if err == nil {
		document, parseErr := catalog.Parse(bundled)
		if parseErr == nil {
			data, _ := document.Project(model)
			if data == nil {
				return codexCatalogPlan{}
			}
			return codexCatalogPlan{path: codexCatalogPath(target.Path), data: data, client: live, state: catalogStateProjected}
		}
	}
	// Regeneration failed. A previous copy is reusable only while it still
	// describes the installed build: after an upgrade the old snapshot would
	// override the client's newer bundled table, which is a worse failure than
	// the fallback it was meant to prevent.
	recorded := ExecutableIdentity{Version: state.CatalogClientVersion, SHA256: state.CatalogClientSHA256}
	if state.CatalogHash != "" && live.same(recorded) && before.Exists && hashBytes(before.Data) == state.CatalogHash {
		return codexCatalogPlan{path: codexCatalogPath(target.Path), data: before.Data, client: recorded, state: catalogStateProjected}
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
		return transaction.NewFileSnapshot(plan.data, ownerOnlyCatalogMode(before)), nil
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

// ReadBundledCatalog reads the installed client's identity and its bundled
// catalog. The identity is returned even when the catalog read fails, because
// deciding whether a previous copy may be reused requires knowing which build is
// installed now.
func ReadBundledCatalog(executable string) (client ExecutableIdentity, data []byte, result error) {
	if strings.TrimSpace(executable) == "" {
		return ExecutableIdentity{}, nil, fmt.Errorf("Codex executable is not configured")
	}
	result = withCatalogProbe(func(ctx context.Context, home string) error {
		var err error
		client, err = IdentifyExecutable(ctx, process.Runner{}, executable, home)
		if err != nil {
			return err
		}
		data, err = runCodexReadOnly(ctx, process.Runner{StdoutLimit: codexCatalogByteLimit}, executable, home, "debug", "models", "--bundled")
		return err
	})
	return client, data, result
}

// ReadEffectiveCatalog asks the installed client to render its selected catalogue
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
	result = withCatalogProbe(func(ctx context.Context, home string) error {
		var err error
		data, err = runCodexReadOnly(ctx, process.Runner{StdoutLimit: codexCatalogByteLimit}, executable, home, args...)
		return err
	})
	return data, result
}

func withCatalogProbe(probe func(context.Context, string) error) (result error) {
	home, err := os.MkdirTemp("", "aigw-codex-catalog-")
	if err != nil {
		return fmt.Errorf("create Codex probe home: %w", err)
	}
	defer func() {
		if err := removeCatalogProbe(home); err != nil {
			result = errors.Join(result, fmt.Errorf("remove Codex probe home %s: %w", home, err))
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), codexCatalogTimeout)
	defer cancel()
	return probe(ctx, home)
}
