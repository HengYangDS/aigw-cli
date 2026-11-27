package codex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/codex/modelcatalog"
	"aigw-cli/internal/adapter/discovery"
)

// codexManifestLayouts are the places where Codex's package manifest,
// codex-package.json, lies relative to the directory of the program file
// that it describes, once links are followed. Each was read on an
// installation on 2026-11-25: Homebrew's cask
// (<prefix>/Caskroom/codex/<version>/bin/codex) and ChatGPT.app's
// Contents/Resources/codex-cli/bin/codex keep it in the package directory,
// one level above bin. A layout read on no installation is not declared.
var codexManifestLayouts = []string{filepath.Join("..", "codex-package.json")}

// packageVersion returns the version that the package manifest of the program
// at path declares, or nothing when no manifest of one of the layouts is
// readable. It runs no program.
func packageVersion(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return ""
	}
	for _, layout := range codexManifestLayouts {
		data, err := os.ReadFile(filepath.Join(filepath.Dir(resolved), layout))
		if err != nil {
			continue
		}
		var manifest struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(data, &manifest) == nil && manifest.Version != "" {
			return manifest.Version
		}
	}
	return ""
}

// tableFix is the one fix of a Codex whose program, declared by its path or
// found by discovery, cannot produce its model table: another program, since
// a sync runs the same one.
const tableFix = "Name a Codex program that prints its model table with `debug models --bundled`: aigw client edit codex --program <path>"

// extensionTableFix is the one fix of a Codex whose program an extensions
// directory resolves to and cannot produce its model table: another version
// of the extension, to which the directory then resolves, or a program outside
// the directory, since AIGW refuses a program inside one of its versions.
const extensionTableFix = "Update or reinstall the Codex extension in the editor, so that the program of its newest installed version prints its model table with `debug models --bundled`, or name a Codex program outside the extensions directory: aigw client edit codex --program <path>"

// tableRepair is the one fix of a Codex whose program cannot produce its model
// table: extensionTableFix for a program inside an installed version of
// Codex's extension, which only an extensions directory resolves to, and
// tableFix for any other.
func tableRepair(program string) string {
	if _, extension := (codexAdapter{}).Spec().ExtensionsDirectoryOf(program); extension {
		return extensionTableFix
	}
	return tableFix
}

// tableStop is the stop of a Codex whose program cannot produce the model
// table that AIGW owns for it, from what the read-only probes of live, its
// identity, err, their failure, and parseErr, the table's, found. The problem
// names which probe failed and no path, which a local check never shows; the
// failure stays its cause. The plan of a sync and a local check give it alike.
func tableStop(program string, live ExecutableIdentity, err, parseErr error) *adapter.StopError {
	problem := "Codex's program cannot produce its model table: "
	switch {
	case !live.known():
		problem += "it reports no version"
	case err != nil:
		problem += "`debug models --bundled` fails"
	default:
		problem += "`debug models --bundled` prints no model table that AIGW can read"
	}
	return adapter.NewStop(ID, problem, tableRepair(program), errors.Join(err, parseErr))
}

// producerProblem compares the program that produced the model table that
// state records with program, the one that Codex uses now, without running
// either: by path, by the SHA-256 of its file and by the version that its
// package manifest declares, against the manifest's version that the record
// keeps; a program without a manifest is compared by path and digest alone.
// It is empty when they agree and when state records no table. A program that
// is not there is reported by the client's own check of its program, so it is
// not compared here.
func producerProblem(program string, state codexState) string {
	if state.CatalogHash == "" {
		return ""
	}
	if available, err := discovery.ExecutableAvailable(program); err != nil || !available {
		return ""
	}
	switch {
	case state.CatalogClientPath == "":
		return "AIGW's record does not name the program that produced Codex's model table"
	case state.CatalogClientPath != program:
		return "Codex's model table came from another program than the one that Codex uses now"
	}
	digest, err := fileSHA256(program)
	switch {
	case err != nil:
		return "Codex's program cannot be read to compare it with the one that produced Codex's model table"
	case digest != state.CatalogClientSHA256:
		return "Codex's program changed after it produced Codex's model table"
	}
	if version := packageVersion(program); version != state.CatalogClientPackage {
		return "The package of Codex's program now declares " + declared(version) + ", and Codex's model table came from " + produced(state.CatalogClientPackage)
	}
	return ""
}

// declared names the version that a package manifest declares now.
func declared(version string) string {
	if version == "" {
		return "no version"
	}
	return "version " + version
}

// produced names the package version that the table's producer declared.
func produced(version string) string {
	if version == "" {
		return "a program whose package declared no version"
	}
	return "version " + version
}

// producerRepair is the problem and fix of a model table whose producer
// differs from Codex's program: a sync produces another, unless the program
// cannot, which the same read-only probes as a sync's plan tell, and then the
// client is stopped, as the sync would stop it.
func producerRepair(program, problem string) (string, string) {
	live, bundled, err := codexBundledCatalog(program)
	if _, parseErr := modelcatalog.Parse(bundled); err != nil || parseErr != nil {
		stop := tableStop(program, live, err, parseErr)
		return stop.Error(), stop.NextAction()
	}
	return problem, "aigw sync"
}
