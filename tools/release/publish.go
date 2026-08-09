package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type githubPublishConfig struct {
	APIBase    string
	Repository string
	Tag        string
	Token      string
	Artifacts  string
}

type gitLabPublishConfig struct {
	APIBase   string
	ProjectID string
	Tag       string
	Token     string
	Artifacts string
}

type githubRelease struct {
	ID        int64  `json:"id"`
	UploadURL string `json:"upload_url"`
	Assets    []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"assets"`
}

func publishGitHubRelease(ctx context.Context, client *http.Client, config githubPublishConfig) (bool, error) {
	version, err := publishInputs(config.APIBase, config.Repository, config.Tag, config.Token, config.Artifacts)
	if err != nil {
		return false, err
	}
	if err := validateArtifactMatrix(config.Artifacts, version); err != nil {
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
			"prerelease":             strings.Contains(version, "-"),
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

func publishGitLabRelease(ctx context.Context, client *http.Client, config gitLabPublishConfig) (bool, error) {
	version, err := publishInputs(config.APIBase, config.ProjectID, config.Tag, config.Token, config.Artifacts)
	if err != nil {
		return false, err
	}
	if err := validateArtifactMatrix(config.Artifacts, version); err != nil {
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

func uploadGitLabArtifacts(ctx context.Context, client *http.Client, config gitLabPublishConfig) error {
	version, err := publishInputs(config.APIBase, config.ProjectID, config.Tag, config.Token, config.Artifacts)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(config.Artifacts)
	if err != nil {
		return err
	}
	base := strings.TrimSuffix(config.APIBase, "/") + "/projects/" + url.PathEscape(config.ProjectID) + "/packages/generic/aigw/" + url.PathEscape(version)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		file, err := os.Open(filepath.Join(config.Artifacts, name))
		if err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPut, base+"/"+url.PathEscape(name), file)
		if err != nil {
			_ = file.Close()
			return err
		}
		request.Header.Set("JOB-TOKEN", config.Token)
		response, err := client.Do(request)
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

func publishInputs(apiBase, target, tag, token, artifacts string) (string, error) {
	if apiBase == "" || target == "" || token == "" || artifacts == "" {
		return "", errors.New("release publication requires API base, target, tag, token, and artifacts")
	}
	parsed, err := url.Parse(apiBase)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return "", errors.New("release API base must be an absolute HTTP URL without user information")
	}
	if !strings.HasPrefix(tag, "v") || len(strings.Split(strings.TrimPrefix(tag, "v"), ".")) < 3 {
		return "", errors.New("release tag must use v<semver>")
	}
	return strings.TrimPrefix(tag, "v"), nil
}

func githubRequest(ctx context.Context, client *http.Client, method, endpoint, token string, body []byte) (githubRelease, int, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return githubRelease{}, 0, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
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

func uploadGitHubAssets(ctx context.Context, client *http.Client, config githubPublishConfig, release githubRelease) error {
	uploadURL := strings.Split(release.UploadURL, "{")[0]
	if uploadURL == "" {
		return errors.New("GitHub release response has no upload URL")
	}
	for _, name := range artifactNames(strings.TrimPrefix(config.Tag, "v")) {
		data, err := os.ReadFile(filepath.Join(config.Artifacts, name))
		if err != nil {
			return err
		}
		endpoint, err := url.Parse(uploadURL)
		if err != nil {
			return err
		}
		query := endpoint.Query()
		query.Set("name", name)
		endpoint.RawQuery = query.Encode()
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(data))
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+config.Token)
		request.Header.Set("Content-Type", mime.TypeByExtension(filepath.Ext(name)))
		response, err := client.Do(request)
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

func verifyGitHubAssets(ctx context.Context, client *http.Client, config githubPublishConfig, release githubRelease, version string) error {
	expected := artifactNames(version)
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
		request.Header.Set("Authorization", "Bearer "+config.Token)
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
	request.Header.Set("JOB-TOKEN", token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
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

func verifyGitLabAssets(ctx context.Context, client *http.Client, config gitLabPublishConfig, expected releasePayload, actual remoteRelease, version string) error {
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
	for _, name := range artifactNames(version) {
		endpoint, found := remote[name]
		if !found {
			return fmt.Errorf("GitLab release verification is missing asset %s", name)
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		same, err := sameAuthority(config.APIBase, endpoint)
		if err != nil {
			return err
		}
		if same {
			request.Header.Set("JOB-TOKEN", config.Token)
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

func responseBytes(client *http.Client, request *http.Request) ([]byte, error) {
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("release asset download failed with HTTP %d", response.StatusCode)
	}
	return io.ReadAll(response.Body)
}
