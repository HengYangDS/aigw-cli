package construction

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// standardLibraryVersion is the Go release that builds the test programs, so
// dependency and SBOM fixtures report it for their standard library.
var standardLibraryVersion = strings.TrimPrefix(runtime.Version(), "go")

func TestDependencyPolicyScansTheGoStandardLibrary(t *testing.T) {
	policy, err := readDependencyPolicy(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	var config []byte
	if _, err := scanDependencies(root, filepath.Join(root, "evidence"), policy, func(call toolCall) error {
		var readErr error
		if config, readErr = os.ReadFile(call.Args[slices.Index(call.Args, "--config")+1]); readErr != nil {
			return readErr
		}
		return writeJSON(call.Args[len(call.Args)-1], dependencyReportFixture(root))
	}); err != nil {
		t.Fatal(err)
	}
	var native map[string]any
	if err := toml.Unmarshal(config, &native); err != nil || native["ScanGoModVersion"] != true {
		t.Fatalf("OSV configuration omits the Go standard library: %s, %v", config, err)
	}
}

func TestReleaseSBOMCarriesTheScannedStandardLibrary(t *testing.T) {
	for _, failure := range []string{"unscanned", "other version", "unnamed program", "one program", "foreign module"} {
		t.Run(failure, func(t *testing.T) {
			root := releaseRoot(t)
			for name, content := range map[string]string{
				"go.mod": "fixture", "go.sum": "fixture", "package-lock.json": "fixture",
				".config/mise/mise.lock": "fixture", "LICENSE": "fixture", "README.md": "fixture",
				".config/mise/config.toml": "[tools]\ngo = \"1.27.1\"\nosv-scanner = \"2.5.1\"\n",
			} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			output := filepath.Join(root, "dist")
			var calls []toolCall
			fault := ""
			runner := releaseFixtureRunner(t, root, &calls, &fault)
			err := buildRelease(t.Context(), buildRequest{Root: root, Output: output, Version: "1.2.3", Epoch: "1784246400", SigningKey: signingKey(t)}, func(call toolCall) error {
				if err := runner(call); err != nil {
					return err
				}
				switch {
				case call.Name == "osv-scanner" && failure == "unscanned":
					report := dependencyReportFixture(root)
					report.Results[0].Packages = slices.DeleteFunc(report.Results[0].Packages, func(item osvFixturePackage) bool { return item.Package.Name == "stdlib" })
					return writeJSON(call.Args[slices.Index(call.Args, "--output-file")+1], report)
				case call.Name == "syft" && failure != "unscanned":
					return rewriteStandardLibraries(strings.TrimPrefix(call.Args[len(call.Args)-1], "spdx-json="), failure)
				}
				return nil
			})
			signed := slices.ContainsFunc(calls, func(call toolCall) bool { return call.Name == "ssh-keygen" })
			if err == nil || !strings.Contains(err.Error(), "standard library") || signed {
				t.Fatalf("release evidence was accepted without the scanned standard library: error=%v signed=%t", err, signed)
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("refused release wrote its output: %v", err)
			}
		})
	}
}

func rewriteStandardLibraries(path, failure string) error {
	var document map[string]any
	if err := readJSON(path, &document); err != nil {
		return err
	}
	packages, _ := document["packages"].([]any)
	relationships, _ := document["relationships"].([]any)
	libraries := map[any]bool{}
	for _, item := range packages {
		if object, ok := item.(map[string]any); ok && object["name"] == "stdlib" {
			libraries[object["SPDXID"]] = true
			if failure == "other version" {
				object["versionInfo"] = "go1.0.0"
			}
		}
	}
	if failure == "unnamed program" {
		packages = slices.DeleteFunc(packages, func(item any) bool {
			object, _ := item.(map[string]any)
			return object["SPDXID"] == "SPDXRef-Package-stdlib-0"
		})
	}
	// "one program" keeps every library, file and relationship but attributes
	// each library to the first program; "foreign module" makes the second
	// program's library a dependency of the first program's main module.
	for _, item := range relationships {
		object, ok := item.(map[string]any)
		if !ok || !libraries[object["spdxElementId"]] {
			continue
		}
		dependency := object["relationshipType"] == "DEPENDENCY_OF"
		switch {
		case failure == "one program" && dependency:
			object["relatedSpdxElement"] = "SPDXRef-Package-aigw-cli-0"
		case failure == "one program":
			object["relatedSpdxElement"] = "SPDXRef-File-0"
		case failure == "foreign module" && dependency && object["spdxElementId"] == "SPDXRef-Package-stdlib-1":
			object["relatedSpdxElement"] = "SPDXRef-Package-aigw-cli-0"
		}
	}
	document["packages"] = packages
	return writeJSON(path, document)
}
