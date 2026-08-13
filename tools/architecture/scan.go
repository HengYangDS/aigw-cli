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
	"unicode"
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
	relPath    string
	name       string
	dir        string
	isTest     bool
	eloc       int
	complexity int
	functions  []functionMetric
}

func analyzeRepository(root string, p policy, policyPath string) (Report, error) {
	absRoot := filepath.Clean(root)
	report := newReport(policyPath, absRoot)
	if p.CheckModuleIdentity {
		if err := checkModuleIdentity(absRoot, &report); err != nil {
			return Report{}, err
		}
	}
	if p.CheckPortability {
		if err := checkPortability(absRoot, &report); err != nil {
			return Report{}, err
		}
	}

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
	goFiles, dirStats, err := collectGoFiles(absRoot, p)
	if err != nil {
		return Report{}, err
	}
	report.DirectoryStats = dirStats
	checkImportOwners(goFiles, p, &report)
	checkCompositionRoots(goFiles, p, &report)
	if err := repositoryAnalysis.peerImports(absRoot, goFiles, p, &report); err != nil {
		return Report{}, err
	}
	if err := repositoryAnalysis.importEdges(absRoot, goFiles, p, &report); err != nil {
		return Report{}, err
	}
	checkFlatDirectories(dirStats, p, &report)
	checkSourceBudgets(goFiles, dirStats, p, &report)
	checkSuffixFlat(goFiles, p, &report)
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
		entries, err := os.ReadDir(packageRootPath)
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
			const modulePrefix = "aigw-cli/"
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

func collectGoFiles(root string, p policy) ([]goFileInfo, []DirectoryStats, error) {
	var files []goFileInfo
	statsByDir := map[string]*DirectoryStats{}

	for _, goRoot := range p.GoRoots {
		abs := filepath.Join(root, filepath.FromSlash(goRoot))
		info, err := os.Stat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, fmt.Errorf("stat go root %s: %w", goRoot, err)
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
			dirRel := toPOSIX(filepath.Dir(rel))
			stat := statsByDir[dirRel]
			if stat == nil {
				stat = &DirectoryStats{Path: dirRel}
				statsByDir[dirRel] = stat
			}
			isTest := isTestGoFile(name)
			info := goFileInfo{
				relPath: relPOSIX,
				name:    name,
				dir:     dirRel,
				isTest:  isTest,
			}
			info.eloc, info.complexity, err = sourceMetrics(path)
			if err != nil {
				return fmt.Errorf("measure %s: %w", relPOSIX, err)
			}
			info.functions, err = functionMetrics(path)
			if err != nil {
				return fmt.Errorf("measure functions in %s: %w", relPOSIX, err)
			}
			files = append(files, info)
			if isTest {
				stat.TestCount++
				stat.TestELOC += info.eloc
				stat.TestComplexity += info.complexity
				stat.TestFiles = append(stat.TestFiles, name)
			} else {
				stat.ProductionCount++
				stat.ProductionELOC += info.eloc
				stat.ProductionComplexity += info.complexity
				stat.ProductionFiles = append(stat.ProductionFiles, name)
			}
			return nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("walk go root %s: %w", goRoot, err)
		}
	}

	dirStats := make([]DirectoryStats, 0, len(statsByDir))
	for _, stat := range statsByDir {
		sort.Strings(stat.ProductionFiles)
		sort.Strings(stat.TestFiles)
		dirStats = append(dirStats, *stat)
	}
	sort.Slice(dirStats, func(i, j int) bool { return dirStats[i].Path < dirStats[j].Path })
	sort.Slice(files, func(i, j int) bool { return files[i].relPath < files[j].relPath })
	return files, dirStats, nil
}

func checkSourceBudgets(files []goFileInfo, stats []DirectoryStats, p policy, report *Report) {
	for _, file := range files {
		elocLimit := p.MaxFileELOC
		complexityLimit := p.MaxFileComplexity
		if file.isTest {
			elocLimit = p.MaxTestFileELOC
			complexityLimit = p.MaxTestFileComplexity
		}
		if file.eloc > elocLimit {
			report.addFinding(Finding{
				Rule:    "file_eloc",
				Path:    file.relPath,
				Count:   file.eloc,
				Limit:   elocLimit,
				Message: fmt.Sprintf("file has %d effective lines; limit is %d", file.eloc, elocLimit),
			})
		}
		if file.complexity > complexityLimit {
			report.addFinding(Finding{
				Rule:    "file_complexity",
				Path:    file.relPath,
				Count:   file.complexity,
				Limit:   complexityLimit,
				Message: fmt.Sprintf("file decision complexity is %d; limit is %d", file.complexity, complexityLimit),
			})
		}
		functionELOCLimit := p.MaxFunctionELOC
		functionComplexityLimit := p.MaxFunctionComplexity
		functionNestingLimit := p.MaxNestingDepth
		if file.isTest {
			functionELOCLimit = p.MaxTestFunctionELOC
			functionComplexityLimit = p.MaxTestFunctionComplexity
			functionNestingLimit = p.MaxTestNestingDepth
		}
		for _, function := range file.functions {
			if function.ELOC > functionELOCLimit {
				report.addFinding(Finding{Rule: "function_eloc", Path: file.relPath, Line: function.Line, Name: function.Name, Count: function.ELOC, Limit: functionELOCLimit, Message: fmt.Sprintf("function %s has %d effective lines; limit is %d", function.Name, function.ELOC, functionELOCLimit)})
			}
			if function.Complexity > functionComplexityLimit {
				report.addFinding(Finding{Rule: "function_complexity", Path: file.relPath, Line: function.Line, Name: function.Name, Count: function.Complexity, Limit: functionComplexityLimit, Message: fmt.Sprintf("function %s decision complexity is %d; limit is %d", function.Name, function.Complexity, functionComplexityLimit)})
			}
			if function.Nesting > functionNestingLimit {
				report.addFinding(Finding{Rule: "function_nesting", Path: file.relPath, Line: function.Line, Name: function.Name, Count: function.Nesting, Limit: functionNestingLimit, Message: fmt.Sprintf("function %s nesting depth is %d; limit is %d", function.Name, function.Nesting, functionNestingLimit)})
			}
		}
	}
	for _, stat := range stats {
		if stat.ProductionELOC > p.MaxDirectoryELOC {
			report.addFinding(Finding{
				Rule:    "directory_eloc",
				Path:    stat.Path,
				Count:   stat.ProductionELOC,
				Limit:   p.MaxDirectoryELOC,
				Message: fmt.Sprintf("directory has %d effective lines; limit is %d", stat.ProductionELOC, p.MaxDirectoryELOC),
			})
		}
		if stat.ProductionComplexity > p.MaxDirectoryComplexity {
			report.addFinding(Finding{
				Rule:    "directory_complexity",
				Path:    stat.Path,
				Count:   stat.ProductionComplexity,
				Limit:   p.MaxDirectoryComplexity,
				Message: fmt.Sprintf("directory decision complexity is %d; limit is %d", stat.ProductionComplexity, p.MaxDirectoryComplexity),
			})
		}
	}
}

func checkFlatDirectories(stats []DirectoryStats, p policy, report *Report) {
	for _, stat := range stats {
		// Always keep directory stats on the report; only production count gates.
		if stat.ProductionCount <= p.FlatDirectoryLimit {
			continue
		}
		files := append([]string(nil), stat.ProductionFiles...)
		report.FlatDirectories = append(report.FlatDirectories, stat)
		report.addFinding(Finding{
			Rule:    "flat_directory",
			Path:    stat.Path,
			Message: fmt.Sprintf("directory has %d production .go files; limit is %d (test files=%d, reported separately)", stat.ProductionCount, p.FlatDirectoryLimit, stat.TestCount),
			Files:   files,
			Count:   stat.ProductionCount,
			Limit:   p.FlatDirectoryLimit,
		})
	}
}

func checkSuffixFlat(files []goFileInfo, p policy, report *Report) {
	platform := p.platformSuffixSet()
	// dir -> prefix -> set of semantic keys + example files
	type group struct {
		keys  map[string]struct{}
		files map[string]struct{}
	}
	grouped := map[string]map[string]*group{}

	for _, file := range files {
		if file.isTest {
			continue
		}
		stem := strings.TrimSuffix(file.name, ".go")
		semantic := collapsePlatformSuffixes(stem, platform)
		if semantic == "" || strings.HasPrefix(semantic, "_") || !strings.Contains(semantic, "_") {
			continue
		}
		prefix, _, _ := strings.Cut(semantic, "_")
		if !isIdentPrefix(prefix) {
			continue
		}
		byPrefix := grouped[file.dir]
		if byPrefix == nil {
			byPrefix = map[string]*group{}
			grouped[file.dir] = byPrefix
		}
		g := byPrefix[prefix]
		if g == nil {
			g = &group{keys: map[string]struct{}{}, files: map[string]struct{}{}}
			byPrefix[prefix] = g
		}
		g.keys[semantic] = struct{}{}
		g.files[file.name] = struct{}{}
	}

	dirs := make([]string, 0, len(grouped))
	for dir := range grouped {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		prefixes := make([]string, 0, len(grouped[dir]))
		for prefix := range grouped[dir] {
			prefixes = append(prefixes, prefix)
		}
		sort.Strings(prefixes)
		for _, prefix := range prefixes {
			g := grouped[dir][prefix]
			if len(g.keys) < p.SuffixFlatGroupMin {
				continue
			}
			fileList := make([]string, 0, len(g.files))
			for name := range g.files {
				fileList = append(fileList, name)
			}
			sort.Strings(fileList)
			report.addFinding(Finding{
				Rule:    "suffix_flat",
				Path:    dir,
				Prefix:  prefix,
				Files:   fileList,
				Count:   len(g.keys),
				Limit:   p.SuffixFlatGroupMin,
				Message: fmt.Sprintf("suffix-flat group %q has %d semantic modules (threshold %d); prefer a %s/ subpackage", prefix, len(g.keys), p.SuffixFlatGroupMin, prefix),
			})
		}
	}
}

func collapsePlatformSuffixes(stem string, platform map[string]struct{}) string {
	// Strip trailing _<platform> segments only. Do not strip arbitrary suffixes.
	for {
		idx := strings.LastIndex(stem, "_")
		if idx <= 0 {
			return stem
		}
		suffix := stem[idx+1:]
		if _, ok := platform[suffix]; !ok {
			return stem
		}
		stem = stem[:idx]
	}
}

func isTestGoFile(name string) bool {
	return strings.HasSuffix(name, "_test.go")
}

func isIdentPrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	for i, r := range prefix {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
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
