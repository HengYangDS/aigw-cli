package construction

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type dependencyIdentity struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
	Version   string `json:"version"`
}

type osvReport struct {
	Results []struct {
		Source struct {
			Path string `json:"path"`
		} `json:"source"`
		Packages []struct {
			Package         dependencyIdentity `json:"package"`
			Licenses        []string           `json:"licenses"`
			Vulnerabilities []osvVulnerability `json:"vulnerabilities"`
		} `json:"packages"`
	} `json:"results"`
}

type osvVulnerability struct {
	ID       string   `json:"id"`
	Aliases  []string `json:"aliases"`
	Summary  string   `json:"summary"`
	Affected []struct {
		Package struct {
			Ecosystem string `json:"ecosystem"`
			Name      string `json:"name"`
		} `json:"package"`
		Ranges []struct {
			Type   string `json:"type"`
			Events []struct {
				Fixed string `json:"fixed"`
			} `json:"events"`
		} `json:"ranges"`
	} `json:"affected"`
}

type vulnerabilityEvidence struct {
	SchemaVersion int                   `json:"schema_version"`
	Sources       []vulnerabilitySource `json:"sources"`
}

type vulnerabilitySource struct {
	Lockfile string                 `json:"lockfile"`
	Packages []vulnerableDependency `json:"packages"`
}

type vulnerableDependency struct {
	dependencyIdentity
	Vulnerabilities []vulnerability `json:"vulnerabilities"`
}

type vulnerability struct {
	ID            string   `json:"id"`
	Aliases       []string `json:"aliases,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	FixedVersions []string `json:"fixed_versions,omitempty"`
}

type licenseEvidence struct {
	SchemaVersion int             `json:"schema_version"`
	Sources       []licenseSource `json:"sources"`
}

type licenseSource struct {
	Lockfile string               `json:"lockfile"`
	Packages []licensedDependency `json:"packages"`
}

type licensedDependency struct {
	dependencyIdentity
	Licenses []string `json:"licenses"`
}

func normalizeDependencyEvidence(source, vulnerabilityTarget, licenseTarget string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read OSV dependency report: %w", err)
	}
	var report osvReport
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("decode OSV dependency report: %w", err)
	}
	vulnerabilities := vulnerabilityEvidence{SchemaVersion: 1, Sources: make([]vulnerabilitySource, 0, len(report.Results))}
	licenses := licenseEvidence{SchemaVersion: 1, Sources: make([]licenseSource, 0, len(report.Results))}
	seen := map[string]bool{}
	for _, result := range report.Results {
		lockfile := filepath.Base(result.Source.Path)
		if lockfile == "." || lockfile == "" || seen[lockfile] {
			return fmt.Errorf("OSV dependency report has invalid or duplicate source %q", lockfile)
		}
		seen[lockfile] = true
		vulnerabilitySource := vulnerabilitySource{Lockfile: lockfile, Packages: []vulnerableDependency{}}
		licenseSource := licenseSource{Lockfile: lockfile, Packages: make([]licensedDependency, 0, len(result.Packages))}
		for _, item := range result.Packages {
			if item.Package.Ecosystem == "" || item.Package.Name == "" || item.Package.Version == "" {
				return errors.New("OSV dependency report contains an incomplete package identity")
			}
			slices.Sort(item.Licenses)
			item.Licenses = slices.Compact(item.Licenses)
			if len(item.Licenses) == 0 {
				return fmt.Errorf("OSV dependency report has no license for %s %s", item.Package.Name, item.Package.Version)
			}
			licenseSource.Packages = append(licenseSource.Packages, licensedDependency{dependencyIdentity: item.Package, Licenses: item.Licenses})
			if len(item.Vulnerabilities) == 0 {
				continue
			}
			finding := vulnerableDependency{dependencyIdentity: item.Package, Vulnerabilities: make([]vulnerability, 0, len(item.Vulnerabilities))}
			for _, reported := range item.Vulnerabilities {
				finding.Vulnerabilities = append(finding.Vulnerabilities, reported.evidenceFor(item.Package))
			}
			slices.SortFunc(finding.Vulnerabilities, func(left, right vulnerability) int { return strings.Compare(left.ID, right.ID) })
			vulnerabilitySource.Packages = append(vulnerabilitySource.Packages, finding)
		}
		sortDependencies(vulnerabilitySource.Packages, func(item vulnerableDependency) dependencyIdentity { return item.dependencyIdentity })
		sortDependencies(licenseSource.Packages, func(item licensedDependency) dependencyIdentity { return item.dependencyIdentity })
		vulnerabilities.Sources = append(vulnerabilities.Sources, vulnerabilitySource)
		licenses.Sources = append(licenses.Sources, licenseSource)
	}
	slices.SortFunc(vulnerabilities.Sources, func(left, right vulnerabilitySource) int { return strings.Compare(left.Lockfile, right.Lockfile) })
	slices.SortFunc(licenses.Sources, func(left, right licenseSource) int { return strings.Compare(left.Lockfile, right.Lockfile) })
	if err := writeJSON(vulnerabilityTarget, vulnerabilities); err != nil {
		return fmt.Errorf("write vulnerability evidence: %w", err)
	}
	if err := writeJSON(licenseTarget, licenses); err != nil {
		return fmt.Errorf("write license evidence: %w", err)
	}
	return nil
}

func (reported osvVulnerability) evidenceFor(dependency dependencyIdentity) vulnerability {
	aliases := slices.Clone(reported.Aliases)
	slices.Sort(aliases)
	var fixed []string
	for _, affected := range reported.Affected {
		if affected.Package.Ecosystem != dependency.Ecosystem || (affected.Package.Name != dependency.Name && affected.Package.Name != "*") {
			continue
		}
		for _, versionRange := range affected.Ranges {
			switch versionRange.Type {
			case "SEMVER", "ECOSYSTEM":
			default:
				continue
			}
			for _, event := range versionRange.Events {
				if version := strings.TrimSpace(event.Fixed); version != "" {
					fixed = append(fixed, version)
				}
			}
		}
	}
	slices.Sort(fixed)
	return vulnerability{ID: reported.ID, Aliases: slices.Compact(aliases), Summary: reported.Summary, FixedVersions: slices.Compact(fixed)}
}

func sortDependencies[T any](items []T, identity func(T) dependencyIdentity) {
	slices.SortFunc(items, func(left, right T) int {
		leftID, rightID := identity(left), identity(right)
		if order := strings.Compare(leftID.Ecosystem, rightID.Ecosystem); order != 0 {
			return order
		}
		if order := strings.Compare(leftID.Name, rightID.Name); order != 0 {
			return order
		}
		return strings.Compare(leftID.Version, rightID.Version)
	})
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func normalizeSPDX(source, target, version string, instant time.Time) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read Syft SPDX document: %w", err)
	}
	for _, forbidden := range []string{"/Users/", "/home/", "/private/tmp/", `:\\Users\\`} {
		if strings.Contains(string(data), forbidden) {
			return fmt.Errorf("syft SPDX document contains a host-local path: %s", forbidden)
		}
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("decode Syft SPDX document: %w", err)
	}
	creation, _ := document["creationInfo"].(map[string]any)
	if creation == nil {
		creation = map[string]any{}
		document["creationInfo"] = creation
	}
	creation["created"] = instant.Format(time.RFC3339)
	digest := sha256.Sum256([]byte(version + "\x00" + instant.Format(time.RFC3339)))
	document["documentNamespace"] = fmt.Sprintf("urn:sha256:%x", digest)
	// A document decoded from JSON contains only values that encoding/json can
	// encode again; MarshalIndent cannot fail for this closed value domain.
	normalized, _ := json.MarshalIndent(document, "", "  ")
	return os.WriteFile(target, append(normalized, '\n'), 0o600)
}
