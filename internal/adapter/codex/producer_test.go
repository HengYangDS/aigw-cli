package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// runnableProgram writes a runnable file that stands for Codex's configured
// executable, which fails every command.
func runnableProgram(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

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

// codexProgram is a Codex whose program is fakeCodexScript in an editor
// extension's directory, bin/<os>-<arch>, beside its package manifest,
// selected with gateway/gpt-6.2-sol for Codex's default home, and the
// dependencies of a synchronization at the adapter layer.
type codexProgram struct {
	program, extension, home string
	cfg                      domain.Config
	deps                     adapter.Dependencies
}

func newCodexProgram(t *testing.T) codexProgram {
	t.Helper()
	root := t.TempDir()
	extension := filepath.Join(root, ".vscode-server", "extensions", "openai.chatgpt-1.0.0-linux-arm64")
	directory := filepath.Join(extension, "bin", "linux-aarch64")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(directory, "codex")
	for name, content := range map[string]string{"models.json": windowedTable, "codex-package.json": `{"layoutVersion":1,"version":"1.0.0"}`} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(program, []byte(fakeCodexScript), 0o755); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(home, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
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
	return codexProgram{program: program, extension: extension, home: home, cfg: cfg, deps: deps}
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

// A sync records the program that produced Codex's model table, and a local
// check compares the configured program with it, by path, by digest and by
// the version of its package manifest, without running it; each difference
// fails the check with a sync as its fix. Without a manifest the version is
// not compared, and a record that names no producer fails too.
func TestACheckComparesTheTablesProducerWithTheConfiguredProgram(t *testing.T) {
	for name, test := range map[string]struct {
		change  func(t *testing.T, fixture codexProgram, cfg *domain.Config)
		problem string
	}{
		"nothing changed": {change: func(*testing.T, codexProgram, *domain.Config) {}},
		"the program changed in place": {
			change: func(t *testing.T, fixture codexProgram, _ *domain.Config) {
				appendTo(t, fixture.program, "# another build\n")
			},
			problem: "Codex's configured executable changed after it produced Codex's model table",
		},
		"another program is configured": {
			change: func(t *testing.T, fixture codexProgram, cfg *domain.Config) {
				binding := cfg.Clients[ID]
				binding.Executable = runnableProgram(t)
				cfg.Clients[ID] = binding
			},
			problem: "Codex's model table came from another executable than the configured one",
		},
		"the manifest declares another version": {
			change: func(t *testing.T, fixture codexProgram, _ *domain.Config) {
				writeTo(t, filepath.Join(filepath.Dir(fixture.program), "codex-package.json"), `{"version":"1.1.0"}`)
			},
			problem: "Codex's configured executable is now version 1.1.0, and Codex's model table came from codex-cli 1.0.0",
		},
		"no manifest is readable": {
			change: func(t *testing.T, fixture codexProgram, _ *domain.Config) {
				if err := os.Remove(filepath.Join(filepath.Dir(fixture.program), "codex-package.json")); err != nil {
					t.Fatal(err)
				}
			},
		},
		"the record names no producer": {
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
			fixture := newCodexProgram(t)
			if _, err := fixture.sync(t); err != nil {
				t.Fatal(err)
			}
			state := readCodexSidecar(t, fixture.home)
			if state.CatalogHash == "" || state.CatalogClientPath != fixture.program || state.CatalogClientVersion != "codex-cli 1.0.0" || state.CatalogClientSHA256 == "" {
				t.Fatalf("the record names the producer as %q %q %q", state.CatalogClientPath, state.CatalogClientVersion, state.CatalogClientSHA256)
			}
			cfg := fixture.cfg.Clone()
			test.change(t, fixture, &cfg)
			runs := filepath.Join(filepath.Dir(fixture.program), "runs")
			if err := os.Remove(runs); err != nil {
				t.Fatal(err)
			}
			status := fixture.inspect(t, cfg)
			if _, err := os.Stat(runs); !os.IsNotExist(err) {
				t.Fatalf("the check ran the program: %v", err)
			}
			if test.problem == "" {
				if !status.Ready {
					t.Fatalf("status = %+v, want ready", status)
				}
				return
			}
			if status.Ready || status.Issue != test.problem || status.RepairAction != "aigw sync" {
				t.Fatalf("status = %+v, want %q with aigw sync", status, test.problem)
			}
		})
	}
}

// A table that AIGW withdrew because the configured program could not produce
// one is named as stale, not as a file that does not match the selected model.
func TestACheckNamesAStaleTable(t *testing.T) {
	fixture := newCodexProgram(t)
	if _, err := fixture.sync(t); err != nil {
		t.Fatal(err)
	}
	// The program is still there and changed, but prints no table.
	if err := os.Remove(filepath.Join(filepath.Dir(fixture.program), "models.json")); err != nil {
		t.Fatal(err)
	}
	appendTo(t, fixture.program, "# a build that prints no table\n")
	if _, err := fixture.sync(t); err != nil {
		t.Fatal(err)
	}
	if state := readCodexSidecar(t, fixture.home); state.CatalogState != catalogStateStale {
		t.Fatalf("the record's table is %q, want stale", state.CatalogState)
	}
	status := fixture.inspect(t, fixture.cfg)
	if status.Ready || !strings.HasPrefix(status.Issue, "Codex's model table is stale: ") || status.RepairAction != "aigw sync" {
		t.Fatalf("status = %+v, want a stale table", status)
	}
}

// A file whose settings are not what AIGW writes for the selected model is
// still reported as not matching it, with a sync as its fix.
func TestACheckSaysThatAFileDoesNotMatchTheSelectedModel(t *testing.T) {
	fixture := newCodexProgram(t)
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
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(text); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeTo(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
