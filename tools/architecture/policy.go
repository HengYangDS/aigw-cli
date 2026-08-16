package main

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const defaultPolicyPath = ".config/checks/architecture/policy.toml"

// policy is the declarative SSOT loaded from TOML. Checker behavior must
// follow these fields rather than hardcoded repository layout constants.
type policy struct {
	Owner                     string              `toml:"owner"`
	Source                    string              `toml:"source"`
	GoRoots                   []string            `toml:"go_roots"`
	PackageChildren           map[string][]string `toml:"package_children"`
	CompositionRootFiles      map[string][]string `toml:"composition_root_files"`
	PeerPackageRoots          map[string][]string `toml:"peer_package_roots"`
	AllowedImportEdges        map[string][]string `toml:"allowed_import_edges"`
	IgnoreRoots               []string            `toml:"ignore_roots"`
	IgnoreDirectoryNames      []string            `toml:"ignore_directory_names"`
	CheckExportedTypeAlias    bool                `toml:"check_exported_type_alias"`
	CheckFunctionVarAlias     bool                `toml:"check_function_var_alias"`
	CheckPackageDocumentation bool                `toml:"check_package_documentation"`
	CheckTrivialWrappers      bool                `toml:"check_trivial_wrappers"`
	CheckDecisionRecords      bool                `toml:"check_decision_records"`
	CheckSemanticNames        bool                `toml:"check_semantic_names"`
	CheckModuleIdentity       bool                `toml:"check_module_identity"`
	CheckPortability          bool                `toml:"check_portability"`
	RequireImportOwners       bool                `toml:"require_import_owners"`
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
	if strings.TrimSpace(p.Owner) == "" || strings.TrimSpace(p.Source) == "" {
		return fmt.Errorf("owner and source must be non-empty")
	}
	if err := validatePackagePolicy(p); err != nil {
		return err
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
	for root, children := range p.PackageChildren {
		if err := validateRelativeRoot(root, "package_children"); err != nil {
			return err
		}
		if len(children) == 0 {
			return fmt.Errorf("package_children values must be non-empty")
		}
		seen := map[string]bool{}
		for _, child := range children {
			if strings.TrimSpace(child) == "" || path.Base(child) != child || strings.ContainsAny(child, `/\\`) || seen[child] {
				return fmt.Errorf("package_children values must be unique child package names")
			}
			seen[child] = true
		}
	}
	for root, files := range p.CompositionRootFiles {
		if !isPortableRelativePath(root) || len(files) == 0 {
			return fmt.Errorf("composition_root_files keys must be relative paths with non-empty file lists")
		}
		seen := map[string]bool{}
		for _, file := range files {
			if strings.TrimSpace(file) == "" || path.Base(file) != file || !strings.HasSuffix(file, ".go") || seen[file] {
				return fmt.Errorf("composition_root_files values must be unique .go base names")
			}
			seen[file] = true
		}
	}
	for root, allowed := range p.PeerPackageRoots {
		if err := validateRelativeRoot(root, "peer_package_roots"); err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, name := range allowed {
			if strings.TrimSpace(name) == "" || path.Base(name) != name || strings.ContainsAny(name, `/\\`) || seen[name] {
				return fmt.Errorf("peer_package_roots values must be unique child package names")
			}
			seen[name] = true
		}
	}
	for source, targets := range p.AllowedImportEdges {
		if err := validateRelativeRoot(source, "allowed_import_edges"); err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, target := range targets {
			if err := validateRelativeRoot(target, "allowed_import_edges"); err != nil {
				return err
			}
			if seen[target] {
				return fmt.Errorf("allowed_import_edges values must be unique package paths")
			}
			seen[target] = true
		}
	}
	return nil
}

func validateRelativeRoot(root, field string) error {
	if !isPortableRelativePath(root) {
		return fmt.Errorf("%s keys must be non-empty relative paths", field)
	}
	return nil
}

func isPortableRelativePath(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || strings.Contains(value, `\`) || strings.HasPrefix(value, "/") {
		return false
	}
	if len(value) >= 2 && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z')) && value[1] == ':' {
		return false
	}
	for _, element := range strings.Split(value, "/") {
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
