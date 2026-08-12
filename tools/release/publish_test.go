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
