package main

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const defaultPolicyPath = ".config/checks/architecture/policy.toml"

// policy is the declarative SSOT loaded from TOML. Checker behavior must
// follow these fields rather than hardcoded repository layout constants.
type policy struct {
	Owner                 string                  `toml:"owner"`
	Source                string                  `toml:"source"`
	RiskModel             string                  `toml:"risk_model"`
	Measurement           string                  `toml:"measurement"`
	FalsePositiveCost     string                  `toml:"false_positive_cost"`
	Remediation           string                  `toml:"remediation"`
	ReviewCondition       string                  `toml:"review_condition"`
	GoRoots               []string                `toml:"go_roots"`
	TrackedCarrierClasses map[string]carrierClass `toml:"tracked_carrier_classes"`
	PackageChildren       map[string][]string     `toml:"package_children"`
	CompositionRootFiles  map[string][]string     `toml:"composition_root_files"`
	PeerPackageRoots      map[string][]string     `toml:"peer_package_roots"`
	AllowedImportEdges    map[string][]string     `toml:"allowed_import_edges"`
	IgnoreRoots           []string                `toml:"ignore_roots"`
	IgnoreDirectoryNames  []string                `toml:"ignore_directory_names"`
	CheckDecisionRecords  bool                    `toml:"check_decision_records"`
	CheckSemanticNames    bool                    `toml:"check_semantic_names"`
	RequireImportOwners   bool                    `toml:"require_import_owners"`
}

type carrierClass struct {
	Responsibility  string   `toml:"responsibility"`
	ExactPaths      []string `toml:"exact_paths"`
	Prefixes        []string `toml:"prefixes"`
	ExcludePrefixes []string `toml:"exclude_prefixes"`
	Suffixes        []string `toml:"suffixes"`
}

func loadPolicy(path string) (policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return policy{}, err
	}
	var p policy
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return policy{}, err
	}
	if err := validatePolicy(p); err != nil {
		return policy{}, err
	}
	return p, nil
}

func validatePolicy(p policy) error {
	for name, value := range map[string]string{
		"owner": p.Owner, "source": p.Source, "risk_model": p.RiskModel,
		"measurement": p.Measurement, "false_positive_cost": p.FalsePositiveCost,
		"remediation": p.Remediation, "review_condition": p.ReviewCondition,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must be non-empty", name)
		}
	}
	if err := validatePackagePolicy(p); err != nil {
		return err
	}
	if err := validateTrackedCarrierClasses(p.TrackedCarrierClasses); err != nil {
		return err
	}
	return nil
}

func validateTrackedCarrierClasses(classes map[string]carrierClass) error {
	if len(classes) == 0 {
		return fmt.Errorf("tracked_carrier_classes must be non-empty")
	}
	for name, class := range classes {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(class.Responsibility) == "" {
			return fmt.Errorf("tracked_carrier_classes names and responsibilities must be non-empty")
		}
		if len(class.ExactPaths) == 0 && len(class.Prefixes) == 0 && len(class.Suffixes) == 0 {
			return fmt.Errorf("tracked_carrier_classes must declare at least one selector")
		}
		paths := make([]string, 0, len(class.ExactPaths)+len(class.Prefixes)+len(class.ExcludePrefixes))
		paths = append(paths, class.ExactPaths...)
		paths = append(paths, class.Prefixes...)
		paths = append(paths, class.ExcludePrefixes...)
		for _, relative := range paths {
			if !isPortableRelativePath(relative) {
				return fmt.Errorf("tracked_carrier_classes paths must be portable relative paths")
			}
		}
		for _, suffix := range class.Suffixes {
			if !strings.HasPrefix(suffix, ".") || strings.ContainsAny(suffix, `/\\`) {
				return fmt.Errorf("tracked_carrier_classes suffixes must be file extensions")
			}
		}
	}
	return nil
}

func validatePackagePolicy(p policy) error {
	if len(p.GoRoots) == 0 {
		return fmt.Errorf("go_roots must be non-empty")
	}
	for _, root := range p.GoRoots {
		if !isPortableRelativePath(root) {
			return fmt.Errorf("go_roots entries must be non-empty relative paths")
		}
	}
	for _, membership := range []struct {
		field   string
		roots   map[string][]string
		minimum int
		accepts func(string) bool
	}{
		{"package_children", p.PackageChildren, 1, isPortableBaseName},
		{"peer_package_roots", p.PeerPackageRoots, 0, isPortableBaseName},
		{"composition_root_files", p.CompositionRootFiles, 1, func(name string) bool {
			return isPortableBaseName(name) && strings.HasSuffix(name, ".go")
		}},
		{"allowed_import_edges", p.AllowedImportEdges, 0, isPortableRelativePath},
	} {
		for _, root := range slices.Sorted(maps.Keys(membership.roots)) {
			members := membership.roots[root]
			if !isPortableRelativePath(root) || len(members) < membership.minimum {
				return fmt.Errorf("%s requires relative roots and at least %d members", membership.field, membership.minimum)
			}
			seen := make(map[string]bool, len(members))
			for _, member := range members {
				if !membership.accepts(member) || seen[member] {
					return fmt.Errorf("%s has invalid or duplicate member %q in %q", membership.field, member, root)
				}
				seen[member] = true
			}
		}
	}
	return nil
}

func isPortableBaseName(value string) bool {
	return isPortableRelativePath(value) && !strings.Contains(value, "/")
}

func isPortableRelativePath(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || strings.Contains(value, `\`) || strings.HasPrefix(value, "/") {
		return false
	}
	if len(value) >= 2 && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z')) && value[1] == ':' {
		return false
	}
	for element := range strings.SplitSeq(value, "/") {
		if element == "" || element == "." || element == ".." {
			return false
		}
	}
	return true
}

func (p policy) ignoreRootSet() map[string]struct{} {
	out := make(map[string]struct{}, len(p.IgnoreRoots))
	for _, name := range p.IgnoreRoots {
		out[name] = struct{}{}
	}
	return out
}

func (p policy) ignoreDirectoryNameSet() map[string]struct{} {
	out := make(map[string]struct{}, len(p.IgnoreDirectoryNames))
	for _, name := range p.IgnoreDirectoryNames {
		out[name] = struct{}{}
	}
	return out
}
