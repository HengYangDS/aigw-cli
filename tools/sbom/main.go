package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
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

func runGoList() ([]byte, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	return cmd.CombinedOutput()
}

func creationTimeFromEnv(getenv func(string) string) (time.Time, error) {
	raw := strings.TrimSpace(getenv("SOURCE_DATE_EPOCH"))
	if raw == "" {
		return time.Time{}, fmt.Errorf("SOURCE_DATE_EPOCH must be set to a non-negative Unix epoch")
	}
	epoch, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || epoch < 0 {
		return time.Time{}, fmt.Errorf("SOURCE_DATE_EPOCH must be a non-negative Unix epoch")
	}
	return time.Unix(epoch, 0).UTC(), nil
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
	return decodeModules(data)
}

func decodeModules(data []byte) ([]module, error) {
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
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("sbom", flag.ContinueOnError)
	flags.SetOutput(stderr)
	version := flags.String("version", "dev", "AIGW version")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	created, err := creationTimeFromEnv(getenv)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	modules, err := loadModules()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if err := writeSBOM(stdout, *version, created, modules); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func writeSBOM(output io.Writer, version string, created time.Time, modules []module) error {
	namespace := documentNamespace(version, created, modules)
	packages := []spdxPackage{{Name: "aigw", SPDXID: "SPDXRef-Package-aigw", VersionInfo: version, DownloadLocation: "NOASSERTION", FilesAnalyzed: false, LicenseConcluded: "MIT", LicenseDeclared: "MIT"}}
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
		"name": "aigw-" + version, "documentNamespace": namespace,
		"creationInfo": map[string]any{"created": created.Format(time.RFC3339), "creators": []string{"Tool: aigw-sbom"}},
		"packages":     packages,
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(doc); err != nil {
		return err
	}
	return nil
}

func documentNamespace(version string, created time.Time, modules []module) string {
	records := make([]string, 0, len(modules))
	for _, item := range modules {
		record := item.Path + "\x00" + item.Version
		if item.Replace != nil {
			record += "\x00" + item.Replace.Path + "\x00" + item.Replace.Version
		}
		records = append(records, record)
	}
	sort.Strings(records)
	identity := "aigw-sbom\x00" + version + "\x00" + strconv.FormatInt(created.Unix(), 10) + "\x00" + strings.Join(records, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return fmt.Sprintf("urn:sha256:%x", digest)
}
