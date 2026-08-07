// Command releasekit owns deterministic package metadata and GitLab release validation.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aigw-cli/internal/selfupdate"
)

type releaseLink struct {
	Name            string `json:"name"`
	URL             string `json:"url"`
	DirectAssetPath string `json:"direct_asset_path"`
}

type releaseAssets struct {
	Links []releaseLink `json:"links"`
}

type releasePayload struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Assets      releaseAssets `json:"assets"`
}

type remoteRelease struct {
	TagName string `json:"tag_name"`
	Assets  struct {
		Links []struct {
			URL string `json:"url"`
		} `json:"links"`
	} `json:"assets"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: releasekit <validate-release-sources|touch-timestamp|msi-metadata|write-gitlab-release|same-authority|resolve-redirect|verify-gitlab-release|project-gitlab-response|validate-gitlab-release|write-candidate-manifest|validate-candidate-manifest>")
	}
	switch args[0] {
	case "validate-release-sources":
		if len(args) != 1 {
			return errors.New("usage: releasekit validate-release-sources")
		}
		return selfupdate.ValidateBuildReleaseSources()
	case "touch-timestamp":
		if len(args) != 2 {
			return errors.New("usage: releasekit touch-timestamp <unix-epoch>")
		}
		value, err := touchTimestamp(args[1])
		if err == nil {
			_, _ = fmt.Fprintln(stdout, value)
		}
		return err
	case "msi-metadata":
		flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		environment := flags.String("environment", "", "Environment IDT output")
		summary := flags.String("summary", "", "summary IDT input/output")
		environmentGUID := flags.String("environment-guid", "", "environment row GUID")
		packageGUID := flags.String("package-guid", "", "package GUID")
		epoch := flags.String("epoch", "", "Unix epoch")
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || *environment == "" || *summary == "" || *environmentGUID == "" || *packageGUID == "" {
			return errors.New("usage: releasekit msi-metadata -environment <path> -summary <path> -environment-guid <guid> -package-guid <guid> -epoch <unix-epoch>")
		}
		instant, err := parseEpoch(*epoch)
		if err != nil {
			return err
		}
		return writeMSIMetadata(*environment, *summary, *environmentGUID, *packageGUID, instant)
	case "write-gitlab-release":
		if len(args) != 4 {
			return errors.New("usage: releasekit write-gitlab-release <tag> <base-url> <output>")
		}
		return writeJSON(args[3], releaseDocument(args[1], args[2]))
	case "same-authority":
		if len(args) != 3 {
			return errors.New("usage: releasekit same-authority <left-url> <right-url>")
		}
		same, err := sameAuthority(args[1], args[2])
		if err != nil {
			return err
		}
		if same {
			_, _ = fmt.Fprintln(stdout, "yes")
		} else {
			_, _ = fmt.Fprintln(stdout, "no")
		}
		return nil
	case "resolve-redirect":
		if len(args) != 3 {
			return errors.New("usage: releasekit resolve-redirect <current-url> <headers-file>")
		}
		location, err := readLocation(args[2])
		if err != nil {
			return err
		}
		resolved, err := resolveRedirect(args[1], location)
		if err == nil {
			_, _ = fmt.Fprintln(stdout, resolved)
		}
		return err
	case "verify-gitlab-release":
		if len(args) != 5 {
			return errors.New("usage: releasekit verify-gitlab-release <expected-json> <actual-json> <asset-list> <tag>")
		}
		return verifyGitLabRelease(args[1], args[2], args[3], args[4])
	case "project-gitlab-response":
		if len(args) != 4 {
			return errors.New("usage: releasekit project-gitlab-response <release-json> <mode> <output>")
		}
		var expected releasePayload
		if err := readJSON(args[1], &expected); err != nil {
			return err
		}
		projected, err := projectGitLabResponse(expected, args[2])
		if err != nil {
			return err
		}
		return writeJSON(args[3], projected)
	case "validate-gitlab-release":
		if len(args) != 3 {
			return errors.New("usage: releasekit validate-gitlab-release <release-json> <tag>")
		}
		var payload releasePayload
		if err := readJSON(args[1], &payload); err != nil {
			return err
		}
		return validateReleaseDocument(payload, args[2])
	case "write-candidate-manifest":
		if len(args) != 8 {
			return errors.New("usage: releasekit write-candidate-manifest <output> <version> <commit> <tree> <created-utc> <checksums-sha256> <artifact-count>")
		}
		count, err := strconv.Atoi(args[7])
		if err != nil {
			return err
		}
		return writeJSON(args[1], candidateManifest{Schema: 1, Kind: "aigw-verified-candidate", Version: args[2], Commit: args[3], Tree: args[4], CreatedUTC: args[5], ArtifactsDir: "artifacts", ChecksumsPath: "artifacts/checksums.txt", ChecksumsSHA256: args[6], ArtifactCount: count})
	case "validate-candidate-manifest":
		if len(args) != 2 {
			return errors.New("usage: releasekit validate-candidate-manifest <candidate-json>")
		}
		var manifest candidateManifest
		if err := readJSON(args[1], &manifest); err != nil {
			return err
		}
		if err := validateCandidateManifest(manifest); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(stdout, manifest.Version)
		_, _ = fmt.Fprintln(stdout, manifest.ChecksumsSHA256)
		return nil
	default:
		return fmt.Errorf("unknown releasekit command: %s", args[0])
	}
}

type candidateManifest struct {
	Schema          int    `json:"schema"`
	Kind            string `json:"kind"`
	Version         string `json:"version"`
	Commit          string `json:"commit"`
	Tree            string `json:"tree"`
	CreatedUTC      string `json:"created_utc"`
	ArtifactsDir    string `json:"artifacts_dir"`
	ChecksumsPath   string `json:"checksums_path"`
	ChecksumsSHA256 string `json:"checksums_sha256"`
	ArtifactCount   int    `json:"artifact_count"`
}

func validateCandidateManifest(manifest candidateManifest) error {
	if manifest.Schema != 1 || manifest.Kind != "aigw-verified-candidate" || manifest.ArtifactsDir != "artifacts" || manifest.ChecksumsPath != "artifacts/checksums.txt" || manifest.ArtifactCount != 15 {
		return errors.New("candidate manifest has invalid fixed fields")
	}
	versionPattern := regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._-]*$`)
	hex40 := regexp.MustCompile(`^[0-9a-f]{40}$`)
	hex64 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if !versionPattern.MatchString(manifest.Version) {
		return errors.New("candidate manifest has invalid version")
	}
	if !hex40.MatchString(manifest.Commit) || !hex40.MatchString(manifest.Tree) {
		return errors.New("candidate manifest has invalid commit or tree")
	}
	if !hex64.MatchString(manifest.ChecksumsSHA256) {
		return errors.New("candidate manifest has invalid checksums_sha256")
	}
	return nil
}

func parseEpoch(raw string) (time.Time, error) {
	epoch, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || epoch < 0 {
		return time.Time{}, errors.New("epoch must be a non-negative integer")
	}
	return time.Unix(epoch, 0).UTC(), nil
}

func touchTimestamp(raw string) (string, error) {
	instant, err := parseEpoch(raw)
	if err != nil {
		return "", err
	}
	return instant.Format("200601021504.05"), nil
}

func authority(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return "", fmt.Errorf("invalid HTTP URL: %s", raw)
	}
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".") + ":" + port, nil
}

func sameAuthority(left, right string) (bool, error) {
	a, err := authority(left)
	if err != nil {
		return false, err
	}
	b, err := authority(right)
	return a == b, err
}

func resolveRedirect(current, location string) (string, error) {
	base, err := url.Parse(current)
	if err != nil {
		return "", err
	}
	next, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(next)
	if (resolved.Scheme != "http" && resolved.Scheme != "https") || resolved.Hostname() == "" || resolved.User != nil || resolved.Fragment != "" {
		return "", errors.New("unsafe redirect URL")
	}
	if base.Scheme == "https" && resolved.Scheme != "https" {
		return "", errors.New("redirect would downgrade HTTPS")
	}
	return resolved.String(), nil
}

func readLocation(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		name, value, found := strings.Cut(scanner.Text(), ":")
		if found && strings.EqualFold(strings.TrimSpace(name), "location") {
			location := strings.TrimSpace(value)
			if location != "" {
				return location, nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errors.New("redirect response has no Location header")
}

func releaseDocument(tag, base string) releasePayload {
	version := strings.TrimPrefix(tag, "v")
	entries := []struct{ name, label string }{
		{"aigw_" + version + "_darwin_universal.pkg", "macOS Universal pkg"},
		{"aigw_" + version + "_darwin_amd64.tar.gz", "macOS amd64 portable"},
		{"aigw_" + version + "_darwin_arm64.tar.gz", "macOS arm64 portable"},
		{"aigw_" + version + "_linux_amd64.deb", "Linux amd64 deb"},
		{"aigw_" + version + "_linux_arm64.deb", "Linux arm64 deb"},
		{"aigw_" + version + "_linux_amd64.rpm", "Linux amd64 rpm"},
		{"aigw_" + version + "_linux_arm64.rpm", "Linux arm64 rpm"},
		{"aigw_" + version + "_linux_amd64.tar.gz", "Linux amd64 portable"},
		{"aigw_" + version + "_linux_arm64.tar.gz", "Linux arm64 portable"},
		{"aigw_" + version + "_windows_amd64.msi", "Windows amd64 msi"},
		{"aigw_" + version + "_windows_arm64.msi", "Windows arm64 msi"},
		{"aigw_" + version + "_windows_amd64.zip", "Windows amd64 portable"},
		{"aigw_" + version + "_windows_arm64.zip", "Windows arm64 portable"},
		{"checksums.txt", "SHA-256 checksums"},
		{"aigw_" + version + ".spdx.json", "SPDX SBOM"},
	}
	payload := releasePayload{TagName: tag, Name: "AIGW " + tag, Description: "Cross-platform AIGW CLI release. Verify downloads with checksums.txt."}
	for _, entry := range entries {
		payload.Assets.Links = append(payload.Assets.Links, releaseLink{Name: entry.label, URL: strings.TrimSuffix(base, "/") + "/" + entry.name, DirectAssetPath: "/" + entry.name})
	}
	return payload
}

func projectGitLabResponse(expected releasePayload, mode string) (remoteRelease, error) {
	result := remoteRelease{TagName: expected.TagName}
	for _, link := range expected.Assets.Links {
		result.Assets.Links = append(result.Assets.Links, struct {
			URL string `json:"url"`
		}{URL: link.URL})
	}
	switch mode {
	case "complete":
	case "missing-asset":
		result.Assets.Links = result.Assets.Links[:len(result.Assets.Links)-1]
	case "extra-asset":
		extra := result.Assets.Links[0]
		extra.URL = strings.TrimSuffix(extra.URL, "/"+lastPath(extra.URL)) + "/unexpected.bin"
		result.Assets.Links = append(result.Assets.Links, extra)
	case "duplicate-asset":
		result.Assets.Links[len(result.Assets.Links)-1].URL = result.Assets.Links[0].URL
	case "wrong-tag":
		result.TagName = "v9.9.9"
	default:
		return remoteRelease{}, fmt.Errorf("unknown GitLab response fixture mode: %s", mode)
	}
	return result, nil
}

func lastPath(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.TrimSuffix(parsed.Path, "/"), "/")
	return parts[len(parts)-1]
}

func validateReleaseDocument(payload releasePayload, expectedTag string) error {
	if payload.TagName != expectedTag {
		return errors.New("release document has the wrong tag")
	}
	if len(payload.Assets.Links) != 15 {
		return fmt.Errorf("release document must contain 15 asset links, found %d", len(payload.Assets.Links))
	}
	for _, link := range payload.Assets.Links {
		if link.Name == "" || link.URL == "" || !strings.HasPrefix(link.DirectAssetPath, "/") {
			return errors.New("release document contains an invalid asset link")
		}
	}
	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func verifyGitLabRelease(expectedPath, actualPath, outputPath, expectedTag string) error {
	var expected releasePayload
	var actual remoteRelease
	if err := readJSON(expectedPath, &expected); err != nil {
		return fmt.Errorf("GitLab release verification returned invalid JSON: %w", err)
	}
	if err := readJSON(actualPath, &actual); err != nil {
		return fmt.Errorf("GitLab release verification returned invalid JSON: %w", err)
	}
	if actual.TagName != expectedTag {
		return errors.New("GitLab release verification returned the wrong tag")
	}
	if len(expected.Assets.Links) != 15 {
		return fmt.Errorf("GitLab release verification expected 15 local asset links, found %d", len(expected.Assets.Links))
	}
	if len(actual.Assets.Links) != 15 {
		return fmt.Errorf("GitLab release verification expected 15 remote asset links, found %d", len(actual.Assets.Links))
	}
	expectedURLs := map[string]bool{}
	actualURLs := map[string]bool{}
	var output strings.Builder
	for _, link := range expected.Assets.Links {
		name := strings.TrimPrefix(link.DirectAssetPath, "/")
		if link.URL == "" || !strings.HasPrefix(link.DirectAssetPath, "/") || name == "" || strings.ContainsAny(name, "/\t\r\n") {
			return errors.New("GitLab release verification found an invalid local asset link")
		}
		if expectedURLs[link.URL] {
			return errors.New("GitLab release verification found duplicate local asset URLs")
		}
		expectedURLs[link.URL] = true
		fmt.Fprintf(&output, "%s\t%s\n", name, link.URL)
	}
	for _, link := range actual.Assets.Links {
		if link.URL == "" {
			return errors.New("GitLab release verification found an invalid remote asset link")
		}
		if actualURLs[link.URL] {
			return errors.New("GitLab release verification found duplicate remote asset URLs")
		}
		actualURLs[link.URL] = true
	}
	for raw := range expectedURLs {
		if !actualURLs[raw] {
			return fmt.Errorf("GitLab release verification is missing asset %s", raw)
		}
	}
	for raw := range actualURLs {
		if !expectedURLs[raw] {
			return fmt.Errorf("GitLab release verification found unexpected asset %s", raw)
		}
	}
	return os.WriteFile(outputPath, []byte(output.String()), 0o600)
}

func readJSON(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}

func writeMSIMetadata(environmentPath, summaryPath, environmentGUID, packageGUID string, instant time.Time) error {
	environment := strings.Join([]string{
		"Environment\tName\tValue\tComponent_",
		"s72\tl64\tL255\ts72",
		"Environment\tEnvironment",
		environmentGUID + "\t=PATH\t[~];[INSTALLBINFOLDER]\tAigwPath",
	}, "\r\n") + "\r\n"
	if err := os.WriteFile(environmentPath, []byte(environment), 0o600); err != nil {
		return err
	}
	raw, err := os.ReadFile(summaryPath)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	seen := map[string]bool{}
	for index := 3; index < len(lines); index++ {
		fields := strings.Split(lines[index], "\t")
		if len(fields) != 2 {
			continue
		}
		switch fields[0] {
		case "9":
			fields[1], seen["9"] = packageGUID, true
		case "12", "13":
			fields[1], seen[fields[0]] = instant.UTC().Format("2006/01/02 15:04:05"), true
		}
		lines[index] = strings.Join(fields, "\t")
	}
	if !seen["9"] || !seen["12"] || !seen["13"] {
		return errors.New("MSI summary information is missing deterministic fields")
	}
	return os.WriteFile(summaryPath, []byte(strings.Join(lines, "\r\n")), 0o600)
}
