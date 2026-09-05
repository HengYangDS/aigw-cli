package publication

import (
	"aigw-cli/tools/release/artifact"
	"fmt"
	"net/url"
	"strings"
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

func releaseDocument(tag, base string) releasePayload {
	version := strings.TrimPrefix(tag, "v")
	payload := releasePayload{TagName: tag, Name: "AIGW " + tag, Description: "Cross-platform AIGW CLI release. Verify downloads with checksums.txt."}
	for _, name := range artifact.Names(version) {
		payload.Assets.Links = append(payload.Assets.Links, releaseLink{Name: name, URL: strings.TrimSuffix(base, "/") + "/" + name, DirectAssetPath: "/" + name})
	}
	return payload
}

func lastPath(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.TrimSuffix(parsed.Path, "/"), "/")
	return parts[len(parts)-1]
}
