package construction

import (
	"aigw-cli/tools/release/artifact"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// scannerName is the command that runs the locked dependency scanner.
const scannerName = "osv-scanner"

// dependencyInputs are the source's dependency policy and the scanner image
// that its Mise lock selects for this host.
type dependencyInputs struct {
	Policy  dependencyPolicy
	Scanner artifact.LockedImage
}

func readDependencyInputs(root string) (dependencyInputs, error) {
	policy, err := readDependencyPolicy(root)
	if err != nil {
		return dependencyInputs{}, err
	}
	lock, err := os.ReadFile(filepath.Join(root, ".config", "mise", "mise.lock"))
	if err != nil {
		return dependencyInputs{}, fmt.Errorf("read Mise lock: %w", err)
	}
	scanner, err := artifact.LockedScanner(lock, artifact.HostPlatform())
	return dependencyInputs{Policy: policy, Scanner: scanner}, err
}

// admitScanner runs the scanner's version report and admits only the image
// that the lock selects, reporting the locked version. It returns that image
// and its complete version report.
func admitScanner(root string, locked artifact.LockedImage, run toolRunner) (toolImage, string, error) {
	var report bytes.Buffer
	var image toolImage
	if err := run(toolCall{Name: scannerName, Directory: root, Args: []string{"--version"}, Stdout: &report, Image: &image, Timeout: time.Minute}); err != nil {
		return toolImage{}, "", fmt.Errorf("report dependency scanner version: %w", err)
	}
	if image.SHA256 != locked.SHA256 {
		return toolImage{}, "", fmt.Errorf("dependency scanner %s has SHA-256 %s, not the %s image %s that the Mise lock selects for %s",
			image.Path, image.SHA256, locked.Version, locked.SHA256, locked.Platform)
	}
	if version := reportedVersion(report.String()); version != locked.Version {
		return toolImage{}, "", fmt.Errorf("dependency scanner %s reports version %q, not the locked %s", image.Path, version, locked.Version)
	}
	return image, report.String(), nil
}

func reportedVersion(report string) string {
	for line := range strings.Lines(report) {
		if version, found := strings.CutPrefix(strings.TrimSpace(line), "osv-scanner version: "); found {
			return version
		}
	}
	return ""
}

// portableArguments writes the absolute evidence and source paths of a scan's
// arguments as {evidence} and {source}; the evidence directory may lie inside
// the source.
func portableArguments(args []string, source, evidence string) []string {
	portable := slices.Clone(args)
	for index, argument := range portable {
		for _, root := range [][2]string{{"{evidence}", evidence}, {"{source}", source}} {
			if relative, found := strings.CutPrefix(argument, root[1]+string(filepath.Separator)); found {
				portable[index] = root[0] + "/" + filepath.ToSlash(relative)
				break
			}
		}
	}
	return portable
}

// scanInvocation records the scanner process behind one raw dependency report.
type scanInvocation struct {
	Executable string   `json:"executable"`
	SHA256     string   `json:"sha256"`
	Platform   string   `json:"platform"`
	Version    string   `json:"version"`
	Arguments  []string `json:"arguments"`
	Report     string   `json:"report_sha256,omitempty"`
}

// recordScanInvocation retains a scan's exact invocation beside its raw report,
// whether or not the scan succeeded.
func recordScanInvocation(raw string, call toolCall, version, platform string) error {
	invocation := scanInvocation{Executable: call.Image.Path, SHA256: call.Image.SHA256, Platform: platform, Version: version, Arguments: call.Args}
	data, err := os.ReadFile(raw)
	if err == nil {
		invocation.Report = fmt.Sprintf("%x", sha256.Sum256(data))
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeJSON(raw+".invocation.json", invocation)
}
