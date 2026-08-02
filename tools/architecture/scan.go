package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
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

type goFileInfo struct {
	relPath    string
	name       string
	dir        string
	isTest     bool
	eloc       int
	complexity int
}

func analyzeRepository(root string, p policy, policyPath string) (Report, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Report{}, fmt.Errorf("resolve root: %w", err)
	}
	report := newReport(policyPath, absRoot)

	if err := checkScriptsRoots(absRoot, p, &report); err != nil {
		return Report{}, err
	}
	goFiles, dirStats, err := collectGoFiles(absRoot, p)
	if err != nil {
		return Report{}, err
	}
	report.DirectoryStats = dirStats
	checkCompositionRoots(goFiles, p, &report)
	if err := checkPeerPackageImports(absRoot, goFiles, p, &report); err != nil {
		return Report{}, err
	}
	checkFlatDirectories(dirStats, p, &report)
	checkSourceBudgets(goFiles, dirStats, p, &report)
	checkSuffixFlat(goFiles, p, &report)
	if err := checkForbiddenNames(absRoot, p, &report); err != nil {
		return Report{}, err
	}
	if err := checkGoAST(absRoot, goFiles, p, &report); err != nil {
		return Report{}, err
	}
	return report, nil
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
				path, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					continue
				}
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

func checkScriptsRoots(root string, p policy, report *Report) error {
	for _, scriptsRoot := range p.ScriptsRoots {
		abs := filepath.Join(root, filepath.FromSlash(scriptsRoot))
		info, err := os.Stat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat scripts root %s: %w", scriptsRoot, err)
		}
		if !info.IsDir() {
			report.addFinding(Finding{
				Rule:    "scripts_root_not_directory",
				Path:    toPOSIX(scriptsRoot),
				Message: "scripts root must be a directory of semantic subdirectories",
			})
			continue
		}
		entries, err := os.ReadDir(abs)
		if err != nil {
			return fmt.Errorf("read scripts root %s: %w", scriptsRoot, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if shouldIgnoreDirName(name, p) {
				continue
			}
			rel := toPOSIX(filepath.Join(scriptsRoot, name))
			if entry.Type()&fs.ModeSymlink != 0 {
				// Resolve only enough to classify file vs directory.
				targetInfo, statErr := os.Stat(filepath.Join(abs, name))
				if statErr != nil {
					report.addFinding(Finding{
						Rule:    "scripts_root_file",
						Path:    rel,
						Message: "scripts root may not contain direct files; use semantic subdirectories only",
					})
					continue
				}
				if targetInfo.IsDir() {
					continue
				}
				report.addFinding(Finding{
					Rule:    "scripts_root_file",
					Path:    rel,
					Message: "scripts root may not contain direct files; use semantic subdirectories only",
				})
				continue
			}
			if entry.IsDir() {
				continue
			}
			report.addFinding(Finding{
				Rule:    "scripts_root_file",
				Path:    rel,
				Message: "scripts root may not contain direct files; use semantic subdirectories only",
			})
		}
	}
	return nil
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
			if !isTest {
				info.eloc, info.complexity, err = sourceMetrics(path)
				if err != nil {
					return fmt.Errorf("measure %s: %w", relPOSIX, err)
				}
			}
			files = append(files, info)
			if isTest {
				stat.TestCount++
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

func sourceMetrics(path string) (int, int, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}

	// The Go scanner omits comments by default. Counting the distinct source
	// lines occupied by real tokens therefore handles block comments, mixed
	// code/comment lines, and raw strings without a text-level approximation.
	lineSet := map[int]struct{}{}
	fset := token.NewFileSet()
	file := fset.AddFile(path, fset.Base(), len(src))
	var lexical scanner.Scanner
	lexical.Init(file, src, nil, 0)
	for {
		pos, tok, _ := lexical.Scan()
		if tok == token.EOF {
			break
		}
		lineSet[fset.Position(pos).Line] = struct{}{}
	}

	parsed, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
	if err != nil {
		// Syntax errors are reported as path-addressable go_parse_error findings
		// by checkGoAST. Preserve the source-size measurement here so malformed
		// files do not disappear from directory totals.
		return len(lineSet), 0, nil
	}
	complexity := 0
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			complexity++
		case *ast.BinaryExpr:
			if typed.Op == token.LAND || typed.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	return len(lineSet), complexity, nil
}

func checkSourceBudgets(files []goFileInfo, stats []DirectoryStats, p policy, report *Report) {
	for _, file := range files {
		if file.isTest {
			continue
		}
		if file.eloc > p.MaxFileELOC {
			report.addFinding(Finding{
				Rule:    "file_eloc",
				Path:    file.relPath,
				Count:   file.eloc,
				Limit:   p.MaxFileELOC,
				Message: fmt.Sprintf("file has %d effective lines; limit is %d", file.eloc, p.MaxFileELOC),
			})
		}
		if file.complexity > p.MaxFileComplexity {
			report.addFinding(Finding{
				Rule:    "file_complexity",
				Path:    file.relPath,
				Count:   file.complexity,
				Limit:   p.MaxFileComplexity,
				Message: fmt.Sprintf("file decision complexity is %d; limit is %d", file.complexity, p.MaxFileComplexity),
			})
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
		prefix, _, ok := strings.Cut(semantic, "_")
		if !ok || prefix == "" {
			continue
		}
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

func checkForbiddenNames(root string, p policy, report *Report) error {
	forbidden := p.forbiddenNameSet()
	allowed := p.allowedForbiddenNameSet()

	// Walk managed go roots and scripts roots for directory names.
	var roots []string
	roots = append(roots, p.GoRoots...)
	roots = append(roots, p.ScriptsRoots...)
	seen := map[string]struct{}{}
	for _, relRoot := range roots {
		abs := filepath.Join(root, filepath.FromSlash(relRoot))
		info, err := os.Stat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat %s: %w", relRoot, err)
		}
		if !info.IsDir() {
			continue
		}
		err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !d.IsDir() {
				return nil
			}
			name := d.Name()
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relPOSIX := toPOSIX(rel)
			if path != abs {
				if shouldIgnoreDirName(name, p) || shouldIgnoreRelPath(relPOSIX, p) {
					return fs.SkipDir
				}
			}
			lower := strings.ToLower(name)
			if _, ok := forbidden[lower]; !ok {
				return nil
			}
			if _, ok := allowed[lower]; ok {
				return nil
			}
			// Dedup exact path.
			if _, ok := seen[relPOSIX]; ok {
				return nil
			}
			seen[relPOSIX] = struct{}{}
			report.addFinding(Finding{
				Rule:    "forbidden_name",
				Path:    relPOSIX,
				Name:    lower,
				Message: fmt.Sprintf("permanent directory/package name %q is forbidden", lower),
			})
			return nil
		})
		if err != nil {
			return fmt.Errorf("walk forbidden names under %s: %w", relRoot, err)
		}
	}
	return nil
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
	if len(parts) == 0 {
		return false
	}
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
