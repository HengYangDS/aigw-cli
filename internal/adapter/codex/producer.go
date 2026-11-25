package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"aigw-cli/internal/adapter/discovery"
)

// codexManifestLayouts are the places where Codex's package manifest,
// codex-package.json, lies relative to the directory of the program file
// that it describes, once links are followed, in the order in which they are
// tried. Each was read on 2026-11-25.
var codexManifestLayouts = []string{
	// An editor extension: bin/<os>-<arch>/codex beside its manifest.
	"codex-package.json",
	// Homebrew's cask (<prefix>/Caskroom/codex/<version>/bin/codex) and
	// ChatGPT.app's Contents/Resources/codex-cli/bin/codex: the manifest is
	// in the package directory, one level above bin.
	filepath.Join("..", "codex-package.json"),
}

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

// printsVersion reports whether printed, a version as Codex's --version
// prints it, names version as a package manifest declares it. Codex prints
// `codex-cli <version>`: measured on 2026-11-25 for 0.171.0-alpha.13.2 and
// 0.172.0, whose manifests declare the bare version.
func printsVersion(printed, version string) bool {
	fields := strings.Fields(printed)
	return len(fields) > 0 && fields[len(fields)-1] == version
}

// producerProblem compares the program that produced the model table that
// state records with program, the configured one, without running either: by
// path, by the SHA-256 of its file and by the version that its package
// manifest declares, when one is readable. It is empty when they agree and
// when state records no table. A program that is not there is reported by
// the client's own check of its program, so it is not compared here.
func producerProblem(program string, state codexState) string {
	if state.CatalogHash == "" {
		return ""
	}
	if available, err := discovery.ExecutableAvailable(program); err != nil || !available {
		return ""
	}
	switch {
	case state.CatalogClientPath == "":
		return "AIGW's record does not name the executable that produced Codex's model table"
	case state.CatalogClientPath != program:
		return "Codex's model table came from another executable than the configured one"
	}
	digest, err := fileSHA256(program)
	switch {
	case err != nil:
		return "Codex's configured executable cannot be read to compare it with the one that produced Codex's model table"
	case digest != state.CatalogClientSHA256:
		return "Codex's configured executable changed after it produced Codex's model table"
	}
	if version := packageVersion(program); version != "" && !printsVersion(state.CatalogClientVersion, version) {
		return "Codex's configured executable is now version " + version + ", and Codex's model table came from " + state.CatalogClientVersion
	}
	return ""
}
