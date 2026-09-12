package publication

import (
	"aigw-cli/tools/release/artifact"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitHubPublisherFailClosedCases(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"malformed metadata": func(response http.ResponseWriter, _ *http.Request) {
			_, _ = response.Write([]byte("{"))
		},
		"creation rejected": func(response http.ResponseWriter, request *http.Request) {
			if request.Method == http.MethodGet {
				http.NotFound(response, request)
				return
			}
			response.WriteHeader(http.StatusInternalServerError)
			_, _ = response.Write([]byte(`{}`))
		},
		"missing upload URL": func(response http.ResponseWriter, request *http.Request) {
			if request.Method == http.MethodGet {
				http.NotFound(response, request)
				return
			}
			response.WriteHeader(http.StatusCreated)
			_, _ = response.Write([]byte(`{"id":1}`))
		},
		"asset set mismatch": func(response http.ResponseWriter, _ *http.Request) {
			_, _ = response.Write([]byte(`{"id":1,"assets":[]}`))
		},
	} {
		t.Run(name, func(t *testing.T) {
			artifacts := releaseFixture(t, "0.1.0")
			server := httptest.NewServer(handler)
			defer server.Close()
			if _, err := PublishGitHub(context.Background(), server.Client(), GitHubConfig{
				APIBase: server.URL, Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
			}); err == nil {
				t.Fatal("invalid GitHub state accepted")
			}
		})
	}
}

func TestGitHubPublisherRejectsUploadAndDownloadFailures(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	for name, statuses := range map[string][2]int{
		"upload":   {http.StatusInternalServerError, http.StatusOK},
		"download": {http.StatusCreated, http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			uploadStatus, downloadStatus := statuses[0], statuses[1]
			created := false
			remote := readReleaseFixture(t, artifacts, "0.1.0")
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				switch {
				case request.Method == http.MethodGet && strings.Contains(request.URL.Path, "/releases/tags/"):
					if !created {
						http.NotFound(response, request)
						return
					}
					writeGitHubFixture(t, response, request.Host, remote)
				case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/releases"):
					created = true
					response.WriteHeader(http.StatusCreated)
					_, _ = fmt.Fprintf(response, `{"id":1,"upload_url":"http://%s/uploads{?name,label}"}`, request.Host)
				case request.Method == http.MethodPost && request.URL.Path == "/uploads":
					response.WriteHeader(uploadStatus)
				case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/assets/"):
					response.WriteHeader(downloadStatus)
					if downloadStatus == http.StatusOK {
						_, _ = response.Write(remote[strings.TrimPrefix(request.URL.Path, "/assets/")])
					}
				}
			}))
			defer server.Close()
			if _, err := PublishGitHub(context.Background(), server.Client(), GitHubConfig{
				APIBase: server.URL, Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
			}); err == nil {
				t.Fatal("GitHub I/O failure accepted")
			}
		})
	}
}

func TestGitLabPublisherFailClosedCases(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	for name, payload := range map[string]string{
		"malformed metadata": "{",
		"wrong tag":          `{"tag_name":"v9.9.9","assets":{"links":[]}}`,
		"asset count":        `{"tag_name":"v0.1.0","assets":{"links":[]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				_, _ = response.Write([]byte(payload))
			}))
			defer server.Close()
			if _, err := PublishGitLab(context.Background(), server.Client(), GitLabConfig{
				APIBase: server.URL, ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
			}); err == nil {
				t.Fatal("invalid GitLab state accepted")
			}
		})
	}
}

func TestGitLabPublisherRejectsMissingAndMismatchedAssets(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	for name, mutate := range map[string]func(map[string][]byte){
		"missing": func(remote map[string][]byte) {
			delete(remote, artifact.Names("0.1.0")[0])
			remote["unexpected.bin"] = []byte("unexpected")
		},
		"mismatch": func(remote map[string][]byte) {
			remote[artifact.Names("0.1.0")[0]] = []byte("tampered")
		},
	} {
		t.Run(name, func(t *testing.T) {
			remote := readReleaseFixture(t, artifacts, "0.1.0")
			mutate(remote)
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if after, ok := strings.CutPrefix(request.URL.Path, "/packages/"); ok {
					_, _ = response.Write(remote[after])
					return
				}
				writeGitLabFixture(t, response, "http://"+request.Host, remote)
			}))
			defer server.Close()
			if _, err := PublishGitLab(context.Background(), server.Client(), GitLabConfig{
				APIBase: server.URL, ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
			}); err == nil {
				t.Fatal("invalid GitLab asset state accepted")
			}
		})
	}
}

func TestPublisherInputErrorsPropagate(t *testing.T) {
	if _, err := PublishGitHub(context.Background(), http.DefaultClient, GitHubConfig{}); err == nil {
		t.Fatal("invalid GitHub configuration accepted")
	}
	if _, err := PublishGitLab(context.Background(), http.DefaultClient, GitLabConfig{}); err == nil {
		t.Fatal("invalid GitLab configuration accepted")
	}
}

func TestCreationTransportAndStatusFailuresPropagate(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	transportFailure := errors.New("transport stopped")
	for name, publish := range map[string]func(*http.Client) error{
		"github": func(client *http.Client) error {
			_, err := PublishGitHub(context.Background(), client, GitHubConfig{
				APIBase: "https://example.test", Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
			})
			return err
		},
		"gitlab": func(client *http.Client) error {
			_, err := PublishGitLab(context.Background(), client, GitLabConfig{
				APIBase: "https://example.test", ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
			})
			return err
		},
	} {
		t.Run(name+" post transport", func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return response(http.StatusNotFound, ""), nil
				}
				return nil, transportFailure
			})}
			if err := publish(client); !errors.Is(err, transportFailure) {
				t.Fatalf("transport error lost: %v", err)
			}
		})
	}

	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusNotFound, ""), nil
	})}
	if _, err := PublishGitLab(context.Background(), client, GitLabConfig{
		APIBase: "https://example.test", ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
	}); err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("GitLab publication status accepted: %v", err)
	}
}

func TestPostCreationRefreshFailuresPropagate(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	want := errors.New("refresh failed")

	t.Run("GitHub", func(t *testing.T) {
		created := false
		client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && !created:
				return response(http.StatusNotFound, ""), nil
			case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/releases"):
				created = true
				return response(http.StatusCreated, `{"id":1,"upload_url":"https://example.test/uploads"}`), nil
			case request.Method == http.MethodPost && request.URL.Path == "/uploads":
				return response(http.StatusCreated, `{}`), nil
			case request.Method == http.MethodGet && created:
				return nil, want
			default:
				return response(http.StatusInternalServerError, ""), nil
			}
		})}
		_, err := PublishGitHub(context.Background(), client, GitHubConfig{
			APIBase: "https://example.test", Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
		})
		if !errors.Is(err, want) {
			t.Fatalf("refresh error = %v", err)
		}
	})

	t.Run("GitLab", func(t *testing.T) {
		created := false
		client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method == http.MethodGet {
				if created {
					return nil, want
				}
				return response(http.StatusNotFound, ""), nil
			}
			created = true
			return response(http.StatusCreated, ""), nil
		})}
		_, err := PublishGitLab(context.Background(), client, GitLabConfig{
			APIBase: "https://example.test", ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
		})
		if !errors.Is(err, want) {
			t.Fatalf("refresh error = %v", err)
		}
	})
}

func TestGitLabUploadUsesOnlyTheArtifactInventory(t *testing.T) {
	directory := releaseFixture(t, "0.1.0")
	if err := os.Mkdir(filepath.Join(directory, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	uploads := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if filepath.Base(request.URL.Path) == "nested" {
			t.Fatal("directory entry triggered an upload")
		}
		uploads++
		return response(http.StatusCreated, ""), nil
	})}
	if err := UploadGitLab(context.Background(), client, GitLabConfig{
		APIBase: "https://example.test", ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: directory, Trust: fixtureTrust(directory), Source: fixtureSource(directory),
	}); err != nil {
		t.Fatal(err)
	}
	if uploads != len(artifact.Names("0.1.0")) {
		t.Fatalf("uploaded %d artifacts", uploads)
	}
}

func TestGitLabUploadRequiresEveryDeclaredArtifact(t *testing.T) {
	directory := releaseFixture(t, "0.1.0")
	if err := os.Remove(filepath.Join(directory, artifact.Archives("0.1.0")[0])); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("incomplete matrix triggered a network request")
		return nil, errors.New("unexpected upload")
	})}
	if err := UploadGitLab(context.Background(), client, GitLabConfig{
		APIBase: "https://example.test", ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: directory, Trust: fixtureTrust(directory), Source: fixtureSource(directory),
	}); err == nil || !strings.Contains(err.Error(), "missing or empty") {
		t.Fatal("unreadable upload artifact was accepted")
	}
}

func TestGitHubAssetHelpersFailClosed(t *testing.T) {
	config := GitHubConfig{APIBase: "https://example.test", Tag: "v0.1.0", Token: "secret", Artifacts: t.TempDir()}
	if err := uploadGitHubAssets(context.Background(), http.DefaultClient, config, githubRelease{UploadURL: "https://example.test/uploads"}); err == nil {
		t.Fatal("missing local upload asset accepted")
	}
	config.Artifacts = releaseFixture(t, "0.1.0")
	if err := uploadGitHubAssets(context.Background(), http.DefaultClient, config, githubRelease{UploadURL: "%"}); err == nil {
		t.Fatal("malformed upload URL accepted")
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("upload failed")
	})}
	if err := uploadGitHubAssets(context.Background(), client, config, githubRelease{UploadURL: "https://example.test/uploads"}); err == nil {
		t.Fatal("upload transport failure accepted")
	}

	release := githubRelease{}
	for _, name := range artifact.Names("0.1.0") {
		release.Assets = append(release.Assets, struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		}{Name: name, URL: ":"})
	}
	if err := verifyGitHubAssets(context.Background(), http.DefaultClient, config, release, "0.1.0"); err == nil {
		t.Fatal("malformed download URL accepted")
	}
}

func TestVerificationReportsLocalArtifactReadFailures(t *testing.T) {
	version := "0.1.0"
	artifacts := releaseFixture(t, version)
	name := artifact.Names(version)[0]
	if err := os.Remove(filepath.Join(artifacts, name)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(artifacts, name), 0o700); err != nil {
		t.Fatal(err)
	}
	remote := readReleaseFixture(t, releaseFixture(t, version), version)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write(remote[strings.TrimPrefix(request.URL.Path, "/assets/")])
	}))
	defer server.Close()
	github := githubRelease{}
	for _, current := range artifact.Names(version) {
		github.Assets = append(github.Assets, struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		}{Name: current, URL: server.URL + "/assets/" + current})
	}
	if err := verifyGitHubAssets(context.Background(), server.Client(), GitHubConfig{Tag: "v" + version, Token: "token", Artifacts: artifacts}, github, version); err == nil {
		t.Fatal("unreadable local GitHub artifact accepted")
	}

	expected := releaseDocument("v"+version, server.URL+"/assets")
	actual := remoteRelease{TagName: "v" + version}
	for _, link := range expected.Assets.Links {
		actual.Assets.Links = append(actual.Assets.Links, struct {
			URL string `json:"url"`
		}{URL: link.URL})
	}
	if err := verifyGitLabAssets(context.Background(), server.Client(), GitLabConfig{APIBase: server.URL, Tag: "v" + version, Token: "token", Artifacts: artifacts}, expected, actual, version); err == nil {
		t.Fatal("unreadable local GitLab artifact accepted")
	}
}

func TestGitLabAssetHelperErrorPaths(t *testing.T) {
	version := "0.1.0"
	expected := releaseDocument("v"+version, "https://example.test/packages")
	actual := remoteRelease{TagName: "v" + version}
	for _, link := range expected.Assets.Links {
		actual.Assets.Links = append(actual.Assets.Links, struct {
			URL string `json:"url"`
		}{URL: "://invalid/" + strings.TrimPrefix(link.DirectAssetPath, "/")})
	}
	config := GitLabConfig{APIBase: "https://example.test", Tag: "v" + version, Token: "secret", Artifacts: releaseFixture(t, version)}
	if err := verifyGitLabAssets(context.Background(), http.DefaultClient, config, expected, actual, version); err == nil {
		t.Fatal("invalid GitLab asset authority accepted")
	}

	actual = remoteRelease{TagName: "v" + version}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(filepath.Base(request.URL.Path) + "\n"))
	}))
	defer server.Close()
	for _, name := range artifact.Names(version) {
		actual.Assets.Links = append(actual.Assets.Links, struct {
			URL string `json:"url"`
		}{URL: server.URL + "/" + name})
	}
	config.APIBase = server.URL
	config.Artifacts = t.TempDir()
	if err := verifyGitLabAssets(context.Background(), server.Client(), config, expected, actual, version); err == nil {
		t.Fatal("missing local GitLab artifact accepted")
	}
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func releaseFixture(t *testing.T, version string) string {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "artifacts")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range artifact.Names(version) {
		if name == "checksums.txt" || name == "checksums.txt.sig" {
			continue
		}
		if err := os.WriteFile(filepath.Join(directory, name), []byte(name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	key := filepath.Join(t.TempDir(), "release-signing-key")
	if output, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("generate signing key: %v: %s", err, output)
	}
	publicKey, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	trust := fixtureTrust(directory)
	if err := os.WriteFile(trust.AllowedSigners, []byte(trust.Principal+` namespaces="git,aigw-release" `+string(publicKey)), 0o600); err != nil {
		t.Fatal(err)
	}
	source := fixtureSource(directory)
	if err := os.Mkdir(source.Repository, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"VERSION": version + "\n", "go.mod": "module example.invalid/aigw\n",
		"go.sum": "sum\n", "package-lock.json": "{}\n", "mise.lock": "lockfile_version = 1\n",
		"mise.toml": "[tools]\ngo = \"1.27.1\"\n",
	} {
		if err := os.WriteFile(filepath.Join(source.Repository, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	releaseGit(t, source.Repository, "init", "-q", "-b", "main")
	releaseGit(t, source.Repository, "config", "gpg.format", "ssh")
	releaseGit(t, source.Repository, "config", "user.signingkey", key)
	releaseGit(t, source.Repository, "add", ".")
	releaseGit(t, source.Repository, "commit", "-q", "-S", "-m", "test: release source")
	releaseGit(t, source.Repository, "tag", "-s", "-m", "Release "+version, "v"+version)
	if err := artifact.WriteProvenance(source.Repository, directory,
		filepath.Join(directory, "aigw_"+version+".provenance.json"), version,
		releaseGit(t, source.Repository, "rev-parse", "HEAD"), releaseGit(t, source.Repository, "rev-parse", "HEAD^{tree}")); err != nil {
		t.Fatal(err)
	}
	if err := artifact.RewriteChecksums(directory, version); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(directory, "checksums.txt")
	if output, err := exec.Command("ssh-keygen", "-Y", "sign", "-n", artifact.SignatureNamespace, "-f", key, manifest).CombinedOutput(); err != nil {
		t.Fatalf("sign checksum manifest: %v: %s", err, output)
	}
	return directory
}

func readReleaseFixture(t *testing.T, directory, version string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	for _, name := range artifact.Names(version) {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		result[name] = data
	}
	return result
}

func writeGitHubFixture(t *testing.T, response http.ResponseWriter, host string, assets map[string][]byte) {
	t.Helper()
	type asset struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	payload := struct {
		ID        int     `json:"id"`
		UploadURL string  `json:"upload_url"`
		Assets    []asset `json:"assets"`
	}{ID: 1, UploadURL: "http://" + host + "/uploads{?name,label}"}
	for name := range assets {
		payload.Assets = append(payload.Assets, asset{Name: name, URL: "http://" + host + "/assets/" + name})
	}
	if err := json.NewEncoder(response).Encode(payload); err != nil {
		t.Fatal(err)
	}
}

func writeGitLabFixture(t *testing.T, response http.ResponseWriter, base string, assets map[string][]byte) {
	t.Helper()
	payload := remoteRelease{TagName: "v0.1.0"}
	for name := range assets {
		payload.Assets.Links = append(payload.Assets.Links, struct {
			URL string `json:"url"`
		}{URL: base + "/packages/" + name})
	}
	if err := json.NewEncoder(response).Encode(payload); err != nil {
		t.Fatal(err)
	}
}

func fixtureTrust(directory string) artifact.SignatureTrust {
	return artifact.SignatureTrust{AllowedSigners: filepath.Join(filepath.Dir(directory), "allowed-signers"), Principal: "release@test.invalid"}
}

func fixtureSource(directory string) artifact.SourceTrust {
	return artifact.SourceTrust{Repository: filepath.Join(filepath.Dir(directory), "source"), AllowedSigners: fixtureTrust(directory).AllowedSigners}
}

func releaseGit(t *testing.T, repository string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository,
		"-c", "core.hooksPath=" + filepath.Join(repository, ".git", "hooks"),
		"-c", "user.name=Release Test", "-c", "user.email=release@test.invalid",
		"-c", "gpg.ssh.program=ssh-keygen",
	}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}
