package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// maxLayerRows bounds the layer table, so that it states a direction instead
// of listing edges.
const maxLayerRows = 12

// layerTable is the policy's ordered stack of rows of package prefixes, from
// the top, with the seams and floors that refine it for a subtree.
type layerTable struct {
	rows   [][]string
	seams  map[string][]string
	floors map[string]string
}

// row returns the index of the row whose prefix is the longest that contains
// pkg, and that prefix; a package that no prefix contains has row -1.
func (table layerTable) row(pkg string) (int, string) {
	index, matched := -1, ""
	for i, prefixes := range table.rows {
		for _, prefix := range prefixes {
			if pathContains(prefix, pkg) && len(prefix) > len(matched) {
				index, matched = i, prefix
			}
		}
	}
	return index, matched
}

// seam returns the subtree whose declared seams contain target, for a source
// inside that subtree.
func (table layerTable) seam(source, target string) (string, bool) {
	for _, subtree := range slices.Sorted(maps.Keys(table.seams)) {
		if pathContains(subtree, source) && slices.Contains(table.seams[subtree], target) {
			return subtree, true
		}
	}
	return "", false
}

// floorAbove returns the subtree and floor that refuse target to source: the
// subtree contains source but not target, and its floor's row is below
// target's.
func (table layerTable) floorAbove(source, target string) (string, string, bool) {
	targetRow, _ := table.row(target)
	for _, subtree := range slices.Sorted(maps.Keys(table.floors)) {
		floor := table.floors[subtree]
		if floorRow, _ := table.row(floor); pathContains(subtree, source) && !pathContains(subtree, target) && targetRow < floorRow {
			return subtree, floor, true
		}
	}
	return "", "", false
}

// admits reports whether a production file of source may import target: one
// of its descendants, a seam of its subtree, or a package of a lower row that
// no floor of source's subtrees refuses.
func (table layerTable) admits(source, target string) bool {
	if pathContains(source, target) {
		return true
	}
	if _, ok := table.seam(source, target); ok {
		return true
	}
	sourceRow, _ := table.row(source)
	targetRow, _ := table.row(target)
	if targetRow <= sourceRow {
		return false
	}
	_, _, refused := table.floorAbove(source, target)
	return !refused
}

// rowName describes a row for a finding; row -1 places no package.
func rowName(row int) string {
	if row < 0 {
		return "which no row places"
	}
	return fmt.Sprintf("in row %d", row+1)
}

// checkLayers checks every module import of every production file against the
// layer table. It reports a production package that no row places, an import
// that the table refuses, and a prefix, seam or floor that nothing uses.
func checkLayers(root string, files []goFileInfo, p policy, report *Report) error {
	if len(p.Layers) == 0 {
		return nil
	}
	module, err := readModuleIdentity(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	table := layerTable{rows: p.Layers, seams: p.LayerSeams, floors: p.LayerFloors}
	use := layerUse{prefixes: map[string]bool{}, seams: map[string]bool{}, placed: map[string]bool{}, unplaced: map[string]bool{}}
	fset := token.NewFileSet()
	for _, file := range files {
		if file.isTest {
			continue
		}
		sourceRow, prefix := table.row(file.dir)
		if sourceRow < 0 {
			if !use.unplaced[file.dir] {
				use.unplaced[file.dir] = true
				report.addFinding(Finding{Rule: "layer_unplaced", Path: file.dir, Message: "production package has no row in the layer table"})
			}
			continue
		}
		use.prefixes[prefix] = true
		use.placed[file.dir] = true
		if err := table.checkImports(root, module, fset, file, use.seams, report); err != nil {
			return err
		}
	}
	use.report(p, report)
	return nil
}

// checkImports checks one production file's module imports against the table
// and records each seam that they use.
func (table layerTable) checkImports(root, module string, fset *token.FileSet, file goFileInfo, usedSeams map[string]bool, report *Report) error {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.relPath)))
	if err != nil {
		return fmt.Errorf("read %s: %w", file.relPath, err)
	}
	parsed, err := parser.ParseFile(fset, file.relPath, data, parser.ImportsOnly)
	if err != nil {
		return fmt.Errorf("parse imports %s: %w", file.relPath, err)
	}
	for _, imported := range parsed.Imports {
		importPath, _ := strconv.Unquote(imported.Path.Value)
		target, ok := strings.CutPrefix(importPath, module+"/")
		if !ok {
			continue
		}
		if subtree, ok := table.seam(file.dir, target); ok && !pathContains(file.dir, target) {
			usedSeams[subtree+"\x00"+target] = true
		}
		if table.admits(file.dir, target) {
			continue
		}
		report.addFinding(Finding{
			Rule:    "layer_import",
			Path:    file.relPath,
			Line:    fset.Position(imported.Pos()).Line,
			Package: target,
			Message: table.refusal(file.dir, target),
		})
	}
	return nil
}

// refusal explains why the table refuses source's import of target.
func (table layerTable) refusal(source, target string) string {
	sourceRow, _ := table.row(source)
	targetRow, _ := table.row(target)
	if subtree, floor, ok := table.floorAbove(source, target); ok {
		return fmt.Sprintf("package %q in row %d of the layer table may not import %q %s; packages of %q import outside it only from the row of %q down", source, sourceRow+1, target, rowName(targetRow), subtree, floor)
	}
	return fmt.Sprintf("package %q in row %d of the layer table may not import %q %s; import a lower row, a descendant or a declared seam", source, sourceRow+1, target, rowName(targetRow))
}

// layerUse records which prefixes, seams and packages the production files
// use, and which packages no row places.
type layerUse struct {
	prefixes, seams, placed, unplaced map[string]bool
}

// report adds a finding for each prefix, seam or floor that nothing uses.
func (use layerUse) report(p policy, report *Report) {
	for _, row := range p.Layers {
		for _, prefix := range row {
			if !use.prefixes[prefix] {
				report.addFinding(Finding{Rule: "unused_layer_prefix", Path: prefix, Message: "layer prefix places no production package; delete it"})
			}
		}
	}
	for _, subtree := range slices.Sorted(maps.Keys(p.LayerSeams)) {
		for _, seam := range p.LayerSeams[subtree] {
			if !use.seams[subtree+"\x00"+seam] {
				report.addFinding(Finding{Rule: "unused_layer_seam", Path: subtree, Package: seam, Message: fmt.Sprintf("no production package of %q imports its seam %q; delete it", subtree, seam)})
			}
		}
	}
	for _, subtree := range slices.Sorted(maps.Keys(p.LayerFloors)) {
		used := false
		for dir := range use.placed {
			used = used || pathContains(subtree, dir)
		}
		if !used {
			report.addFinding(Finding{Rule: "unused_layer_floor", Path: subtree, Message: fmt.Sprintf("the floor of %q holds no production package; delete it", subtree)})
		}
	}
}

// validateLayers admits at most maxLayerRows non-empty rows of distinct portable
// prefixes. A seam lies inside the placed subtree that declares it and never in
// a row above that subtree; a floor names a prefix of a row below its placed
// subtree.
func validateLayers(p policy) error {
	if len(p.Layers) > maxLayerRows {
		return fmt.Errorf("layers holds at most %d rows, not %d", maxLayerRows, len(p.Layers))
	}
	if len(p.Layers) == 0 && (len(p.LayerSeams) > 0 || len(p.LayerFloors) > 0) {
		return fmt.Errorf("layer_seams and layer_floors refine layers, which is empty")
	}
	prefixes := map[string]bool{}
	for _, row := range p.Layers {
		if len(row) == 0 {
			return fmt.Errorf("layers rows must be non-empty")
		}
		for _, prefix := range row {
			if !isPortableRelativePath(prefix) || prefixes[prefix] {
				return fmt.Errorf("layers has an invalid or repeated prefix %q", prefix)
			}
			prefixes[prefix] = true
		}
	}
	table := layerTable{rows: p.Layers}
	for _, subtree := range slices.Sorted(maps.Keys(p.LayerSeams)) {
		subtreeRow, _ := table.row(subtree)
		if subtreeRow < 0 {
			return fmt.Errorf("layer_seams subtree %q has no row in layers", subtree)
		}
		for _, seam := range p.LayerSeams[subtree] {
			if seamRow, _ := table.row(seam); !pathContains(subtree, seam) || seamRow < subtreeRow {
				return fmt.Errorf("layer_seams of %q may name only its own subtree's packages at or below its row, not %q", subtree, seam)
			}
		}
	}
	for _, subtree := range slices.Sorted(maps.Keys(p.LayerFloors)) {
		floor := p.LayerFloors[subtree]
		subtreeRow, _ := table.row(subtree)
		switch floorRow, _ := table.row(floor); {
		case !isPortableRelativePath(subtree) || subtreeRow < 0:
			return fmt.Errorf("layer_floors subtree %q has no row in layers", subtree)
		case !prefixes[floor]:
			return fmt.Errorf("layer_floors of %q must name a prefix of layers, not %q", subtree, floor)
		case floorRow <= subtreeRow:
			return fmt.Errorf("layer_floors of %q must name a row below the subtree's own row, not %q", subtree, floor)
		}
	}
	return nil
}
