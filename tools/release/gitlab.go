package main

import (
	"aigw-cli/tools/release/artifact"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
)

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
		{"aigw_" + version + "_darwin_amd64.tar.gz", "macOS amd64 portable"},
		{"aigw_" + version + "_darwin_arm64.tar.gz", "macOS arm64 portable"},
		{"aigw_" + version + "_linux_amd64.tar.gz", "Linux amd64 portable"},
		{"aigw_" + version + "_linux_arm64.tar.gz", "Linux arm64 portable"},
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
	if len(payload.Assets.Links) != len(artifact.Names(strings.TrimPrefix(expectedTag, "v"))) {
		return fmt.Errorf("release document must contain %d asset links, found %d", len(artifact.Names(strings.TrimPrefix(expectedTag, "v"))), len(payload.Assets.Links))
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
	wanted := len(artifact.Names(strings.TrimPrefix(expectedTag, "v")))
	if len(expected.Assets.Links) != wanted {
		return fmt.Errorf("GitLab release verification expected %d local asset links, found %d", wanted, len(expected.Assets.Links))
	}
	if len(actual.Assets.Links) != wanted {
		return fmt.Errorf("GitLab release verification expected %d remote asset links, found %d", wanted, len(actual.Assets.Links))
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
