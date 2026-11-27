package activation

import (
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	domainreadiness "aigw-cli/internal/app/readiness"
	builtinclients "aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// An enabled client that declares its program is judged by its inspection
// alone, files that match AIGW's record included, so is a client whose
// inspection names a step of its own, and a disabled client waits for
// nothing. A client that declares none waits for the step that gives it a
// program: a sync, which adopts the one that discovery finds, or else the
// missing program's step, whether its files match AIGW's record or wait for a
// sync to write them. Discovery is asked only for such a client.
func TestAwaitedFilesFollowTheRecordAndNameTheProgramStep(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Responses: "https://team.test/v1"}}
	cfg.Models[domain.NewModelRef("team", "model")] = domain.Model{Provider: "team", Family: "model", ID: "model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("team", "model"), "")
	cfg.SetClientActivation(codex.ID, true, "", []string{"/home/user/.codex/config.toml"})
	declared, disabled := cfg.Clone(), cfg.Clone()
	declared.DeclareProgram(codex.ID, "", "/home/user/.vscode/extensions")
	disabled.SetClientActivation(codex.ID, false, "", nil)
	ready, unwritten := adapter.Status{Ready: true}, adapter.Status{Issue: "Codex configuration target is missing", RepairAction: "aigw sync"}
	edited := adapter.Status{Issue: "Codex config conflict", RepairAction: "Undo the edit"}
	missing := adapter.ProgramNotFound(cfg, codex.ID)
	for name, test := range map[string]struct {
		cfg       domain.Config
		inspected adapter.Status
		found     bool
		want      Awaited
	}{
		"a declared program whose files match":  {declared, ready, false, Awaited{}},
		"a declared program whose files wait":   {declared, unwritten, false, Awaited{}},
		"a disabled client":                     {disabled, unwritten, true, Awaited{}},
		"an edit outside AIGW":                  {cfg, edited, true, Awaited{}},
		"files that match, a program found":     {cfg, ready, true, Awaited{Problem: "Codex declares no program, and a sync adopts the one that discovery finds" + recorded, Step: "aigw sync", Found: true}},
		"files that match, no program found":    {cfg, ready, false, Awaited{Problem: missing.Error() + recorded, Step: missing.NextAction()}},
		"files AIGW has not written, found":     {cfg, unwritten, true, Awaited{Problem: "Codex is installed, but AIGW has not written its files yet", Step: "aigw sync", Found: true, Unwritten: true}},
		"files AIGW has not written, not found": {cfg, unwritten, false, Awaited{Problem: missing.Error(), Step: missing.NextAction(), Unwritten: true}},
	} {
		t.Run(name, func(t *testing.T) {
			asked := false
			got := AwaitedFiles(test.cfg, codex.ID, test.inspected, func() bool { asked = true; return test.found })
			if got != test.want {
				t.Fatalf("AwaitedFiles() = %+v, want %+v", got, test.want)
			}
			if asked != (test.want.Step != "") {
				t.Fatalf("discovery asked = %v for %+v", asked, got)
			}
		})
	}
}

// What an observation of the enabled clients found replaces the estimate that
// the configuration gives: a client whose files match AIGW's record and that
// waits for its program keeps the activation from being deferred, and names
// its step, and a client that waits for nothing leaves the step to verify.
func TestObservedFilesReplaceTheConfigurationsEstimate(t *testing.T) {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Models[domain.NewModelRef("team", "model")] = domain.Model{Provider: "team", Family: "model", ID: "model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedModel(hermes.ID, domain.NewModelRef("team", "model"), "")
	cfg.SetClientActivation(hermes.ID, true, "", nil)
	estimated := AssessActivation(cfg, selectedMetadataStore{})
	if estimated.State != domainreadiness.Deferred || estimated.ProjectionPrerequisites[hermes.ID] == "" {
		t.Fatalf("the configuration's estimate = %+v", estimated)
	}
	adoption := estimated
	adoption.ObserveFiles(map[string]Awaited{hermes.ID: {Problem: "Hermes declares no program", Step: "aigw sync", Found: true}})
	if adoption.State != "" || adoption.NextActionFor(nil) != "aigw sync" || adoption.VerificationPrerequisite != "" {
		t.Fatalf("a client whose files match its record = %+v", adoption)
	}
	waiting := estimated
	waiting.ObserveFiles(map[string]Awaited{hermes.ID: {Problem: "Hermes is installed", Step: "aigw sync", Found: true, Unwritten: true}})
	if waiting.State != domainreadiness.Deferred || waiting.NextActionFor(nil) != "aigw sync" {
		t.Fatalf("a client whose files wait = %+v", waiting)
	}
	settled := estimated
	settled.ObserveFiles(map[string]Awaited{hermes.ID: {}})
	if settled.State != "" || len(settled.ProjectionPrerequisites) != 0 || settled.NextActionFor(nil) != "aigw check" {
		t.Fatalf("a client that waits for nothing = %+v", settled)
	}
}
