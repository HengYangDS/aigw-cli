package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadGitLabArtifactsUsesGenericPackageAPI(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "asset.txt"), []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || request.URL.Path != "/projects/7/packages/generic/aigw/1.2.3/asset.txt" {
			t.Fatalf("request=%s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("JOB-TOKEN") != "token" {
			t.Fatalf("token=%q", request.Header.Get("JOB-TOKEN"))
		}
		data, _ := io.ReadAll(request.Body)
		if string(data) != "payload" {
			t.Fatalf("payload=%q", data)
		}
		response.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	if err := uploadGitLabArtifacts(context.Background(), server.Client(), gitLabPublishConfig{APIBase: server.URL, ProjectID: "7", Tag: "v1.2.3", Token: "token", Artifacts: directory}); err != nil {
		t.Fatal(err)
	}
}

func TestUploadGitLabArtifactsFailsClosedAtEveryBoundary(t *testing.T) {
	valid := gitLabPublishConfig{APIBase: "https://example.test/api/v4", ProjectID: "7", Tag: "v1.2.3", Token: "token", Artifacts: t.TempDir()}
	if err := uploadGitLabArtifacts(context.Background(), http.DefaultClient, gitLabPublishConfig{}); err == nil {
		t.Fatal("invalid upload inputs accepted")
	}
	missing := valid
	missing.Artifacts = filepath.Join(t.TempDir(), "missing")
	if err := uploadGitLabArtifacts(context.Background(), http.DefaultClient, missing); err == nil {
		t.Fatal("missing artifact directory accepted")
	}
	if err := os.Mkdir(filepath.Join(valid.Artifacts, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(valid.Artifacts, "asset.txt"), []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	transport := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("upload unavailable")
	})}
	if err := uploadGitLabArtifacts(context.Background(), transport, valid); err == nil || !strings.Contains(err.Error(), "upload unavailable") {
		t.Fatalf("transport error = %v", err)
	}
	status := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusConflict, ""), nil
	})}
	if err := uploadGitLabArtifacts(context.Background(), status, valid); err == nil || !strings.Contains(err.Error(), "HTTP 409") {
		t.Fatalf("status error = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestGitHubPublisherCreatesAndVerifiesImmutableRelease(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0-rc.1")
	remote := map[string][]byte{}
	created := false
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("missing GitHub authorization: %q", request.Header.Get("Authorization"))
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/repos/acme/aigw/releases/tags/v0.1.0-rc.1":
			if !created {
				http.NotFound(response, request)
				return
			}
			writeGitHubFixture(t, response, request.Host, remote)
		case request.Method == http.MethodPost && request.URL.Path == "/repos/acme/aigw/releases":
			var payload struct {
				Prerelease bool `json:"prerelease"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if !payload.Prerelease {
				t.Fatal("release candidate was not marked prerelease")
			}
			created = true
			response.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(response, `{"id":1,"upload_url":"http://%s/uploads{?name,label}","assets":[]}`, request.Host)
		case request.Method == http.MethodPost && request.URL.Path == "/uploads":
			data, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatal(err)
			}
			remote[request.URL.Query().Get("name")] = data
			response.WriteHeader(http.StatusCreated)
		case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/assets/"):
			_, _ = response.Write(remote[strings.TrimPrefix(request.URL.Path, "/assets/")])
		default:
			t.Fatalf("unexpected GitHub request: %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	createdNow, err := publishGitHubRelease(context.Background(), server.Client(), githubPublishConfig{
		APIBase:    server.URL,
		Repository: "acme/aigw",
		Tag:        "v0.1.0-rc.1",
		Token:      "secret",
		Artifacts:  artifacts,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !createdNow || len(remote) != len(artifactNames("0.1.0-rc.1")) {
		t.Fatalf("created=%v remote assets=%d", createdNow, len(remote))
	}
}

func TestGitHubPublisherRejectsExistingMismatchWithoutMutation(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	remote := readReleaseFixture(t, artifacts, "0.1.0")
	remote[artifactNames("0.1.0")[0]] = []byte("tampered")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			t.Fatal("existing release must never be mutated")
		}
		if strings.HasPrefix(request.URL.Path, "/assets/") {
			_, _ = response.Write(remote[strings.TrimPrefix(request.URL.Path, "/assets/")])
			return
		}
		writeGitHubFixture(t, response, request.Host, remote)
	}))
	defer server.Close()

	if _, err := publishGitHubRelease(context.Background(), server.Client(), githubPublishConfig{
		APIBase: server.URL, Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts,
	}); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("mismatch accepted: %v", err)
	}
}

func TestGitHubPublisherAcceptsExistingExactRelease(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	remote := readReleaseFixture(t, artifacts, "0.1.0")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			t.Fatal("exact existing release must not be mutated")
		}
		if strings.HasPrefix(request.URL.Path, "/assets/") {
			_, _ = response.Write(remote[strings.TrimPrefix(request.URL.Path, "/assets/")])
			return
		}
		writeGitHubFixture(t, response, request.Host, remote)
	}))
	defer server.Close()

	created, err := publishGitHubRelease(context.Background(), server.Client(), githubPublishConfig{
		APIBase: server.URL, Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts,
	})
	if err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
}

func TestGitLabPublisherCreatesAndVerifiesImmutableRelease(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	remote := readReleaseFixture(t, artifacts, "0.1.0")
	created := false
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("JOB-TOKEN") != "secret" {
			t.Fatalf("missing GitLab authorization")
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/projects/7/releases/v0.1.0":
			if !created {
				http.NotFound(response, request)
				return
			}
			writeGitLabFixture(t, response, "http://"+request.Host, remote)
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/projects/7/releases":
			created = true
			response.WriteHeader(http.StatusCreated)
		case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/packages/"):
			_, _ = response.Write(remote[strings.TrimPrefix(request.URL.Path, "/packages/")])
		default:
			t.Fatalf("unexpected GitLab request: %s %s", request.Method, request.URL.String())
		}
	}))
	defer server.Close()

	createdNow, err := publishGitLabRelease(context.Background(), server.Client(), gitLabPublishConfig{
		APIBase: server.URL + "/api/v4", ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !createdNow {
		t.Fatal("GitLab release was not created")
	}
}

func TestGitLabPublisherAcceptsExistingExactRelease(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	remote := readReleaseFixture(t, artifacts, "0.1.0")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			t.Fatal("exact existing release must not be mutated")
		}
		if strings.HasPrefix(request.URL.Path, "/packages/") {
			_, _ = response.Write(remote[strings.TrimPrefix(request.URL.Path, "/packages/")])
			return
		}
		writeGitLabFixture(t, response, "http://"+request.Host, remote)
	}))
	defer server.Close()

	created, err := publishGitLabRelease(context.Background(), server.Client(), gitLabPublishConfig{
		APIBase: server.URL + "/api/v4", ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts,
	})
	if err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
}

func TestPublishInputsFailClosed(t *testing.T) {
	for name, values := range map[string][5]string{
		"missing":   {"", "target", "v1.2.3", "token", "dist"},
		"bad URL":   {"file:///tmp", "target", "v1.2.3", "token", "dist"},
		"bad tag":   {"https://example.test", "target", "1.2.3", "token", "dist"},
		"short tag": {"https://example.test", "target", "v1", "token", "dist"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := publishInputs(values[0], values[1], values[2], values[3], values[4]); err == nil {
				t.Fatal("invalid publish inputs accepted")
			}
		})
	}
}

func TestPublishersRejectUnexpectedRemoteStates(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusInternalServerError)
		_, _ = response.Write([]byte(`{"message":"failed"}`))
	}))
	defer server.Close()
	if _, err := publishGitHubRelease(context.Background(), server.Client(), githubPublishConfig{
		APIBase: server.URL, Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts,
	}); err == nil {
		t.Fatal("GitHub 500 accepted")
	}
	if _, err := publishGitLabRelease(context.Background(), server.Client(), gitLabPublishConfig{
		APIBase: server.URL, ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts,
	}); err == nil {
		t.Fatal("GitLab 500 accepted")
	}
}

func TestPublishersRejectInvalidArtifactsAndTransportFailure(t *testing.T) {
	invalid := t.TempDir()
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network unavailable")
	})}
	for name, publish := range map[string]func(string, *http.Client) error{
		"github": func(artifacts string, client *http.Client) error {
			_, err := publishGitHubRelease(context.Background(), client, githubPublishConfig{
				APIBase: "https://example.test", Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts,
			})
			return err
		},
		"gitlab": func(artifacts string, client *http.Client) error {
			_, err := publishGitLabRelease(context.Background(), client, gitLabPublishConfig{
				APIBase: "https://example.test/api/v4", ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts,
			})
			return err
		},
	} {
		t.Run(name+" invalid artifacts", func(t *testing.T) {
			if err := publish(invalid, http.DefaultClient); err == nil {
				t.Fatal("invalid artifact matrix accepted")
			}
		})
		t.Run(name+" transport", func(t *testing.T) {
			if err := publish(releaseFixture(t, "0.1.0"), client); err == nil || !strings.Contains(err.Error(), "network unavailable") {
				t.Fatalf("transport failure not returned: %v", err)
			}
		})
	}
}

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
			server := httptest.NewServer(handler)
			defer server.Close()
			if _, err := publishGitHubRelease(context.Background(), server.Client(), githubPublishConfig{
				APIBase: server.URL, Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: releaseFixture(t, "0.1.0"),
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
			if _, err := publishGitHubRelease(context.Background(), server.Client(), githubPublishConfig{
				APIBase: server.URL, Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts,
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
			if _, err := publishGitLabRelease(context.Background(), server.Client(), gitLabPublishConfig{
				APIBase: server.URL, ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts,
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
			delete(remote, artifactNames("0.1.0")[0])
			remote["unexpected.bin"] = []byte("unexpected")
		},
		"mismatch": func(remote map[string][]byte) {
			remote[artifactNames("0.1.0")[0]] = []byte("tampered")
		},
	} {
		t.Run(name, func(t *testing.T) {
			remote := readReleaseFixture(t, artifacts, "0.1.0")
			mutate(remote)
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if strings.HasPrefix(request.URL.Path, "/packages/") {
					_, _ = response.Write(remote[strings.TrimPrefix(request.URL.Path, "/packages/")])
					return
				}
				writeGitLabFixture(t, response, "http://"+request.Host, remote)
			}))
			defer server.Close()
			if _, err := publishGitLabRelease(context.Background(), server.Client(), gitLabPublishConfig{
				APIBase: server.URL, ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts,
			}); err == nil {
				t.Fatal("invalid GitLab asset state accepted")
			}
		})
	}
}

func TestPublisherInputErrorsPropagate(t *testing.T) {
	if _, err := publishGitHubRelease(context.Background(), http.DefaultClient, githubPublishConfig{}); err == nil {
		t.Fatal("invalid GitHub configuration accepted")
	}
	if _, err := publishGitLabRelease(context.Background(), http.DefaultClient, gitLabPublishConfig{}); err == nil {
		t.Fatal("invalid GitLab configuration accepted")
	}
}

func TestCreationTransportAndStatusFailuresPropagate(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	transportFailure := errors.New("transport stopped")
	for name, publish := range map[string]func(*http.Client) error{
		"github": func(client *http.Client) error {
			_, err := publishGitHubRelease(context.Background(), client, githubPublishConfig{
				APIBase: "https://example.test", Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts,
			})
			return err
		},
		"gitlab": func(client *http.Client) error {
			_, err := publishGitLabRelease(context.Background(), client, gitLabPublishConfig{
				APIBase: "https://example.test", ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts,
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
	if _, err := publishGitLabRelease(context.Background(), client, gitLabPublishConfig{
		APIBase: "https://example.test", ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts,
	}); err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("GitLab publication status accepted: %v", err)
	}
}

func TestRequestHelpersRejectMalformedEndpointsAndTransportFailure(t *testing.T) {
	if _, _, err := githubRequest(context.Background(), http.DefaultClient, http.MethodGet, ":", "token", nil); err == nil {
		t.Fatal("malformed GitHub endpoint accepted")
	}
	if _, _, err := gitLabRequest(context.Background(), http.DefaultClient, http.MethodGet, ":", "token", nil); err == nil {
		t.Fatal("malformed GitLab endpoint accepted")
	}
	request, err := http.NewRequest(http.MethodGet, "https://example.test/asset", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("download failed")
	})}
	if _, err := responseBytes(client, request); err == nil {
		t.Fatal("download transport failure accepted")
	}
}

func TestGitHubAssetHelpersFailClosed(t *testing.T) {
	config := githubPublishConfig{Tag: "v0.1.0", Token: "secret", Artifacts: t.TempDir()}
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
	for _, name := range artifactNames("0.1.0") {
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
	name := artifactNames(version)[0]
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
	for _, current := range artifactNames(version) {
		github.Assets = append(github.Assets, struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		}{Name: current, URL: server.URL + "/assets/" + current})
	}
	if err := verifyGitHubAssets(context.Background(), server.Client(), githubPublishConfig{Tag: "v" + version, Token: "token", Artifacts: artifacts}, github, version); err == nil {
		t.Fatal("unreadable local GitHub artifact accepted")
	}

	expected := releaseDocument("v"+version, server.URL+"/assets")
	actual := remoteRelease{TagName: "v" + version}
	for _, link := range expected.Assets.Links {
		actual.Assets.Links = append(actual.Assets.Links, struct {
			URL string `json:"url"`
		}{URL: link.URL})
	}
	if err := verifyGitLabAssets(context.Background(), server.Client(), gitLabPublishConfig{APIBase: server.URL, Tag: "v" + version, Token: "token", Artifacts: artifacts}, expected, actual, version); err == nil {
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
	config := gitLabPublishConfig{APIBase: "https://example.test", Tag: "v" + version, Token: "secret", Artifacts: releaseFixture(t, version)}
	if err := verifyGitLabAssets(context.Background(), http.DefaultClient, config, expected, actual, version); err == nil {
		t.Fatal("invalid GitLab asset authority accepted")
	}

	actual = remoteRelease{TagName: "v" + version}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(filepath.Base(request.URL.Path) + "\n"))
	}))
	defer server.Close()
	for _, name := range artifactNames(version) {
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
	directory := t.TempDir()
	for _, name := range artifactNames(version)[:len(artifactNames(version))-1] {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := rewriteChecksums(directory, version); err != nil {
		t.Fatal(err)
	}
	return directory
}

func readReleaseFixture(t *testing.T, directory, version string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	for _, name := range artifactNames(version) {
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
