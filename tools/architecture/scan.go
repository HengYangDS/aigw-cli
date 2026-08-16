package main

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var repositoryAnalysis = struct {
	decisionRecords func(string, *Report) error
	peerImports     func(string, []goFileInfo, policy, *Report) error
	importEdges     func(string, []goFileInfo, policy, *Report) error
	goAST           func(string, []goFileInfo, policy, *Report) error
}{
	decisionRecords: checkDecisionRecords,
	peerImports:     checkPeerPackageImports,
	importEdges:     checkImportEdges,
	goAST:           checkGoAST,
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
		if err := checkSemanticNames(absRoot, &report); err != nil {
			return Report{}, err
		}
	}
	if err := checkTextLayout(absRoot, &report); err != nil {
		return Report{}, err
	}
	if err := checkPackageChildren(absRoot, p, &report); err != nil {
		return Report{}, err
	}
	goFiles, err := collectGoFiles(absRoot, p)
	if err != nil {
		return Report{}, err
	}
	checkImportOwners(goFiles, p, &report)
	checkCompositionRoots(goFiles, p, &report)
	if err := repositoryAnalysis.peerImports(absRoot, goFiles, p, &report); err != nil {
		return Report{}, err
	}
	if err := repositoryAnalysis.importEdges(absRoot, goFiles, p, &report); err != nil {
		return Report{}, err
	}
	if err := repositoryAnalysis.goAST(absRoot, goFiles, p, &report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func checkImportOwners(files []goFileInfo, p policy, report *Report) {
	if !p.RequireImportOwners {
		return
	}
	managed := make(map[string]bool, len(p.AllowedImportEdges))
	for owner := range p.AllowedImportEdges {
		managed[owner] = true
	}
	seen := map[string]bool{}
	for _, file := range files {
		if file.isTest || seen[file.dir] || managed[file.dir] {
			continue
		}
		seen[file.dir] = true
		report.addFinding(Finding{
			Rule:    "unmanaged_import_owner",
			Path:    file.dir,
			Message: "production package is missing from allowed_import_edges",
		})
	}
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
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("stat package root %s: %w", packageRoot, err)
		}
		if !info.IsDir() {
			continue
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

func checkImportEdges(root string, files []goFileInfo, p policy, report *Report) error {
	if len(p.AllowedImportEdges) == 0 {
		return nil
	}
	fset := token.NewFileSet()
	module, err := readModuleIdentity(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	modulePrefix := module + "/"
	for _, file := range files {
		if file.isTest {
			continue
		}
		allowedTargets, managed := p.AllowedImportEdges[file.dir]
		if !managed {
			continue
		}
		allowed := make(map[string]bool, len(allowedTargets))
		for _, target := range allowedTargets {
			allowed[target] = true
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.relPath)))
		if err != nil {
			return fmt.Errorf("read %s: %w", file.relPath, err)
		}
		parsed, err := parser.ParseFile(fset, file.relPath, data, parser.ImportsOnly)
		if err != nil {
			continue
		}
		for _, imported := range parsed.Imports {
			importPath, _ := strconv.Unquote(imported.Path.Value)
			if !strings.HasPrefix(importPath, modulePrefix) {
				continue
			}
			target := strings.TrimPrefix(importPath, modulePrefix)
			if target == file.dir || allowed[target] {
				continue
			}
			pos := fset.Position(imported.Pos())
			report.addFinding(Finding{
				Rule:    "import_edge",
				Path:    file.relPath,
				Line:    pos.Line,
				Package: target,
				Message: fmt.Sprintf("package %q may not import %q; declare the dependency or move shared behavior to a neutral owner", file.dir, target),
			})
		}
	}
	return nil
}

func checkPeerPackageImports(root string, files []goFileInfo, p policy, report *Report) error {
	fset := token.NewFileSet()
	for peerRoot, allowedNames := range p.PeerPackageRoots {
		allowed := make(map[string]bool, len(allowedNames))
		for _, name := range allowedNames {
			allowed[name] = true
		}
		for _, file := range files {
			if file.isTest || path.Dir(file.dir) != peerRoot {
				continue
			}
			sourceChild := path.Base(file.dir)
			abs := filepath.Join(root, filepath.FromSlash(file.relPath))
			data, err := os.ReadFile(abs)
			if err != nil {
				return fmt.Errorf("read %s: %w", file.relPath, err)
			}
			parsed, err := parser.ParseFile(fset, file.relPath, data, parser.ImportsOnly)
			if err != nil {
				continue
			}
			for _, imported := range parsed.Imports {
				path, _ := strconv.Unquote(imported.Path.Value)
				marker := "/" + peerRoot + "/"
				index := strings.Index(path, marker)
				if index < 0 {
					continue
				}
				tail := path[index+len(marker):]
				targetChild, _, _ := strings.Cut(tail, "/")
				if targetChild == "" || targetChild == sourceChild || allowed[targetChild] {
					continue
				}
				pos := fset.Position(imported.Pos())
				report.addFinding(Finding{
					Rule:    "peer_package_import",
					Path:    file.relPath,
					Line:    pos.Line,
					Package: targetChild,
					Message: fmt.Sprintf("peer package %q may not import sibling %q under %s; move shared behavior to its domain owner", sourceChild, targetChild, peerRoot),
				})
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
	var files []goFileInfo

	for _, goRoot := range p.GoRoots {
		abs := filepath.Join(root, filepath.FromSlash(goRoot))
		info, err := os.Stat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat go root %s: %w", goRoot, err)
		}
		if !info.IsDir() {
			continue
		}
		err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			name := d.Name()
			rel, err := filepath.Rel(root, path)
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
	_, ok := p.ignoreDirectoryNameSet()[name]
	return ok
}

func shouldIgnoreRelPath(relPOSIX string, p policy) bool {
	parts := strings.Split(relPOSIX, "/")
	if _, ok := p.ignoreRootSet()[parts[0]]; ok {
		return true
	}
	ignoreDirs := p.ignoreDirectoryNameSet()
	for _, part := range parts {
		if _, ok := ignoreDirs[part]; ok {
			return true
		}
	}
	return false
}
