package construction

import (
	"aigw-cli/internal/process"
	"aigw-cli/tools/release/artifact"
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // SPDX 2.3 requires SHA1 file metadata; signed SHA256 owns integrity.
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/pelletier/go-toml/v2"
)

type dependencyPolicy struct {
	IgnoredVulns     []dependencyException `toml:"IgnoredVulns"`
	PackageOverrides []struct {
		Name      string `toml:"name"`
		Version   string `toml:"version"`
		Ecosystem string `toml:"ecosystem"`
		Reason    string `toml:"reason"`
		License   struct {
			Override []string `toml:"override"`
		} `toml:"license"`
	} `toml:"PackageOverrides"`
}

type dependencyException struct {
	ID          string    `toml:"id"`
	Reason      string    `toml:"reason"`
	IgnoreUntil time.Time `toml:"ignoreUntil"`
}

func readDependencyPolicy(root string) (dependencyPolicy, error) {
	var policy dependencyPolicy
	data, err := os.ReadFile(filepath.Join(root, ".config", "checks", "dependencies", "policy.toml"))
	if err != nil {
		return policy, fmt.Errorf("read dependency policy: %w", err)
	}
	if err := toml.NewDecoder(strings.NewReader(string(data))).DisallowUnknownFields().Decode(&policy); err != nil {
		return policy, fmt.Errorf("decode dependency policy: %w", err)
	}
	if len(policy.IgnoredVulns) == 0 {
		return policy, nil
	}
	var lock struct {
		LockfileVersion int `json:"lockfileVersion"`
		Packages        map[string]struct {
			Version string `json:"version"`
			Dev     bool   `json:"dev"`
		} `json:"packages"`
	}
	data, err = os.ReadFile(filepath.Join(root, "package-lock.json"))
	if err != nil {
		return policy, fmt.Errorf("read dependency exception scope: %w", err)
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return policy, fmt.Errorf("decode dependency exception scope: %w", err)
	}
	// Exact authorized development boundary; this is product admission, not
	// a conditional OSV rule. Retire with the native policy entry.
	for _, exception := range policy.IgnoredVulns {
		if exception.ID == "" || strings.TrimSpace(exception.Reason) == "" || !exception.IgnoreUntil.After(time.Now()) || exception.IgnoreUntil.After(time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)) ||
			exception.ID != "GHSA-vfj7-8cjw-p6xm" || lock.LockfileVersion != 3 {
			return policy, fmt.Errorf("dependency exception %q requires an unexpired exact npm development scope", exception.ID)
		}
		matches := 0
		for path, item := range lock.Packages {
			if path != "node_modules/braces" && !strings.HasSuffix(path, "/node_modules/braces") {
				continue
			}
			if item.Version != "3.0.3" || !item.Dev {
				return policy, fmt.Errorf("dependency exception %s does not admit the locked package at %s", exception.ID, path)
			}
			matches++
		}
		if matches == 0 {
			return policy, fmt.Errorf("dependency exception %s has no current locked consumer", exception.ID)
		}
	}
	return policy, nil
}

func validateDependencyExceptions(policy dependencyPolicy, raw string) error {
	data, err := os.ReadFile(raw)
	if err != nil {
		return err
	}
	var report osvReport
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("decode dependency exception evidence: %w", err)
	}
	for _, exception := range policy.IgnoredVulns {
		matches := 0
		for _, result := range report.Results {
			for _, item := range result.Packages {
				for _, finding := range item.Vulnerabilities {
					if finding.ID != exception.ID && !slices.Contains(finding.Aliases, exception.ID) {
						continue
					}
					if item.Package.Ecosystem != "npm" || item.Package.Name != "braces" || item.Package.Version != "3.0.3" || !slices.Equal(item.DependencyGroups, []string{"dev"}) {
						return fmt.Errorf("dependency exception %s does not admit the native finding identity", exception.ID)
					}
					for _, fixed := range finding.evidenceFor(item.Package).FixedVersions {
						version, parseErr := semver.StrictNewVersion(fixed)
						if parseErr == nil && version.Prerelease() == "" {
							return fmt.Errorf("dependency exception %s must retire: official stable fix %s is reported", exception.ID, fixed)
						}
					}
					matches++
				}
			}
		}
		if matches == 0 {
			return fmt.Errorf("dependency exception %s has no current raw finding; remove the obsolete disposition", exception.ID)
		}
	}
	return nil
}

func scanDependencies(root, output string, policy dependencyPolicy, run toolRunner) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(output, 0o700); err != nil {
		return "", err
	}
	evidence, err := os.MkdirTemp(output, "scan-")
	if err != nil {
		return "", err
	}
	unfiltered := policy
	unfiltered.IgnoredVulns = nil
	data, err := toml.Marshal(unfiltered)
	if err != nil {
		return "", err
	}
	config := filepath.Join(evidence, "unfiltered.toml")
	if err := os.WriteFile(config, data, 0o600); err != nil {
		return "", err
	}
	raw := filepath.Join(evidence, "dependencies.raw.json")
	lockfiles := []string{filepath.Join(root, "go.mod"), filepath.Join(root, "package-lock.json")}
	args := []string{"scan", "source", "--config", config, "--lockfile", lockfiles[0], "--lockfile", lockfiles[1], "--no-call-analysis=go", "--format", "json", "--all-packages", "--all-vulns", "--licenses=", "--output-file", raw}
	call := toolCall{Name: "osv-scanner", Directory: root, Args: args, Timeout: 2 * time.Minute}
	if err := runDependencyScan(call, run); err != nil {
		exit, ok := errors.AsType[*exec.ExitError](err)
		if !ok || exit.ExitCode() != 1 {
			return raw, fmt.Errorf("scan raw dependencies: %w", err)
		}
	}
	if err := normalizeDependencyEvidence(raw, filepath.Join(evidence, "vulnerabilities.json"), filepath.Join(evidence, "licenses.json"), lockfiles); err != nil {
		return raw, err
	}
	if err := validateDependencyExceptions(policy, raw); err != nil {
		return raw, err
	}
	call.Args = slices.Clone(args)
	data, err = toml.Marshal(policy)
	if err != nil {
		return raw, err
	}
	config = filepath.Join(evidence, "disposition.toml")
	if err := os.WriteFile(config, data, 0o600); err != nil {
		return raw, err
	}
	call.Args[3] = config
	disposition := filepath.Join(evidence, "dependencies.disposition.json")
	call.Args[len(call.Args)-1] = disposition
	if err := runDependencyScan(call, run); err != nil {
		return raw, fmt.Errorf("native dependency disposition refused; complete evidence retained at %s: %w", evidence, err)
	}
	if err := normalizeDependencyEvidence(disposition, filepath.Join(evidence, "disposition-vulnerabilities.json"), filepath.Join(evidence, "disposition-licenses.json"), lockfiles); err != nil {
		return raw, err
	}
	original, err := os.ReadFile(filepath.Join(evidence, "licenses.json"))
	if err != nil {
		return raw, err
	}
	decided, err := os.ReadFile(filepath.Join(evidence, "disposition-licenses.json"))
	if err != nil || !bytes.Equal(original, decided) {
		return raw, errors.New("native dependency disposition changed the complete package/license inventory")
	}
	return raw, nil
}

func runDependencyScan(call toolCall, run toolRunner) error {
	output := call.Args[len(call.Args)-1]
	stdout, err := os.Create(output + ".stdout")
	if err != nil {
		return err
	}
	defer func() { _ = stdout.Close() }()
	stderr, err := os.Create(output + ".stderr")
	if err != nil {
		return err
	}
	defer func() { _ = stderr.Close() }()
	call.Stdout, call.Stderr = stdout, stderr
	result := run(call)
	code := 0
	if result != nil {
		code = -1
		if exit, ok := errors.AsType[*exec.ExitError](result); ok {
			code = exit.ExitCode()
		}
	}
	if err := stderr.Close(); err != nil {
		return errors.Join(result, err)
	}
	diagnostics, readErr := os.ReadFile(output + ".stderr")
	if readErr != nil {
		return errors.Join(result, readErr)
	}
	invalidDiagnostics := process.DiagnosticFailure(diagnostics)
	if err := writeJSON(output+".exit.json", struct {
		Exit   int  `json:"exit"`
		Failed bool `json:"failed"`
	}{code, result != nil || invalidDiagnostics}); err != nil {
		return errors.Join(result, err)
	}
	if invalidDiagnostics {
		return fmt.Errorf("native dependency diagnostics prevent qualification; retained at %s", output+".stderr")
	}
	return result
}

// ScanDependencies preserves complete raw evidence before native policy disposition.
func ScanDependencies(ctx context.Context, root, output string) error {
	policy, err := readDependencyPolicy(root)
	if err != nil {
		return err
	}
	_, err = scanDependencies(root, output, policy, executeTool(ctx))
	return err
}

type dependencyIdentity struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
	Version   string `json:"version"`
}

type osvReport struct {
	Results []struct {
		Source struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"source"`
		Packages []struct {
			DependencyGroups []string           `json:"dependency_groups"`
			Package          dependencyIdentity `json:"package"`
			Licenses         []string           `json:"licenses"`
			Vulnerabilities  []osvVulnerability `json:"vulnerabilities"`
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

func normalizeDependencyEvidence(source, vulnerabilityTarget, licenseTarget string, lockfiles []string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read OSV dependency report: %w", err)
	}
	var report osvReport
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("decode OSV dependency report: %w", err)
	}
	if len(lockfiles) == 0 || len(report.Results) != len(lockfiles) {
		return errors.New("OSV dependency report does not cover the selected lockfiles")
	}
	remaining := make(map[string]bool, len(lockfiles))
	for _, path := range lockfiles {
		remaining[filepath.Clean(path)] = true
	}
	vulnerabilities := vulnerabilityEvidence{SchemaVersion: 1, Sources: make([]vulnerabilitySource, 0, len(report.Results))}
	licenses := licenseEvidence{SchemaVersion: 1, Sources: make([]licenseSource, 0, len(report.Results))}
	for _, result := range report.Results {
		path := filepath.Clean(result.Source.Path)
		if !remaining[path] || result.Source.Type != "lockfile" || len(result.Packages) == 0 {
			return fmt.Errorf("OSV dependency report has an unselected, duplicate or unobserved lockfile %q", result.Source.Path)
		}
		delete(remaining, path)
		lockfile := filepath.Base(path)
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
			for _, license := range item.Licenses {
				switch strings.ToUpper(strings.TrimSpace(license)) {
				case "", "UNKNOWN", "NOASSERTION", "NONE":
					return fmt.Errorf("OSV dependency report has no established license for %s %s", item.Package.Name, item.Package.Version)
				}
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
	files, ok := document["files"].([]any)
	if !ok || len(files) != len(artifact.Archives(version)) {
		return errors.New("Syft SPDX document must describe the complete release binary matrix")
	}
	if err := bindSPDXFiles(filepath.Dir(source), files); err != nil {
		return err
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

func bindSPDXFiles(directory string, files []any) error {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	seen := make(map[string]bool, len(files))
	for _, value := range files {
		file, ok := value.(map[string]any)
		if !ok {
			return errors.New("SPDX binary entry must be an object")
		}
		name, _ := file["fileName"].(string)
		// Syft paths are rooted at the scan, not the host; Windows may retain
		// the root separator because its SPDX converter only recognizes '/'.
		name = strings.TrimPrefix(strings.ReplaceAll(name, `\`, "/"), "/")
		name = filepath.Clean(filepath.FromSlash(name))
		if !filepath.IsLocal(name) || seen[name] {
			return fmt.Errorf("SPDX binary path is not unique and relative: %q", name)
		}
		seen[name] = true
		data, err := root.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read SPDX binary %q: %w", name, err)
		}
		sha256Value := fmt.Sprintf("%x", sha256.Sum256(data))
		checksums, _ := file["checksums"].([]any)
		for _, item := range checksums {
			checksum, ok := item.(map[string]any)
			if !ok {
				return errors.New("SPDX checksum must be an object")
			}
			if checksum["algorithm"] == "SHA256" && checksum["checksumValue"] != sha256Value {
				return fmt.Errorf("SPDX SHA256 differs from emitted binary %q", name)
			}
		}
		// SPDX mandates SHA1 metadata; SHA256 remains the integrity authority.
		sha1Value := fmt.Sprintf("%x", sha1.Sum(data)) //nolint:gosec // Required non-security SPDX checksum.
		file["fileName"] = filepath.ToSlash(name)
		file["checksums"] = []map[string]string{
			{"algorithm": "SHA1", "checksumValue": sha1Value},
			{"algorithm": "SHA256", "checksumValue": sha256Value},
		}
	}
	return nil
}
