package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/processtest"
	"aigw-cli/internal/secret"
)

// fakeCodexScript stands for Codex: it prints its version and windowedTable
// as its bundled model table, and notes each run in the file runs beside it.
const fakeCodexScript = `#!/bin/sh
here=$(dirname "$0")
echo "$*" >> "$here/runs"
case "$1" in
--version) echo "codex-cli 1.0.0" ;;
debug) cat "$here/models.json" ;;
*) exit 2 ;;
esac
`

// codexProgram is a Codex whose program is fakeCodexScript at program, the
// selection of gateway/gpt-6.2-sol for Codex's default home, and the
// dependencies of a synchronization at the adapter layer. package is the
// folder that holds the program's package, which the test removes to remove
// the program.
type codexProgram struct {
	program, pkg, home string
	cfg                domain.Config
	deps               adapter.Dependencies
}

// caskLayout is Homebrew's: <package>/bin/codex, with the package manifest in
// the package folder. extensionLayout is an editor extension's
// bin/<os>-<arch>/codex, for which AIGW declares no manifest.
const (
	caskLayout      = "Caskroom/codex/1.0.0"
	extensionLayout = ".vscode-server/extensions/openai.chatgpt-1.0.0-linux-arm64"
)

func newCodexProgram(t *testing.T, layout string) codexProgram {
	t.Helper()
	root := t.TempDir()
	pkg := filepath.Join(root, layout)
	directory := filepath.Join(pkg, "bin")
	if layout == extensionLayout {
		directory = filepath.Join(directory, "linux-aarch64")
	} else {
		writeTo(t, filepath.Join(pkg, "codex-package.json"), `{"layoutVersion":1,"version":"1.0.0"}`)
	}
	writeTo(t, filepath.Join(directory, "models.json"), windowedTable)
	program := filepath.Join(directory, "codex")
	processtest.Write(t, program, fakeCodexScript)
	home := filepath.Join(root, ".codex", "config.toml")
	writeTo(t, home, "model_provider = \"native\"\n")
	cfg := windowConfiguration("gpt-6.2-sol", "gpt-6.2-sol", home)
	binding := cfg.Clients[ID]
	binding.Executable = program
	cfg.Clients[ID] = binding
	tokens := secret.NewMemoryStore()
	if err := tokens.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	// Discovery finds no Codex: the program is in no folder of PATH.
	deps := adapter.Dependencies{
		Secrets: tokens, AIGWExecutable: filepath.Join(root, "aigw"), Host: discovery.Host{Home: root},
		Discovery: fixedDiscoverer{result: discovery.Result{Surfaces: []discovery.Surface{{ID: string(HomeDefault), ConfigPath: home, AutoManaged: true}}}},
	}
	return codexProgram{program: program, pkg: pkg, home: home, cfg: cfg, deps: deps}
}

// sync converges and applies Codex's files from the fixture's configuration
// as `aigw sync` does, with the real reader of Codex's bundled table, and
// returns the configuration that convergence kept.
func (fixture codexProgram) sync(t *testing.T) (domain.Config, error) {
	t.Helper()
	registry := codexRegistry()
	after, err := registry.Converge(fixture.deps, fixture.cfg, fixture.deps.Discovery.Discover(ID), ID)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = registry.Apply(context.Background(), fixture.deps, fixture.cfg, after, ID)
	return after, err
}

// inspect checks Codex's files as `aigw check --depth local` does.
func (fixture codexProgram) inspect(t *testing.T, cfg domain.Config) adapter.Status {
	t.Helper()
	runtime, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	return codexRegistry().Inspect(context.Background(), fixture.deps, cfg, ID, runtime)
}

// runs returns what the fixture's program was asked since the last call, and
// forgets it.
func (fixture codexProgram) runs(t *testing.T) []string {
	t.Helper()
	path := filepath.Join(filepath.Dir(fixture.program), "runs")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// files reads Codex's configuration file, AIGW's record of it and its model
// table.
func (fixture codexProgram) files(t *testing.T) [][]byte {
	t.Helper()
	contents := [][]byte{}
	for _, path := range []string{fixture.home, codexRecordPath(fixture.home), codexCatalogPath(fixture.home)} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		contents = append(contents, data)
	}
	return contents
}

// A sync records the program that produced Codex's model table, by path,
// digest and the version of its package manifest, and a local check compares
// the configured program with it. Each difference fails the check with a sync
// as its fix, after only the read-only probes that tell that a sync can
// produce a table; a check of the same program runs nothing. A program whose
// package has no manifest is compared by path and digest alone.
func TestACheckComparesTheTablesProducerWithTheConfiguredProgram(t *testing.T) {
	for name, test := range map[string]struct {
		layout  string
		change  func(t *testing.T, fixture codexProgram, cfg *domain.Config)
		problem string
	}{
		"nothing changed":              {layout: caskLayout, change: func(*testing.T, codexProgram, *domain.Config) {}},
		"a program without a manifest": {layout: extensionLayout, change: func(*testing.T, codexProgram, *domain.Config) {}},
		"the program changed in place": {
			layout: caskLayout,
			change: func(t *testing.T, fixture codexProgram, _ *domain.Config) {
				appendTo(t, fixture.program, "# another build\n")
			},
			problem: "Codex's configured executable changed after it produced Codex's model table",
		},
		"another program is configured": {
			layout: caskLayout,
			change: func(t *testing.T, fixture codexProgram, cfg *domain.Config) {
				other := filepath.Join(filepath.Dir(fixture.program), "codex-next")
				processtest.Write(t, other, fakeCodexScript)
				binding := cfg.Clients[ID]
				binding.Executable = other
				cfg.Clients[ID] = binding
			},
			problem: "Codex's model table came from another executable than the configured one",
		},
		"the manifest declares another version": {
			layout: caskLayout,
			change: func(t *testing.T, fixture codexProgram, _ *domain.Config) {
				writeTo(t, filepath.Join(fixture.pkg, "codex-package.json"), `{"version":"1.1.0"}`)
			},
			problem: "The package of Codex's configured executable now declares version 1.1.0, and Codex's model table came from version 1.0.0",
		},
		"the manifest is gone": {
			layout: caskLayout,
			change: func(t *testing.T, fixture codexProgram, _ *domain.Config) {
				if err := os.Remove(filepath.Join(fixture.pkg, "codex-package.json")); err != nil {
					t.Fatal(err)
				}
			},
			problem: "The package of Codex's configured executable now declares no version, and Codex's model table came from version 1.0.0",
		},
		"the record names no producer": {
			layout: caskLayout,
			change: func(t *testing.T, fixture codexProgram, _ *domain.Config) {
				record := codexRecordPath(fixture.home)
				data, err := os.ReadFile(record)
				if err != nil {
					t.Fatal(err)
				}
				writeTo(t, record, strings.Replace(string(data), `  "catalog_client_path": "`+fixture.program+`",`+"\n", "", 1))
			},
			problem: "AIGW's record does not name the executable that produced Codex's model table",
		},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newCodexProgram(t, test.layout)
			if _, err := fixture.sync(t); err != nil {
				t.Fatal(err)
			}
			state := readCodexSidecar(t, fixture.home)
			wantPackage := "1.0.0"
			if test.layout == extensionLayout {
				wantPackage = ""
			}
			if state.CatalogHash == "" || state.CatalogClientPath != fixture.program || state.CatalogClientPackage != wantPackage || state.CatalogClientSHA256 == "" {
				t.Fatalf("the record names the producer as %q %q %q", state.CatalogClientPath, state.CatalogClientPackage, state.CatalogClientSHA256)
			}
			cfg := fixture.cfg.Clone()
			test.change(t, fixture, &cfg)
			fixture.runs(t)
			status := fixture.inspect(t, cfg)
			runs := fixture.runs(t)
			if test.problem == "" {
				if !status.Ready || len(runs) != 0 {
					t.Fatalf("status = %+v after the runs %q, want ready and no run", status, runs)
				}
				return
			}
			if status.Ready || status.Issue != test.problem || status.RepairAction != "aigw sync" {
				t.Fatalf("status = %+v, want %q with aigw sync", status, test.problem)
			}
			if slices.ContainsFunc(runs, func(run string) bool { return run != "--version" && run != "debug models --bundled" }) {
				t.Fatalf("the check ran %q, want the read-only probes alone", runs)
			}
		})
	}
}

// A Codex whose configured executable runs but cannot produce its model table
// stops: each sync keeps Codex's file, AIGW's record and the table byte for
// byte and fails, and a check names the same problem and fix, which points at
// the executable, never at a sync.
func TestACodexThatCannotProduceItsTableStops(t *testing.T) {
	fixture := newCodexProgram(t, caskLayout)
	if _, err := fixture.sync(t); err != nil {
		t.Fatal(err)
	}
	// The program is still there and changed, but prints no table.
	if err := os.Remove(filepath.Join(filepath.Dir(fixture.program), "models.json")); err != nil {
		t.Fatal(err)
	}
	appendTo(t, fixture.program, "# a build that prints no table\n")
	before := fixture.files(t)
	var stops []*adapter.StopError
	for attempt := range 2 {
		_, err := fixture.sync(t)
		stop, stopped := errors.AsType[*adapter.StopError](err)
		if !stopped || stop.NextAction() != tableFix || !strings.HasPrefix(stop.Error(), "Codex's configured executable cannot produce its model table") {
			t.Fatalf("sync %d = %v, want Codex stopped for its table", attempt+2, err)
		}
		if after := fixture.files(t); !slices.EqualFunc(after, before, slices.Equal) {
			t.Fatalf("sync %d changed Codex's files", attempt+2)
		}
		stops = append(stops, stop)
	}
	status := fixture.inspect(t, fixture.cfg)
	if status.Ready || status.Issue != stops[0].Error() || status.Issue != stops[1].Error() || status.RepairAction != tableFix {
		t.Fatalf("status = %+v, want the sync's problem %q and fix", status, stops[0])
	}
}

// A Codex whose configured executable runs and has never produced a model
// table for AIGW, and cannot, is no loop: each sync succeeds without a table,
// the second writes nothing, and each check passes and names no repair, so
// no check sends the operator to a sync that cannot carry it out. This pins
// the narrow form of the stop that design's Risks names: such a Codex is not
// stopped.
func TestACodexThatNeverProducedATableIsNoLoop(t *testing.T) {
	fixture := newCodexProgram(t, caskLayout)
	if err := os.Remove(filepath.Join(filepath.Dir(fixture.program), "models.json")); err != nil {
		t.Fatal(err)
	}
	var first []byte
	for round := range 2 {
		if _, err := fixture.sync(t); err != nil {
			t.Fatalf("sync %d = %v, want no stop", round+1, err)
		}
		if state := readCodexSidecar(t, fixture.home); state.CatalogHash != "" || state.CatalogState != "" {
			t.Fatalf("sync %d recorded a table: %+v", round+1, state)
		}
		if _, err := os.Stat(codexCatalogPath(fixture.home)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("sync %d wrote a table: %v", round+1, err)
		}
		config, err := os.ReadFile(fixture.home)
		if err != nil {
			t.Fatal(err)
		}
		if round == 1 && !slices.Equal(config, first) {
			t.Fatal("the second sync changed Codex's file")
		}
		first = config
		if status := fixture.inspect(t, fixture.cfg); !status.Ready || status.RepairAction != "" {
			t.Fatalf("check %d = %+v, want a pass that names no repair", round+1, status)
		}
	}
}

// A record that v0.10.0 marked stale, after it withdrew a table that it could
// not produce again, holds no table, so AIGW owns none: a check names the
// stale mark with a sync as its fix, the sync succeeds with a Codex that
// prints no table and clears the mark, the check then passes, and a second
// sync writes nothing.
func TestAStaleRecordOfAnEarlierReleaseIsClearedBySync(t *testing.T) {
	fixture := newCodexProgram(t, caskLayout)
	if err := os.Remove(filepath.Join(filepath.Dir(fixture.program), "models.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.sync(t); err != nil {
		t.Fatal(err)
	}
	record := codexRecordPath(fixture.home)
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	fields["catalog_state"], fields["catalog_client_version"], fields["catalog_client_sha256"] = "stale", "codex-cli 0.9.0", "aaaa"
	stale, err := adapter.EncodeJSON(fields)
	if err != nil {
		t.Fatal(err)
	}
	writeTo(t, record, string(stale))
	if status := fixture.inspect(t, fixture.cfg); status.Ready || !strings.HasPrefix(status.Issue, "Codex's model table is stale: ") || status.RepairAction != "aigw sync" {
		t.Fatalf("check of the stale record = %+v, want the stale mark and a sync", status)
	}
	if _, err := fixture.sync(t); err != nil {
		t.Fatalf("sync of the stale record = %v, want a success", err)
	}
	if state := readCodexSidecar(t, fixture.home); state.CatalogState != "" || state.CatalogHash != "" || state.CatalogClientVersion != "" {
		t.Fatalf("the sync kept the stale mark: %+v", state)
	}
	if status := fixture.inspect(t, fixture.cfg); !status.Ready {
		t.Fatalf("check after the sync = %+v, want ready", status)
	}
	before := [][]byte{}
	for _, path := range []string{fixture.home, record} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before = append(before, data)
	}
	if _, err := fixture.sync(t); err != nil {
		t.Fatal(err)
	}
	for index, path := range []string{fixture.home, record} {
		if data, err := os.ReadFile(path); err != nil || !slices.Equal(data, before[index]) {
			t.Fatalf("the second sync changed %s: %v", path, err)
		}
	}
}

// The limit that design's Risks names: a launcher whose path, digest and
// package manifest stay as they were while the program that it starts is
// gone. Each sync stops Codex, keeping its files, while a local check runs no
// program, since the producer agrees with the record, and passes; only
// `aigw check --depth client` runs the program.
func TestAnUnchangedLauncherWhoseProgramIsGoneStopsSyncWhileCheckPasses(t *testing.T) {
	fixture := newCodexProgram(t, caskLayout)
	target := filepath.Join(filepath.Dir(fixture.program), "real-codex")
	processtest.Write(t, target, fakeCodexScript)
	processtest.Write(t, fixture.program, "#!/bin/sh\nexec \"$(dirname \"$0\")/real-codex\" \"$@\"\n")
	if _, err := fixture.sync(t); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	before := fixture.files(t)
	for attempt := range 2 {
		_, err := fixture.sync(t)
		if stop, stopped := errors.AsType[*adapter.StopError](err); !stopped || stop.NextAction() != tableFix {
			t.Fatalf("sync %d = %v, want Codex stopped for its table", attempt+2, err)
		}
		if after := fixture.files(t); !slices.EqualFunc(after, before, slices.Equal) {
			t.Fatalf("sync %d changed Codex's files", attempt+2)
		}
		if status := fixture.inspect(t, fixture.cfg); !status.Ready {
			t.Fatalf("check %d = %+v, want the pass that runs no program", attempt+2, status)
		}
	}
}

// A file whose settings are not what AIGW writes for the selected model is
// still reported as not matching it, with a sync as its fix.
func TestACheckSaysThatAFileDoesNotMatchTheSelectedModel(t *testing.T) {
	fixture := newCodexProgram(t, caskLayout)
	if _, err := fixture.sync(t); err != nil {
		t.Fatal(err)
	}
	moved := fixture.cfg.Clone()
	provider := moved.Providers["gateway"]
	provider.Endpoints.Responses = "https://moved.gateway.test/v1"
	moved.Providers["gateway"] = provider
	status := fixture.inspect(t, moved)
	if status.Ready || status.Issue != driftIssue+"Codex configuration target does not match the selected model" || status.RepairAction != "aigw sync" {
		t.Fatalf("status = %+v, want a file that does not match the selected model", status)
	}
}

func appendTo(t *testing.T, path, text string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	processtest.Write(t, path, string(data)+text)
}

func writeTo(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}
