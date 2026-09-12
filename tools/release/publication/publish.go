// Package publication publishes verified release artifacts to configured forges.
package publication

import (
	"aigw-cli/tools/release/artifact"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// GitHubConfig contains the exact release identity, repository, credential, and artifact directory for GitHub publication.
type GitHubConfig struct {
	APIBase    string
	Repository string
	Tag        string
	Token      string
	Artifacts  string
	Trust      artifact.SignatureTrust
	Source     artifact.SourceTrust
}

// GitLabConfig contains the exact release identity, project, credential, and artifact directory for GitLab publication.
type GitLabConfig struct {
	APIBase   string
	ProjectID string
	Tag       string
	Token     string
	Artifacts string
	Trust     artifact.SignatureTrust
	Source    artifact.SourceTrust
}

type githubRelease struct {
	ID        int64  `json:"id"`
	UploadURL string `json:"upload_url"`
	Assets    []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"assets"`
}

// PublishGitHub creates or verifies one immutable GitHub Release and its complete asset inventory.
func PublishGitHub(ctx context.Context, client *http.Client, config GitHubConfig) (bool, error) {
	identity, err := publishInputs(config.APIBase, config.Repository, config.Tag, config.Token, config.Artifacts)
	if err != nil {
		return false, err
	}
	version := identity.String()
	if err := artifact.VerifyMatrix(ctx, config.Artifacts, version, config.Trust); err != nil {
		return false, err
	}
	if err := artifact.VerifyProvenance(ctx, config.Artifacts, config.Tag, config.Source); err != nil {
		return false, err
	}
	base := strings.TrimSuffix(config.APIBase, "/")
	releaseURL := base + "/repos/" + config.Repository + "/releases/tags/" + config.Tag
	release, status, err := githubRequest(ctx, client, http.MethodGet, releaseURL, config.Token, nil)
	if err != nil {
		return false, err
	}
	created := false
	if status == http.StatusNotFound {
		payload, _ := json.Marshal(map[string]any{
			"tag_name":               config.Tag,
			"name":                   "AIGW " + config.Tag,
			"generate_release_notes": true,
			"prerelease":             identity.Prerelease() != "",
		})
		release, status, err = githubRequest(ctx, client, http.MethodPost, base+"/repos/"+config.Repository+"/releases", config.Token, payload)
		if err != nil {
			return false, err
		}
		if status != http.StatusCreated {
			return false, fmt.Errorf("GitHub release publication failed with HTTP %d", status)
		}
		if err := uploadGitHubAssets(ctx, client, config, release); err != nil {
			return false, err
		}
		created = true
		release, status, err = githubRequest(ctx, client, http.MethodGet, releaseURL, config.Token, nil)
		if err != nil {
			return false, err
		}
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("GitHub release preflight failed with HTTP %d", status)
	}
	if err := verifyGitHubAssets(ctx, client, config, release, version); err != nil {
		return false, err
	}
	return created, nil
}

// PublishGitLab creates or verifies one immutable GitLab Release and its complete asset inventory.
func PublishGitLab(ctx context.Context, client *http.Client, config GitLabConfig) (bool, error) {
	identity, err := publishInputs(config.APIBase, config.ProjectID, config.Tag, config.Token, config.Artifacts)
	if err != nil {
		return false, err
	}
	version := identity.String()
	if err := artifact.VerifyMatrix(ctx, config.Artifacts, version, config.Trust); err != nil {
		return false, err
	}
	if err := artifact.VerifyProvenance(ctx, config.Artifacts, config.Tag, config.Source); err != nil {
		return false, err
	}
	base := strings.TrimSuffix(config.APIBase, "/") + "/projects/" + url.PathEscape(config.ProjectID)
	releaseURL := base + "/releases/" + config.Tag
	release, status, err := gitLabRequest(ctx, client, http.MethodGet, releaseURL, config.Token, nil)
	if err != nil {
		return false, err
	}
	created := false
	if status == http.StatusNotFound {
		packageBase := base + "/packages/generic/aigw/" + version
		payload, _ := json.Marshal(releaseDocument(config.Tag, packageBase))
		_, status, err = gitLabRequest(ctx, client, http.MethodPost, base+"/releases", config.Token, payload)
		if err != nil {
			return false, err
		}
		if status != http.StatusCreated && status != http.StatusConflict {
			return false, fmt.Errorf("GitLab release publication failed with HTTP %d", status)
		}
		created = status == http.StatusCreated
		release, status, err = gitLabRequest(ctx, client, http.MethodGet, releaseURL, config.Token, nil)
		if err != nil {
			return false, err
		}
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("GitLab release preflight failed with HTTP %d", status)
	}
	expected := releaseDocument(config.Tag, base+"/packages/generic/aigw/"+version)
	if err := verifyGitLabAssets(ctx, client, config, expected, release, version); err != nil {
		return false, err
	}
	return created, nil
}

// UploadGitLab uploads one complete artifact set through GitLab's generic package boundary before Release publication.
func UploadGitLab(ctx context.Context, client *http.Client, config GitLabConfig) error {
	identity, err := publishInputs(config.APIBase, config.ProjectID, config.Tag, config.Token, config.Artifacts)
	if err != nil {
		return err
	}
	version := identity.String()
	if err := artifact.VerifyMatrix(ctx, config.Artifacts, version, config.Trust); err != nil {
		return err
	}
	if err := artifact.VerifyProvenance(ctx, config.Artifacts, config.Tag, config.Source); err != nil {
		return err
	}
	base := strings.TrimSuffix(config.APIBase, "/") + "/projects/" + url.PathEscape(config.ProjectID) + "/packages/generic/aigw/" + url.PathEscape(version)
	for _, name := range artifact.Names(version) {
		file, err := os.Open(filepath.Join(config.Artifacts, name))
		if err != nil {
			return err
		}
		// publishInputs validates the base URL and every dynamic path segment is
		// escaped, so request construction has no remaining error domain.
		request, _ := http.NewRequestWithContext(ctx, http.MethodPut, base+"/"+url.PathEscape(name), file)
		request.Header.Set("Job-Token", config.Token)
		response, err := requestWithoutRedirects(client, request)
		_ = file.Close()
		if err != nil {
			return err
		}
		_ = response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return fmt.Errorf("GitLab artifact upload failed for %s with HTTP %d", name, response.StatusCode)
		}
	}
	return nil
}

func publishInputs(apiBase, target, tag, token, artifacts string) (*semver.Version, error) {
	if apiBase == "" || target == "" || token == "" || artifacts == "" {
		return nil, errors.New("release publication requires API base, target, tag, token, and artifacts")
	}
	parsed, err := url.Parse(apiBase)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return nil, errors.New("release API base must be an absolute HTTP URL without user information")
	}
	if !strings.HasPrefix(tag, "v") {
		return nil, errors.New("release tag must use v<semver>")
	}
	version, err := semver.StrictNewVersion(strings.TrimPrefix(tag, "v"))
	if err != nil {
		return nil, fmt.Errorf("release tag must use v<semver>: %w", err)
	}
	return version, nil
}

func githubRequest(ctx context.Context, client *http.Client, method, endpoint, token string, body []byte) (githubRelease, int, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return githubRelease{}, 0, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Github-Api-Version", "2022-11-28")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := requestWithoutRedirects(client, request)
	if err != nil {
		return githubRelease{}, 0, err
	}
	defer func() { _ = response.Body.Close() }()
	var release githubRelease
	if response.StatusCode != http.StatusNotFound {
		if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
			return githubRelease{}, response.StatusCode, err
		}
	}
	return release, response.StatusCode, nil
}

func uploadGitHubAssets(ctx context.Context, client *http.Client, config GitHubConfig, release githubRelease) error {
	uploadURL, _, _ := strings.Cut(release.UploadURL, "{")
	if uploadURL == "" {
		return errors.New("GitHub release response has no upload URL")
	}
	if _, err := assetAuthority(config.APIBase, uploadURL); err != nil {
		return err
	}
	endpoint, err := url.Parse(uploadURL)
	if err != nil {
		return err
	}
	for _, name := range artifact.Names(strings.TrimPrefix(config.Tag, "v")) {
		data, err := os.ReadFile(filepath.Join(config.Artifacts, name))
		if err != nil {
			return err
		}
		query := endpoint.Query()
		query.Set("name", name)
		endpoint.RawQuery = query.Encode()
		// url.Parse above and Query.Encode establish a valid request URL.
		request, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(data))
		request.Header.Set("Authorization", "Bearer "+config.Token)
		request.Header.Set("Content-Type", mime.TypeByExtension(filepath.Ext(name)))
		response, err := requestWithoutRedirects(client, request)
		if err != nil {
			return err
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			return fmt.Errorf("GitHub asset upload failed for %s with HTTP %d", name, response.StatusCode)
		}
	}
	return nil
}

func verifyGitHubAssets(ctx context.Context, client *http.Client, config GitHubConfig, release githubRelease, version string) error {
	expected := artifact.Names(version)
	actual := make([]string, 0, len(release.Assets))
	assets := map[string]string{}
	for _, asset := range release.Assets {
		actual = append(actual, asset.Name)
		assets[asset.Name] = asset.URL
	}
	slices.Sort(actual)
	sortedExpected := slices.Clone(expected)
	slices.Sort(sortedExpected)
	if !slices.Equal(actual, sortedExpected) {
		return fmt.Errorf("GitHub release asset set differs: %v", actual)
	}
	for _, name := range expected {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, assets[name], nil)
		if err != nil {
			return err
		}
		request.Header.Set("Accept", "application/octet-stream")
		same, err := assetAuthority(config.APIBase, assets[name])
		if err != nil {
			return err
		}
		if same {
			request.Header.Set("Authorization", "Bearer "+config.Token)
		}
		remote, err := responseBytes(client, request)
		if err != nil {
			return err
		}
		local, err := os.ReadFile(filepath.Join(config.Artifacts, name))
		if err != nil {
			return err
		}
		if !bytes.Equal(local, remote) {
			return fmt.Errorf("GitHub release asset differs from locally verified %s", name)
		}
	}
	return nil
}

func gitLabRequest(ctx context.Context, client *http.Client, method, endpoint, token string, body []byte) (remoteRelease, int, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return remoteRelease{}, 0, err
	}
	request.Header.Set("Job-Token", token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := requestWithoutRedirects(client, request)
	if err != nil {
		return remoteRelease{}, 0, err
	}
	defer func() { _ = response.Body.Close() }()
	var release remoteRelease
	if response.StatusCode != http.StatusNotFound {
		if err := json.NewDecoder(response.Body).Decode(&release); err != nil && response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusConflict {
			return remoteRelease{}, response.StatusCode, err
		}
	}
	return release, response.StatusCode, nil
}

func verifyGitLabAssets(ctx context.Context, client *http.Client, config GitLabConfig, expected releasePayload, actual remoteRelease, version string) error {
	if actual.TagName != config.Tag {
		return errors.New("GitLab release verification returned the wrong tag")
	}
	if len(actual.Assets.Links) != len(expected.Assets.Links) {
		return errors.New("GitLab release asset set differs")
	}
	remote := map[string]string{}
	for _, link := range actual.Assets.Links {
		remote[lastPath(link.URL)] = link.URL
	}
	for _, name := range artifact.Names(version) {
		endpoint, found := remote[name]
		if !found {
			return fmt.Errorf("GitLab release verification is missing asset %s", name)
		}
		// lastPath accepted this URL as the exact expected asset name; request
		// construction cannot fail before the stricter authority check below.
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		same, err := assetAuthority(config.APIBase, endpoint)
		if err != nil {
			return err
		}
		if same {
			request.Header.Set("Job-Token", config.Token)
		}
		data, err := responseBytes(client, request)
		if err != nil {
			return err
		}
		local, err := os.ReadFile(filepath.Join(config.Artifacts, name))
		if err != nil {
			return err
		}
		if !bytes.Equal(local, data) {
			return fmt.Errorf("GitLab release asset differs from locally verified %s", name)
		}
	}
	return nil
}

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
