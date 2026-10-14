package construction

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// spdxGraph holds the identities and relationships that bind release programs
// to the Go modules Syft found in them.
type spdxGraph struct {
	Files []struct {
		ID   string `json:"SPDXID"`
		Name string `json:"fileName"`
	} `json:"files"`
	Packages      []spdxPackage `json:"packages"`
	Relationships []struct {
		Element string `json:"spdxElementId"`
		Type    string `json:"relationshipType"`
		Related string `json:"relatedSpdxElement"`
		Comment string `json:"comment"`
	} `json:"relationships"`
}

type spdxPackage struct {
	ID           string          `json:"SPDXID"`
	Name         string          `json:"name"`
	Version      string          `json:"versionInfo"`
	ExternalRefs []spdxReference `json:"externalRefs"`
}

type spdxReference struct {
	Type    string `json:"referenceType"`
	Locator string `json:"referenceLocator"`
}

// isModule reports whether the package names the Go module and its package URL.
func (item spdxPackage) isModule(module string) bool {
	return item.Name == module && slices.Contains(item.ExternalRefs, spdxReference{Type: "purl", Locator: "pkg:golang/" + module})
}

// mainModulePath reads the module that go.mod declares for the release programs.
func mainModulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for line := range strings.Lines(string(data)) {
		if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`), nil
		}
	}
	return "", errors.New("go.mod declares no module path")
}

// scannedStandardLibrary returns the one Go standard library version that the
// dependency scan observed in go.mod.
func scannedStandardLibrary(licenses string) (string, error) {
	var evidence licenseEvidence
	if err := readJSON(licenses, &evidence); err != nil {
		return "", fmt.Errorf("read license evidence: %w", err)
	}
	var scanned []string
	for _, source := range evidence.Sources {
		for _, item := range source.Packages {
			if source.Lockfile == "go.mod" && item.Ecosystem == "Go" && item.Name == "stdlib" {
				scanned = append(scanned, item.Version)
			}
		}
	}
	if len(scanned) != 1 {
		return "", fmt.Errorf("dependency scan observed %d Go standard library versions, want 1; OSV needs ScanGoModVersion", len(scanned))
	}
	return scanned[0], nil
}

// edges indexes the evident-by files and DEPENDENCY_OF targets of each element.
func (graph spdxGraph) edges() (evidentBy, dependencyOf map[string][]string) {
	evidentBy, dependencyOf = map[string][]string{}, map[string][]string{}
	for _, relation := range graph.Relationships {
		switch {
		case relation.Type == "OTHER" && strings.HasPrefix(relation.Comment, "evident-by:"):
			evidentBy[relation.Element] = append(evidentBy[relation.Element], relation.Related)
		case relation.Type == "DEPENDENCY_OF":
			dependencyOf[relation.Element] = append(dependencyOf[relation.Element], relation.Related)
		}
	}
	return evidentBy, dependencyOf
}

// mainModules maps each program to the one package of the declared main module
// that the program is evidence for.
func (graph spdxGraph) mainModules(module string, evidentBy map[string][]string) (map[string]string, error) {
	mainOf := map[string]string{}
	for _, item := range graph.Packages {
		if !item.isModule(module) {
			continue
		}
		programs := evidentBy[item.ID]
		if len(programs) != 1 || mainOf[programs[0]] != "" {
			return nil, fmt.Errorf("release SBOM main module %s is not evident by exactly one program", item.ID)
		}
		mainOf[programs[0]] = item.ID
	}
	return mainOf, nil
}

// bindStandardLibrary requires each release program in the SBOM to carry
// exactly one package of the main module that go.mod declares and exactly one
// Go standard library, at the version the dependency scan observed in go.mod,
// as a dependency of that same program's main module.
func bindStandardLibrary(licenses, sbom, module string) error {
	scanned, err := scannedStandardLibrary(licenses)
	if err != nil {
		return err
	}
	var graph spdxGraph
	if err := readJSON(sbom, &graph); err != nil {
		return fmt.Errorf("read release SBOM: %w", err)
	}
	evidentBy, dependencyOf := graph.edges()
	mainOf, err := graph.mainModules(module, evidentBy)
	if err != nil {
		return err
	}
	libraries := map[string]int{}
	for _, item := range graph.Packages {
		if item.Name != "stdlib" {
			continue
		}
		programs, modules := evidentBy[item.ID], dependencyOf[item.ID]
		if item.Version != "go"+scanned || len(programs) != 1 || len(modules) != 1 || modules[0] != mainOf[programs[0]] {
			return fmt.Errorf("release SBOM standard library %s %s is not Go %s of exactly one program's %s main module", item.ID, item.Version, scanned, module)
		}
		libraries[programs[0]]++
	}
	for _, program := range graph.Files {
		if mainOf[program.ID] == "" || libraries[program.ID] != 1 {
			return fmt.Errorf("release SBOM program %s carries %d Go standard library packages and main module %q, want 1 and %s", program.Name, libraries[program.ID], mainOf[program.ID], module)
		}
		delete(libraries, program.ID)
		delete(mainOf, program.ID)
	}
	if len(libraries) != 0 || len(mainOf) != 0 {
		return errors.New("release SBOM attributes a main module or Go standard library to a file outside the release programs")
	}
	return nil
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(data, value)
	}
	return err
}
