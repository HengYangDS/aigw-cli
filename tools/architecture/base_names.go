package main

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
)

// checkPackageBaseNames refuses two production packages that share a base
// name, so that an import path's last element names one owner. A directory
// that holds only test files is a fixture, as in the layer table.
func checkPackageBaseNames(files []goFileInfo, p policy, report *Report) {
	if !p.UniquePackageBaseNames {
		return
	}
	owners := map[string][]string{}
	for _, file := range files {
		base := path.Base(file.dir)
		if file.isTest || slices.Contains(owners[base], file.dir) {
			continue
		}
		owners[base] = append(owners[base], file.dir)
	}
	for _, base := range slices.Sorted(maps.Keys(owners)) {
		dirs := owners[base]
		if len(dirs) < 2 {
			continue
		}
		slices.Sort(dirs)
		report.addFinding(Finding{
			Rule:    "package_base_name",
			Path:    dirs[0],
			Name:    base,
			Files:   dirs,
			Message: fmt.Sprintf("packages %s share the base name %q; name each after what it owns", strings.Join(dirs, ", "), base),
		})
	}
}
