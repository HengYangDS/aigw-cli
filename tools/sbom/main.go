package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type module struct {
	Path    string  `json:"Path"`
	Version string  `json:"Version"`
	Replace *module `json:"Replace"`
}

type spdxPackage struct {
	Name             string `json:"name"`
	SPDXID           string `json:"SPDXID"`
	VersionInfo      string `json:"versionInfo,omitempty"`
	DownloadLocation string `json:"downloadLocation"`
	FilesAnalyzed    bool   `json:"filesAnalyzed"`
	LicenseConcluded string `json:"licenseConcluded"`
	LicenseDeclared  string `json:"licenseDeclared"`
}

var runGoList = func() ([]byte, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	return cmd.CombinedOutput()
}

func loadModules() ([]module, error) {
	data, err := runGoList()
	if err != nil {
		detail := strings.TrimSpace(string(data))
		if detail == "" {
			return nil, fmt.Errorf("run go list -m -json all: %w", err)
		}
		return nil, fmt.Errorf("run go list -m -json all: %w\n%s", err, detail)
	}

	decoder := json.NewDecoder(strings.NewReader(string(data)))
	modules := []module{}
	for {
		var item module
		if err := decoder.Decode(&item); err == io.EOF {
			return modules, nil
		} else if err != nil {
			return nil, fmt.Errorf("decode go list module metadata: %w", err)
		}
		modules = append(modules, item)
	}
}

func main() {
	version := flag.String("version", "dev", "AIGW version")
	flag.Parse()
	modules, err := loadModules()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	packages := []spdxPackage{{Name: "aigw", SPDXID: "SPDXRef-Package-aigw", VersionInfo: *version, DownloadLocation: "NOASSERTION", FilesAnalyzed: false, LicenseConcluded: "MIT", LicenseDeclared: "MIT"}}
	index := 0
	for _, item := range modules {
		if item.Version == "" {
			continue
		}
		index++
		packages = append(packages, spdxPackage{Name: item.Path, SPDXID: fmt.Sprintf("SPDXRef-Dependency-%d", index), VersionInfo: item.Version, DownloadLocation: "NOASSERTION", FilesAnalyzed: false, LicenseConcluded: "NOASSERTION", LicenseDeclared: "NOASSERTION"})
	}
	doc := map[string]any{
		"spdxVersion": "SPDX-2.3", "dataLicense": "CC0-1.0", "SPDXID": "SPDXRef-DOCUMENT",
		"name": "aigw-" + *version, "documentNamespace": "https://aigw.internal/spdx/" + *version,
		"creationInfo": map[string]any{"created": time.Now().UTC().Format(time.RFC3339), "creators": []string{"Tool: aigw-sbom"}},
		"packages":     packages,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(doc); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
