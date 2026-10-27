package artifact

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// ScannerTool is the Mise tool whose locked release image produces the
// dependency reports.
const ScannerTool = "github:google/osv-scanner"

// Scanner names the scanner image behind a dependency report and the arguments
// it ran with. The arguments write the scanned source and the scan's evidence
// directory as {source} and {evidence}, so they do not depend on the host.
type Scanner struct {
	Tool      string   `json:"tool"`
	Version   string   `json:"version"`
	Platform  string   `json:"platform"`
	SHA256    string   `json:"sha256"`
	Arguments []string `json:"arguments"`
}

// LockedImage is the release asset that a Mise lock selects for a tool on one
// platform.
type LockedImage struct {
	Version, Platform, URL, SHA256 string
}

// misePlatformWords maps Go's operating system and architecture names to the
// words of Mise lock platforms; the other names are shared.
var misePlatformWords = map[string]string{"darwin": "macos", "amd64": "x64"}

// HostPlatform names the running host as Mise lock platforms do.
func HostPlatform() string {
	return platformName(runtime.GOOS, runtime.GOARCH)
}

func platformName(goos, goarch string) string {
	return cmp.Or(misePlatformWords[goos], goos) + "-" + cmp.Or(misePlatformWords[goarch], goarch)
}

// LockedScanner reads the scanner asset that a Mise lock selects for a platform.
func LockedScanner(lock []byte, platform string) (LockedImage, error) {
	var document struct {
		Tools map[string][]map[string]any `toml:"tools"`
	}
	if err := toml.Unmarshal(lock, &document); err != nil {
		return LockedImage{}, fmt.Errorf("decode Mise lock: %w", err)
	}
	selected := document.Tools[ScannerTool]
	if len(selected) != 1 {
		return LockedImage{}, fmt.Errorf("the Mise lock selects %d versions of %s, not one", len(selected), ScannerTool)
	}
	version, _ := selected[0]["version"].(string)
	asset, _ := selected[0]["platforms."+platform].(map[string]any)
	checksum, _ := asset["checksum"].(string)
	url, _ := asset["url"].(string)
	digest, hashed := strings.CutPrefix(checksum, "sha256:")
	if version == "" || url == "" || !hashed || !isSHA256(digest) {
		return LockedImage{}, fmt.Errorf("the Mise lock has no complete %s asset for %s", ScannerTool, platform)
	}
	return LockedImage{Version: version, Platform: platform, URL: url, SHA256: digest}, nil
}

func isSHA256(text string) bool {
	decoded, err := hex.DecodeString(text)
	return err == nil && len(decoded) == sha256.Size && text == strings.ToLower(text)
}

// scannerDependency returns the locked scanner asset that produced a
// candidate's dependency reports. Schema 1 reports predate the scanner binding;
// they name no scanner and add no dependency.
func scannerDependency(readSource func(string) ([]byte, error), lock, candidate, version string) ([]resourceDescriptor, error) {
	var named []*Scanner
	for _, report := range []string{"vulnerabilities", "licenses"} {
		scanner, err := reportScanner(filepath.Join(candidate, "aigw_"+version+"."+report+".json"))
		if err != nil {
			return nil, err
		}
		named = append(named, scanner)
	}
	if !sameScanner(named[0], named[1]) {
		return nil, errors.New("the dependency reports name different scanners")
	}
	scanner := named[0]
	if scanner == nil {
		return nil, nil
	}
	data, err := readSource(lock)
	if err != nil {
		return nil, fmt.Errorf("read Mise lock: %w", err)
	}
	image, err := LockedScanner(data, scanner.Platform)
	if err != nil {
		return nil, err
	}
	if scanner.Tool != ScannerTool || scanner.Version != image.Version || scanner.SHA256 != image.SHA256 {
		return nil, fmt.Errorf("the dependency reports name %s %s %s, not the image that the Mise lock selects for %s", scanner.Tool, scanner.Version, scanner.SHA256, scanner.Platform)
	}
	return []resourceDescriptor{{URI: image.URL, Digest: map[string]string{"sha256": image.SHA256}}}, nil
}

// reportScanner reads the scanner that a dependency report names; a report
// names one exactly at schema 2.
func reportScanner(path string) (*Scanner, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read dependency report: %w", err)
	}
	var report struct {
		SchemaVersion int      `json:"schema_version"`
		Scanner       *Scanner `json:"scanner"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("decode dependency report %s: %w", filepath.Base(path), err)
	}
	if report.SchemaVersion != 1 && report.SchemaVersion != 2 || (report.SchemaVersion == 2) != (report.Scanner != nil) {
		return nil, fmt.Errorf("dependency report %s has schema %d, which names a scanner exactly at schema 2", filepath.Base(path), report.SchemaVersion)
	}
	return report.Scanner, nil
}

func sameScanner(left, right *Scanner) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Tool == right.Tool && left.Version == right.Version && left.Platform == right.Platform &&
		left.SHA256 == right.SHA256 && slices.Equal(left.Arguments, right.Arguments)
}
