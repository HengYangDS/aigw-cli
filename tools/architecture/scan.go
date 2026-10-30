package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

var repositoryAnalysis = struct {
	decisionRecords func(string, *Report) error
	semanticNames   func(string, *Report) error
	packageChildren func(string, policy, *Report) error
	goFiles         func(string, policy) ([]goFileInfo, error)
	layers          func(string, []goFileInfo, policy, *Report) error
}{
	decisionRecords: checkDecisionRecords,
	semanticNames:   checkSemanticNames,
	packageChildren: checkPackageChildren,
	goFiles:         collectGoFiles,
	layers:          checkLayers,
}

type goFileInfo struct {
	relPath string
	name    string
	dir     string
	isTest  bool
}

func analyzeRepository(root string, p policy, policyPath string) (Report, error) {
	absRoot := filepath.Clean(root)
	report := newReport(policyPath, absRoot)
	if p.CheckDecisionRecords {
		if err := repositoryAnalysis.decisionRecords(absRoot, &report); err != nil {
			return Report{}, err
		}
	}
	if p.CheckSemanticNames {
		if err := repositoryAnalysis.semanticNames(absRoot, &report); err != nil {
			return Report{}, err
		}
	}
	tracked, err := trackedFiles(absRoot)
	if err != nil {
		return Report{}, err
	}
	checkTrackedCarrierClasses(tracked, p, &report)
	if err := repositoryAnalysis.packageChildren(absRoot, p, &report); err != nil {
		return Report{}, err
	}
	goFiles, err := repositoryAnalysis.goFiles(absRoot, p)
	if err != nil {
		return Report{}, err
	}
	checkCompositionRoots(goFiles, p, &report)
	if err := checkClientConfinement(absRoot, goFiles, p.ClientConfinement, &report); err != nil {
		return Report{}, err
	}
	if err := repositoryAnalysis.layers(absRoot, goFiles, p, &report); err != nil {
		return Report{}, err
	}
	if err := checkStandardLibraryOnly(absRoot, goFiles, p, &report); err != nil {
		return Report{}, err
	}
	if err := checkUsedExportsOnly(absRoot, goFiles, p, &report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func checkTrackedCarrierClasses(files []string, p policy, report *Report) {
	for _, file := range files {
		matches := make([]string, 0, 1)
		for name, class := range p.TrackedCarrierClasses {
			if class.matches(file) {
				matches = append(matches, name)
			}
		}
		if len(matches) == 1 {
			continue
		}
		sort.Strings(matches)
		message := "tracked carrier has no declared responsibility"
		if len(matches) > 1 {
			message = fmt.Sprintf("tracked carrier has multiple responsibilities: %s", strings.Join(matches, ", "))
		}
		report.addFinding(Finding{
			Rule:    "tracked_carrier_responsibility",
			Path:    file,
			Name:    file,
			Message: message,
		})
	}
}

func (class carrierClass) matches(file string) bool {
	for _, excluded := range class.ExcludePrefixes {
		if pathContains(excluded, file) {
			return false
		}
	}
	if slices.Contains(class.ExactPaths, file) {
		return true
	}
	for _, prefix := range class.Prefixes {
		if !pathContains(prefix, file) {
			continue
		}
		if len(class.Suffixes) == 0 || slices.ContainsFunc(class.Suffixes, func(suffix string) bool {
			return strings.HasSuffix(file, suffix)
		}) {
			return true
		}
	}
	return len(class.Prefixes) == 0 && slices.ContainsFunc(class.Suffixes, func(suffix string) bool {
		return strings.HasSuffix(file, suffix)
	})
}

func pathContains(parent, file string) bool {
	return file == parent || strings.HasPrefix(file, parent+"/")
}

func checkPackageChildren(root string, p policy, report *Report) error {
	return checkPackageChildrenWithReadDir(root, p, report, os.ReadDir)
}

func checkPackageChildrenWithReadDir(
	root string,
	p policy,
	report *Report,
	readDir func(string) ([]fs.DirEntry, error),
) error {
	for packageRoot, allowedChildren := range p.PackageChildren {
		packageRootPath := filepath.Join(root, filepath.FromSlash(packageRoot))
		info, err := os.Stat(packageRootPath)
		if err != nil {
			return fmt.Errorf("stat package root %s: %w", packageRoot, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("package root %s is not a directory", packageRoot)
		}
		entries, err := readDir(packageRootPath)
		if err != nil {
			return fmt.Errorf("read package root %s: %w", packageRoot, err)
		}
		allowed := make(map[string]bool, len(allowedChildren))
		for _, child := range allowedChildren {
			allowed[child] = true
		}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name()[0] == '.' {
				continue
			}
			if !allowed[entry.Name()] {
				report.addFinding(Finding{
					Rule:    "package_child",
					Path:    path.Join(packageRoot, entry.Name()),
					Name:    entry.Name(),
					Message: fmt.Sprintf("package root %q admits only its declared semantic owners", packageRoot),
				})
				continue
			}
		}
	}
	return nil
}

func checkCompositionRoots(files []goFileInfo, p policy, report *Report) {
	for root, allowedFiles := range p.CompositionRootFiles {
		allowed := make(map[string]bool, len(allowedFiles))
		for _, name := range allowedFiles {
			allowed[name] = true
		}
		for _, file := range files {
			if file.isTest || file.dir != root || allowed[file.name] {
				continue
			}
			report.addFinding(Finding{
				Rule:    "composition_root_file",
				Path:    file.relPath,
				Name:    file.name,
				Files:   append([]string(nil), allowedFiles...),
				Message: fmt.Sprintf("composition root %q permits only %s; move behavior to its semantic subpackage", root, strings.Join(allowedFiles, ", ")),
			})
		}
	}
}

func collectGoFiles(root string, p policy) ([]goFileInfo, error) {
	return collectGoFilesWith(root, p, filepath.WalkDir, filepath.Rel)
}

func collectGoFilesWith(
	root string,
	p policy,
	walkDir func(string, fs.WalkDirFunc) error,
	relative func(string, string) (string, error),
) ([]goFileInfo, error) {
	var files []goFileInfo

	for _, goRoot := range p.GoRoots {
		abs := filepath.Join(root, filepath.FromSlash(goRoot))
		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("stat go root %s: %w", goRoot, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("go root %s is not a directory", goRoot)
		}
		err = walkDir(abs, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			name := d.Name()
			rel, err := relative(root, path)
			if err != nil {
				return err
			}
			relPOSIX := toPOSIX(rel)
			if d.IsDir() {
				if path == abs {
					return nil
				}
				if shouldIgnoreDirName(name, p) || shouldIgnoreRelPath(relPOSIX, p) {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(name, ".go") {
				return nil
			}
			if shouldIgnoreRelPath(relPOSIX, p) {
				return nil
			}
			isTest := isTestGoFile(name)
			info := goFileInfo{
				relPath: relPOSIX,
				name:    name,
				dir:     toPOSIX(filepath.Dir(rel)),
				isTest:  isTest,
			}
			files = append(files, info)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk go root %s: %w", goRoot, err)
		}
	}

	sort.Slice(files, func(i, j int) bool { return files[i].relPath < files[j].relPath })
	return files, nil
}

func isTestGoFile(name string) bool {
	return strings.HasSuffix(name, "_test.go")
}

func shouldIgnoreDirName(name string, p policy) bool {
	if name == "." || name == ".." {
		return false
	}
	return slices.Contains(p.IgnoreDirectoryNames, name)
}

func shouldIgnoreRelPath(relPOSIX string, p policy) bool {
	root, _, _ := strings.Cut(relPOSIX, "/")
	if slices.Contains(p.IgnoreRoots, root) {
		return true
	}
	for part := range strings.SplitSeq(relPOSIX, "/") {
		if slices.Contains(p.IgnoreDirectoryNames, part) {
			return true
		}
	}
	return false
}
