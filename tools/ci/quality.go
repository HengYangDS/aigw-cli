package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type qualityConcern string

const (
	qualityFormat        qualityConcern = "format"
	qualityLint          qualityConcern = "lint"
	qualityType          qualityConcern = "type"
	qualityTest          qualityConcern = "test"
	qualitySecurity      qualityConcern = "security"
	qualityArchitecture  qualityConcern = "architecture"
	qualityDocumentation qualityConcern = "documentation"
	qualitySchema        qualityConcern = "schema"
	qualityWorkflow      qualityConcern = "workflow"
	qualityProjection    qualityConcern = "projection"
)

var requiredRepositoryConcerns = []qualityConcern{
	qualityFormat,
	qualityLint,
	qualityType,
	qualityTest,
	qualitySecurity,
	qualityArchitecture,
	qualityDocumentation,
	qualitySchema,
	qualityWorkflow,
	qualityProjection,
}

type qualityGate struct {
	ID         string
	Command    command
	Concerns   []qualityConcern
	SourceOnly bool
}

type carrierQuality struct {
	Class    string
	Required []qualityConcern
	Gates    []string
}

type qualityGraph struct {
	Gates       []qualityGate
	CommonGates []string
	Carriers    []carrierQuality
}

var repositoryQualityGraph = qualityGraph{
	Gates: []qualityGate{
		{ID: "quality-coverage", Command: command{Name: "go", Args: []string{"run", "./tools/ci", "check-quality-coverage", "."}}, Concerns: []qualityConcern{qualityArchitecture}},
		{ID: "go-policy-schema", Command: command{Name: "golangci-lint", Args: []string{"config", "verify", "--config", ".config/checks/go/policy.yml"}}, Concerns: []qualityConcern{qualitySchema}},
		{ID: "release-policy-schema", Command: command{Name: "goreleaser", Args: []string{"check", ".config/release/goreleaser.yaml"}}, Concerns: []qualityConcern{qualitySchema}},
		{ID: "cue-format", Command: command{Name: "cue", Args: []string{"fmt", "--check", "--files", ".config/ci"}}, Concerns: []qualityConcern{qualityFormat}},
		{ID: "ci-projection", Command: command{Name: "go", Args: []string{"run", "./tools/ci", "project", "--check"}}, Concerns: []qualityConcern{qualitySchema, qualityWorkflow, qualityProjection}},
		{ID: "npm-signatures", Command: command{Name: "npm", Args: []string{"audit", "signatures"}}, Concerns: []qualityConcern{qualitySecurity}},
		{ID: "openspec", Command: command{Name: "go", Args: []string{"run", "./tools/ci", "openspec"}}, Concerns: []qualityConcern{qualityDocumentation, qualitySchema}},
		{ID: "text-layout", Command: command{Name: "editorconfig-checker", Args: []string{"-disable-indentation", "-disable-indent-size"}}, Concerns: []qualityConcern{qualityFormat}},
		{ID: "format", Command: command{Name: "go", Args: []string{"run", "./tools/ci", "check-format", "."}}, Concerns: []qualityConcern{qualityFormat}},
		{ID: "markdown", Command: command{Name: "go", Args: []string{"run", "./tools/ci", "check-markdown", "."}}, Concerns: []qualityConcern{qualityLint, qualityDocumentation}},
		{ID: "mermaid", Command: command{Name: "go", Args: []string{"run", "./tools/ci", "check-mermaid", "."}}, Concerns: []qualityConcern{qualityDocumentation, qualitySchema}},
		{ID: "links", Command: command{Name: "go", Args: []string{"run", "./tools/ci", "links", "."}}, Concerns: []qualityConcern{qualityDocumentation}},
		{ID: "toml", Command: command{Name: "go", Args: []string{"run", "./tools/ci", "check-toml", "."}}, Concerns: []qualityConcern{qualityFormat, qualitySchema}},
		{ID: "go-module-tidy", Command: command{Name: "go", Args: []string{"mod", "tidy", "-diff"}}, Concerns: []qualityConcern{qualityLint, qualitySchema}},
		{ID: "go-module-integrity", Command: command{Name: "go", Args: []string{"mod", "verify"}}, Concerns: []qualityConcern{qualitySecurity}},
		{ID: "vulnerabilities", Command: command{Name: "osv-scanner", Args: []string{"scan", "source", "--config", ".config/checks/dependencies/policy.toml", "--lockfile", "go.mod", "--lockfile", "package-lock.json", "--format", "table", "--verbosity", "warn", "."}}, Concerns: []qualityConcern{qualitySecurity}},
		{ID: "secrets", Command: command{Name: "go", Args: []string{"run", "./tools/ci", "check-secrets", "."}}, Concerns: []qualityConcern{qualitySecurity}},
		{ID: "toolchain", Command: command{Name: "go", Args: []string{"run", "./tools/release", "validate-toolchain", "go.mod"}}, Concerns: []qualityConcern{qualitySchema}},
		{ID: "release-sources", Command: command{Name: "go", Args: []string{"run", "./tools/release", "validate-release-sources"}}, Concerns: []qualityConcern{qualitySchema, qualityProjection}},
		{ID: "changelog", Command: command{Name: "go", Args: []string{"run", "./tools/release", "validate-changelog"}}, Concerns: []qualityConcern{qualityDocumentation, qualitySchema}},
		{ID: "architecture", Command: command{Name: "go", Args: []string{"run", "./tools/architecture", "--root", "."}}, Concerns: []qualityConcern{qualityArchitecture}},
		{ID: "source-size", Command: command{Name: "go", Args: []string{"run", "./tools/ci", "check-source-size", "."}}, Concerns: []qualityConcern{qualityArchitecture}},
		{ID: "go-analysis", Command: command{Name: "go", Args: []string{"run", "./tools/ci", "check-go", "."}}, Concerns: []qualityConcern{qualityFormat, qualityLint, qualityType, qualitySecurity}},
		{ID: "client-acceptance", Command: command{Name: "go", Args: []string{"test", "-tags=client_acceptance", "./tools/release", "-run", "^TestNativeClient(Inputs|StreamEnvelope|FilePreservation)$"}}, Concerns: []qualityConcern{qualityTest}},
		{ID: "performance-acceptance", Command: command{Name: "go", Args: []string{"test", "-tags=performance_acceptance", "./tools/release", "-run", "^TestNative(PeakMemoryBudget|Performance(Samples|Command|PooledSamples))$"}}, Concerns: []qualityConcern{qualityTest}},
		{ID: "workflow-lint", Command: command{Name: "actionlint"}, Concerns: []qualityConcern{qualitySchema, qualityWorkflow}},
		{ID: "coverage", Command: command{Name: "go", Args: []string{"run", "./tools/coverage", "--race"}}, Concerns: []qualityConcern{qualityTest}, SourceOnly: true},
	},
	CommonGates: []string{"quality-coverage", "text-layout", "secrets", "architecture"},
	Carriers: []carrierQuality{
		{Class: "go-source", Required: []qualityConcern{qualityFormat, qualityLint, qualityType, qualityTest, qualitySecurity, qualityArchitecture}, Gates: []string{"go-analysis", "source-size", "client-acceptance", "performance-acceptance", "coverage"}},
		{Class: "native-check-adapters", Required: []qualityConcern{qualityFormat, qualityTest, qualitySecurity, qualityArchitecture}, Gates: []string{"format", "npm-signatures", "coverage"}},
		{Class: "current-documentation", Required: []qualityConcern{qualityFormat, qualityLint, qualityDocumentation, qualitySecurity, qualityArchitecture}, Gates: []string{"format", "markdown", "mermaid", "links", "changelog"}},
		{Class: "active-openspec", Required: []qualityConcern{qualityFormat, qualityLint, qualityDocumentation, qualitySchema, qualitySecurity, qualityArchitecture}, Gates: []string{"format", "markdown", "mermaid", "links", "openspec"}},
		{Class: "archived-openspec", Required: []qualityConcern{qualityFormat, qualitySecurity, qualityArchitecture}},
		{Class: "ethos-governance", Required: []qualityConcern{qualityFormat, qualityTest, qualitySecurity, qualityArchitecture, qualitySchema, qualityProjection}, Gates: []string{"toml", "ci-projection", "coverage"}},
		{Class: "ci-authority", Required: []qualityConcern{qualityFormat, qualityTest, qualitySecurity, qualityArchitecture, qualitySchema, qualityWorkflow, qualityProjection}, Gates: []string{"cue-format", "ci-projection", "coverage"}},
		{Class: "ci-projection", Required: []qualityConcern{qualityFormat, qualityTest, qualitySecurity, qualityArchitecture, qualitySchema, qualityWorkflow, qualityProjection}, Gates: []string{"format", "ci-projection", "workflow-lint", "coverage"}},
		{Class: "quality-policy", Required: []qualityConcern{qualityFormat, qualityTest, qualitySecurity, qualityArchitecture, qualitySchema}, Gates: []string{"go-policy-schema", "format", "markdown", "toml", "vulnerabilities", "source-size", "go-analysis", "coverage"}},
		{Class: "release-policy", Required: []qualityConcern{qualityFormat, qualityTest, qualitySecurity, qualityArchitecture, qualitySchema, qualityProjection}, Gates: []string{"release-policy-schema", "format", "release-sources", "coverage"}},
		{Class: "dependency-policy", Required: []qualityConcern{qualityFormat, qualityTest, qualitySecurity, qualityArchitecture}, Gates: []string{"format", "coverage"}},
		{Class: "team-manifest", Required: []qualityConcern{qualityFormat, qualityTest, qualitySecurity, qualityArchitecture, qualitySchema}, Gates: []string{"toml", "coverage"}},
		{Class: "toolchain", Required: []qualityConcern{qualityFormat, qualityLint, qualityTest, qualitySecurity, qualityArchitecture, qualitySchema}, Gates: []string{"format", "toml", "npm-signatures", "go-module-tidy", "go-module-integrity", "vulnerabilities", "toolchain", "coverage"}},
		{Class: "repository-metadata", Required: []qualityConcern{qualityFormat, qualityTest, qualitySecurity, qualityArchitecture, qualitySchema}, Gates: []string{"format", "release-sources", "coverage"}},
	},
}

func (graph qualityGraph) commands(source bool) []command {
	commands := make([]command, 0, len(graph.Gates))
	for _, gate := range graph.Gates {
		if !source && gate.SourceOnly {
			continue
		}
		commands = append(commands, gate.Command)
	}
	return commands
}

func checkQualityCoverage(root string, _ commandRunner) error {
	classes, err := loadTrackedCarrierClasses(filepath.Join(root, ".config", "checks", "architecture", "policy.toml"))
	if err != nil {
		return err
	}
	return validateQualityGraph(repositoryQualityGraph, classes)
}

func loadTrackedCarrierClasses(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read architecture policy: %w", err)
	}
	var document struct {
		TrackedCarrierClasses map[string]map[string]any `toml:"tracked_carrier_classes"`
	}
	if err := toml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode architecture policy: %w", err)
	}
	classes := make([]string, 0, len(document.TrackedCarrierClasses))
	for name := range document.TrackedCarrierClasses {
		classes = append(classes, name)
	}
	slices.Sort(classes)
	if len(classes) == 0 {
		return nil, fmt.Errorf("architecture policy declares no tracked carrier classes")
	}
	return classes, nil
}

func validateQualityGraph(graph qualityGraph, classes []string) error {
	knownConcerns := qualityConcernSet(requiredRepositoryConcerns)
	knownGates, coveredConcerns, err := indexQualityGates(graph.Gates, knownConcerns)
	if err != nil {
		return err
	}
	declaredClasses, err := uniqueCarrierClasses(classes)
	if err != nil {
		return err
	}
	common, err := resolveQualityGates(graph.CommonGates, knownGates, "common")
	if err != nil {
		return err
	}
	consumers := make(map[string]int, len(graph.Gates))
	for id := range common {
		consumers[id] = len(classes)
	}
	coveredClasses, err := validateCarrierCoverage(graph.Carriers, declaredClasses, knownConcerns, knownGates, common, consumers)
	if err != nil {
		return err
	}
	if err := requireCoveredCarriers(declaredClasses, coveredClasses); err != nil {
		return err
	}
	if err := requireGateConsumers(graph.Gates, consumers); err != nil {
		return err
	}
	for _, concern := range requiredRepositoryConcerns {
		if !coveredConcerns[concern] {
			return fmt.Errorf("quality graph has no gate for repository concern %q", concern)
		}
	}
	return nil
}

func qualityConcernSet(concerns []qualityConcern) map[qualityConcern]bool {
	set := make(map[qualityConcern]bool, len(concerns))
	for _, concern := range concerns {
		set[concern] = true
	}
	return set
}

func indexQualityGates(gates []qualityGate, knownConcerns map[qualityConcern]bool) (map[string]qualityGate, map[qualityConcern]bool, error) {
	indexed := make(map[string]qualityGate, len(gates))
	covered := make(map[qualityConcern]bool, len(knownConcerns))
	for _, gate := range gates {
		if strings.TrimSpace(gate.ID) == "" || strings.TrimSpace(gate.Command.Name) == "" {
			return nil, nil, fmt.Errorf("quality gates require non-empty identities and commands")
		}
		if _, exists := indexed[gate.ID]; exists {
			return nil, nil, fmt.Errorf("duplicate quality gate %q", gate.ID)
		}
		if len(gate.Concerns) == 0 {
			return nil, nil, fmt.Errorf("quality gate %q declares no concern", gate.ID)
		}
		seen := make(map[qualityConcern]bool, len(gate.Concerns))
		for _, concern := range gate.Concerns {
			if !knownConcerns[concern] {
				return nil, nil, fmt.Errorf("quality gate %q declares unknown concern %q", gate.ID, concern)
			}
			if seen[concern] {
				return nil, nil, fmt.Errorf("quality gate %q duplicates concern %q", gate.ID, concern)
			}
			seen[concern] = true
			covered[concern] = true
		}
		indexed[gate.ID] = gate
	}
	return indexed, covered, nil
}

func uniqueCarrierClasses(classes []string) (map[string]bool, error) {
	declared := make(map[string]bool, len(classes))
	for _, class := range classes {
		if strings.TrimSpace(class) == "" || declared[class] {
			return nil, fmt.Errorf("architecture policy has an empty or duplicate carrier class %q", class)
		}
		declared[class] = true
	}
	return declared, nil
}

func validateCarrierCoverage(
	carriers []carrierQuality,
	declaredClasses map[string]bool,
	knownConcerns map[qualityConcern]bool,
	knownGates map[string]qualityGate,
	common map[string]qualityGate,
	consumers map[string]int,
) (map[string]bool, error) {
	covered := make(map[string]bool, len(carriers))
	for _, carrier := range carriers {
		if covered[carrier.Class] {
			return nil, fmt.Errorf("duplicate carrier quality coverage %q", carrier.Class)
		}
		covered[carrier.Class] = true
		if !declaredClasses[carrier.Class] {
			return nil, fmt.Errorf("unknown carrier quality coverage %q", carrier.Class)
		}
		if len(carrier.Required) == 0 {
			return nil, fmt.Errorf("carrier quality coverage %q declares no required concern", carrier.Class)
		}
		selected, err := resolveQualityGates(carrier.Gates, knownGates, carrier.Class)
		if err != nil {
			return nil, err
		}
		for id := range selected {
			consumers[id]++
		}
		maps.Copy(selected, common)
		if err := requireCarrierConcerns(carrier, selected, knownConcerns); err != nil {
			return nil, err
		}
	}
	return covered, nil
}

func requireCarrierConcerns(carrier carrierQuality, gates map[string]qualityGate, known map[qualityConcern]bool) error {
	available := make(map[qualityConcern]bool, len(known))
	for _, gate := range gates {
		for _, concern := range gate.Concerns {
			available[concern] = true
		}
	}
	required := make(map[qualityConcern]bool, len(carrier.Required))
	for _, concern := range carrier.Required {
		if !known[concern] {
			return fmt.Errorf("carrier quality coverage %q declares unknown concern %q", carrier.Class, concern)
		}
		if required[concern] {
			return fmt.Errorf("carrier quality coverage %q duplicates concern %q", carrier.Class, concern)
		}
		required[concern] = true
		if !available[concern] {
			return fmt.Errorf("carrier quality coverage %q is missing required quality concern %q", carrier.Class, concern)
		}
	}
	return nil
}

func requireCoveredCarriers(declared, covered map[string]bool) error {
	missing := make([]string, 0)
	for class := range declared {
		if !covered[class] {
			missing = append(missing, class)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	slices.Sort(missing)
	return fmt.Errorf("missing carrier quality coverage: %s", strings.Join(missing, ", "))
}

func requireGateConsumers(gates []qualityGate, consumers map[string]int) error {
	for _, gate := range gates {
		if consumers[gate.ID] == 0 {
			return fmt.Errorf("quality gate has no carrier consumer: %s", gate.ID)
		}
	}
	return nil
}

func resolveQualityGates(ids []string, gates map[string]qualityGate, owner string) (map[string]qualityGate, error) {
	selected := make(map[string]qualityGate, len(ids))
	for _, id := range ids {
		if _, duplicate := selected[id]; duplicate {
			return nil, fmt.Errorf("quality coverage %q duplicates gate %q", owner, id)
		}
		gate, exists := gates[id]
		if !exists {
			return nil, fmt.Errorf("quality coverage %q references unknown quality gate %q", owner, id)
		}
		selected[id] = gate
	}
	return selected, nil
}
