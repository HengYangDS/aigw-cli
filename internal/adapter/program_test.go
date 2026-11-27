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

// One rule decides a client's program: the declared one whenever one is
// declared, gone or not, and the discovered one only when none is.
func TestAClientsProgramIsTheDeclaredOneWheneverOneIsDeclared(t *testing.T) {
	discovered := discovery.Result{Executables: map[string]string{"codex": "/discovered/codex"}}
	gone := filepath.Join(t.TempDir(), "gone")
	for name, test := range map[string]struct {
		declared, want string
		adopts         bool
	}{
		"a declared program":              {declared: "/declared/codex", want: "/declared/codex"},
		"a declared program that is gone": {declared: gone, want: gone},
		"no declared program":             {want: "/discovered/codex", adopts: true},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := domain.NewConfig(clients.Admission())
			cfg.DeclareProgram("codex", test.declared, "")
			if got, _ := adapter.ClientProgram(cfg, "codex", discovered, discovery.Host{}); got.Path != test.want || got.Discovered != test.adopts {
				t.Fatalf("ClientProgram() = %+v, want %q", got, test.want)
			}
		})
	}
}

// Every client stops when its declared program is missing: convergence
// keeps the declared path and adopts nothing, the registry plans nothing for
// the client, and its inspection names the missing program with the one fix
// that names another, before its files are read. A disabled client and a
// client without a declared program are not stopped.
func TestAMissingDeclaredProgramStopsEveryClient(t *testing.T) {
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
			binding.Program = gone
			cfg.Clients[id] = binding

			after, err := registry.Converge(deps, cfg, discovered, id)
			if err != nil || after.Clients[id].Program != gone {
				t.Fatalf("Converge() kept %q, %v; want the declared path", after.Clients[id].Program, err)
			}
			missing, stopped := errors.AsType[*adapter.StopError](func() error {
				_, err := registry.Plan(deps, cfg, after, id)
				return err
			}())
			spec, _ := registry.Admission().Spec(id)
			fix := adapter.ProgramRepair(spec)
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
			if _, stops, err := registry.PlanStopping(deps, cfg, after, id); err != nil || len(stops) != 1 || stops[0].Client() != id {
				t.Fatalf("PlanStopping() = %v, %v; want the client stopped", stops, err)
			}
			// Withdrawing files needs no program, so it never stops.
			withdrawing := deps
			withdrawing.WithdrawOnly = []string{id}
			if _, stops, err := registry.PlanStopping(withdrawing, cfg, after, id); err != nil || len(stops) != 0 {
				t.Fatalf("PlanStopping() of a withdrawal = %v, %v; want no stop", stops, err)
			}
			disabled := cfg.Clone()
			binding.Enabled = false
			disabled.Clients[id] = binding
			if _, stops, err := registry.PlanStopping(deps, disabled, disabled, id); err != nil || len(stops) != 0 {
				t.Fatalf("PlanStopping() of a disabled client = %v, %v; want no stop", stops, err)
			}
			undeclared := cfg.Clone()
			binding.Enabled, binding.Program = true, ""
			undeclared.Clients[id] = binding
			if missing := adapter.MissingProgram(undeclared, id, deps.Host); missing != nil {
				t.Fatalf("MissingProgram() = %v for a client that declares none", missing)
			}
		})
	}
}

// The missing-program repair names --program for every client, and
// --extensions beside it for a client whose program an editor extension
// ships.
func TestTheProgramRepairNamesExtensionsWhereAnExtensionShipsTheProgram(t *testing.T) {
	for id, want := range map[string]string{
		"codex":          "aigw client edit codex --program <path> or --extensions <dir>",
		"claude":         "aigw client edit claude --program <path> or --extensions <dir>",
		"claude-desktop": "aigw client edit claude-desktop --program <path>",
		"hermes":         "aigw client edit hermes --program <path>",
	} {
		spec, _ := clients.Admission().Spec(id)
		if got := adapter.ProgramRepair(spec); got != want {
			t.Errorf("ProgramRepair(%s) = %q, want %q", id, got, want)
		}
	}
}

// An enabled client that declares no program and of which discovery finds
// none is not installed: the registry plans nothing for it and stops it
// neither, so that its files wait for it, as for a client not installed yet.
func TestAnUndeclaredClientThatDiscoveryDoesNotFindWaits(t *testing.T) {
	registry := clients.Registry()
	for _, id := range registry.IDs() {
		t.Run(id, func(t *testing.T) {
			cfg, deps, _ := configuredClient(t, id)
			binding := cfg.Clients[id]
			binding.Program = ""
			cfg.Clients[id] = binding
			deps.Discovery = fixedDiscovery{}
			if plans, stops, err := registry.PlanStopping(deps, cfg, cfg, id); err != nil || len(stops) != 0 || len(plans) != 0 {
				t.Fatalf("PlanStopping() = %v, %v, %v; want no plan and no stop", plans, stops, err)
			}
			if stop := adapter.ProgramNotFound(cfg, id); stop.Client() != id || stop.Error() != cfg.Admission().ProductName(id)+" program was not found" {
				t.Fatalf("ProgramNotFound() = %v", stop)
			}
		})
	}
}

// A declared program that is there is not missing, and one that cannot be
// inspected is, with the failure kept.
func TestAProgramThatCannotBeInspectedIsMissing(t *testing.T) {
	cfg, deps, _ := configuredClient(t, "codex")
	if missing := adapter.MissingProgram(cfg, "codex", deps.Host); missing != nil {
		t.Fatalf("MissingProgram() = %v for a program that runs", missing)
	}
	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Fatal(err)
	}
	binding := cfg.Clients["codex"]
	binding.Program = loop
	cfg.Clients["codex"] = binding
	missing := adapter.MissingProgram(cfg, "codex", deps.Host)
	if missing == nil || missing.Error() != "Codex program cannot be inspected" || errors.Unwrap(missing) == nil {
		t.Fatalf("MissingProgram() = %v, want the inspection failure", missing)
	}
	if missing.NextAction() != "aigw client edit codex --program <path> or --extensions <dir>" {
		t.Fatalf("NextAction() = %q", missing.NextAction())
	}
}
