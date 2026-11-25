package adapter_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// One rule decides a client's program: the configured one whenever one is
// configured, gone or not, and the discovered one only when none is.
func TestAClientsProgramIsTheConfiguredOneWheneverOneIsConfigured(t *testing.T) {
	discovered := discovery.Result{Executables: map[string]string{"codex": "/discovered/codex"}}
	gone := filepath.Join(t.TempDir(), "gone")
	for name, test := range map[string]struct{ configured, want string }{
		"a configured program":              {configured: "/configured/codex", want: "/configured/codex"},
		"a configured program that is gone": {configured: gone, want: gone},
		"no configured program":             {want: "/discovered/codex"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := adapter.ClientProgram(domain.ClientBinding{Executable: test.configured}, discovered, "codex"); got != test.want {
				t.Fatalf("ClientProgram() = %q, want %q", got, test.want)
			}
		})
	}
}

// Every client stops when its configured program is missing: convergence
// keeps the configured path and adopts nothing, the registry plans nothing
// for the client, and its inspection names the missing program with the one
// fix that names another, before its files are read. A disabled client and a
// client without a configured program are not stopped.
func TestAMissingConfiguredProgramStopsEveryClient(t *testing.T) {
	registry := clients.Registry()
	for _, id := range registry.IDs() {
		t.Run(id, func(t *testing.T) {
			cfg, deps, discovered := configuredClient(t, id)
			tokens := secret.NewMemoryStore()
			if err := tokens.Set("gateway", "public-token"); err != nil {
				t.Fatal(err)
			}
			deps.Secrets = tokens
			gone := filepath.Join(t.TempDir(), "gone")
			binding := cfg.Clients[id]
			binding.Executable = gone
			cfg.Clients[id] = binding

			after, err := registry.Converge(deps, cfg, discovered, id)
			if err != nil || after.Clients[id].Executable != gone {
				t.Fatalf("Converge() kept %q, %v; want the configured path", after.Clients[id].Executable, err)
			}
			missing, stopped := errors.AsType[*adapter.MissingProgramError](func() error {
				_, err := registry.Plan(deps, cfg, after, id)
				return err
			}())
			fix := "aigw client edit " + id + " --executable <path>"
			if !stopped || missing.Client() != id || missing.NextAction() != fix {
				t.Fatalf("Plan() refused %v, want the missing program", missing)
			}
			runtime, err := cfg.ResolveRuntime(id, domain.ModelRef{})
			if err != nil {
				t.Fatal(err)
			}
			if status := registry.Inspect(context.Background(), deps, cfg, id, runtime); status.Ready || status.Issue != missing.Error() || status.RepairAction != fix {
				t.Fatalf("Inspect() = %+v, want the missing program and its fix", status)
			}
			if got := registry.MissingPrograms(cfg); len(got) != 1 || got[0].Client() != id {
				t.Fatalf("MissingPrograms() = %v", got)
			}

			disabled := cfg.Clone()
			binding.Enabled = false
			disabled.Clients[id] = binding
			unconfigured := cfg.Clone()
			binding.Enabled, binding.Executable = true, ""
			unconfigured.Clients[id] = binding
			for _, other := range []domain.Config{disabled, unconfigured} {
				if got := registry.MissingPrograms(other); len(got) != 0 {
					t.Fatalf("MissingPrograms() = %v, want none", got)
				}
			}
		})
	}
}

// A configured program that is there is not missing, and one that cannot be
// inspected is, with the failure kept.
func TestAProgramThatCannotBeInspectedIsMissing(t *testing.T) {
	cfg, _, _ := configuredClient(t, "codex")
	if missing := adapter.MissingProgram(cfg, "codex"); missing != nil {
		t.Fatalf("MissingProgram() = %v for a program that runs", missing)
	}
	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Skipf("symbolic link unavailable: %v", err)
	}
	binding := cfg.Clients["codex"]
	binding.Executable = loop
	cfg.Clients["codex"] = binding
	missing := adapter.MissingProgram(cfg, "codex")
	if missing == nil || missing.Error() != "Codex executable cannot be inspected" || errors.Unwrap(missing) == nil {
		t.Fatalf("MissingProgram() = %v, want the inspection failure", missing)
	}
	if missing.NextAction() != "aigw client edit codex --executable <path>" {
		t.Fatalf("NextAction() = %q", missing.NextAction())
	}
}
