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
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadGitLabArtifactsUsesGenericPackageAPI(t *testing.T) {
	directory := releaseFixture(t, "1.2.3")
	expected := readReleaseFixture(t, directory, "1.2.3")
	uploaded := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || !strings.HasPrefix(request.URL.Path, "/projects/7/packages/generic/aigw/1.2.3/") {
			t.Fatalf("request=%s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Job-Token") != "token" {
			t.Fatalf("token=%q", request.Header.Get("Job-Token"))
		}
		data, _ := io.ReadAll(request.Body)
		if string(data) != string(expected[filepath.Base(request.URL.Path)]) {
			t.Fatalf("payload=%q", data)
		}
		uploaded++
		response.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	if err := UploadGitLab(context.Background(), server.Client(), GitLabConfig{APIBase: server.URL, ProjectID: "7", Tag: "v1.2.3", Token: "token", Artifacts: directory, Trust: fixtureTrust(directory), Source: fixtureSource(directory)}); err != nil {
		t.Fatal(err)
	}
	if uploaded != len(expected) {
		t.Fatalf("uploaded=%d, expected=%d", uploaded, len(expected))
	}
}

func TestUploadGitLabArtifactsFailsClosedAtEveryBoundary(t *testing.T) {
	directory := releaseFixture(t, "1.2.3")
	valid := GitLabConfig{APIBase: "https://example.test/api/v4", ProjectID: "7", Tag: "v1.2.3", Token: "token", Artifacts: directory, Trust: fixtureTrust(directory), Source: fixtureSource(directory)}
	if err := UploadGitLab(context.Background(), http.DefaultClient, GitLabConfig{}); err == nil {
		t.Fatal("invalid upload inputs accepted")
	}
	missing := valid
	missing.Artifacts = filepath.Join(t.TempDir(), "missing")
	if err := UploadGitLab(context.Background(), http.DefaultClient, missing); err == nil {
		t.Fatal("missing artifact directory accepted")
	}
	if err := os.Mkdir(filepath.Join(valid.Artifacts, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	transport := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("upload unavailable")
	})}
	if err := UploadGitLab(context.Background(), transport, valid); err == nil || !strings.Contains(err.Error(), "upload unavailable") {
		t.Fatalf("transport error = %v", err)
	}
	status := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusConflict, ""), nil
	})}
	if err := UploadGitLab(context.Background(), status, valid); err == nil || !strings.Contains(err.Error(), "HTTP 409") {
		t.Fatalf("status error = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func TestPublicationRequiresSourceProvenanceBeforeNetwork(t *testing.T) {
	directory := releaseFixture(t, "1.2.3")
	trust := fixtureTrust(directory)
	source := fixtureSource(directory)
	// A signed successor with the same version must not authorize artifacts
	// built from its predecessor, even when both sources use the approved key.
	if err := os.WriteFile(filepath.Join(source.Repository, "change.txt"), []byte("successor\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	releaseGit(t, source.Repository, "add", "change.txt")
	releaseGit(t, source.Repository, "commit", "-q", "-S", "-m", "test: successor source")
	releaseGit(t, source.Repository, "tag", "-f", "-s", "-m", "Successor", "v1.2.3")
	for name, publish := range map[string]func(*http.Client) error{
		"github": func(client *http.Client) error {
			_, err := PublishGitHub(t.Context(), client, GitHubConfig{
				APIBase: "https://example.test", Repository: "acme/aigw", Tag: "v1.2.3", Token: "token", Artifacts: directory, Trust: trust, Source: source,
			})
			return err
		},
		"gitlab": func(client *http.Client) error {
			_, err := PublishGitLab(t.Context(), client, GitLabConfig{
				APIBase: "https://example.test", ProjectID: "7", Tag: "v1.2.3", Token: "token", Artifacts: directory, Trust: trust, Source: source,
			})
			return err
		},
		"gitlab upload": func(client *http.Client) error {
			return UploadGitLab(t.Context(), client, GitLabConfig{
				APIBase: "https://example.test", ProjectID: "7", Tag: "v1.2.3", Token: "token", Artifacts: directory, Trust: trust, Source: source,
			})
		},
	} {
		t.Run(name, func(t *testing.T) {
			requests := 0
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests++
				return response(http.StatusInternalServerError, "{}"), nil
			})}
			if err := publish(client); err == nil || !strings.Contains(err.Error(), "provenance does not match") || requests != 0 {
				t.Fatalf("source admission=%v, network requests=%d, want 0", err, requests)
			}
		})
	}
}

func TestProvenanceUsesTaggedInputsAndCompleteArtifactSubjects(t *testing.T) {
	directory := releaseFixture(t, "1.2.3")
	source := fixtureSource(directory)
	commit := releaseGit(t, source.Repository, "rev-parse", "refs/tags/v1.2.3^{commit}")
	tree := releaseGit(t, source.Repository, "rev-parse", commit+"^{tree}")
	for _, name := range []string{"go.mod", "go.sum", "package-lock.json", "mise.lock", "mise.toml"} {
		t.Run(name, func(t *testing.T) {
			original, err := os.ReadFile(filepath.Join(source.Repository, name))
			if err != nil {
				t.Fatal(err)
			}
			changed := []byte("changed input\n")
			if name == "mise.toml" {
				changed = []byte("[tools]\ngo = \"0.0.0\"\n")
			}
			if err := os.WriteFile(filepath.Join(source.Repository, name), changed, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := artifact.VerifyProvenance(t.Context(), directory, "v1.2.3", source); err != nil {
				t.Fatalf("mutable checkout must not replace tagged inputs: %v", err)
			}
			target := filepath.Join(directory, "aigw_1.2.3.provenance.json")
			if err := artifact.WriteProvenance(source.Repository, directory, target, "1.2.3", commit, tree); err != nil {
				t.Fatal(err)
			}
			if err := artifact.VerifyProvenance(t.Context(), directory, "v1.2.3", source); err == nil {
				t.Fatal("untagged dependency or toolchain input accepted")
			}
			if err := os.WriteFile(filepath.Join(source.Repository, name), original, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := artifact.WriteProvenance(source.Repository, directory, target, "1.2.3", commit, tree); err != nil {
				t.Fatal(err)
			}
		})
	}
	if err := os.WriteFile(filepath.Join(directory, artifact.Archives("1.2.3")[0]), []byte("different binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := artifact.VerifyProvenance(t.Context(), directory, "v1.2.3", source); err == nil {
		t.Fatal("artifact absent from signed provenance subjects accepted")
	}
}

func TestProvenanceRequiresApprovedSignedSourceAndVersion(t *testing.T) {
	for name, mutate := range map[string]func(*testing.T, artifact.SourceTrust){
		"missing tag": func(t *testing.T, source artifact.SourceTrust) {
			releaseGit(t, source.Repository, "tag", "-d", "v1.2.3")
		},
		"lightweight tag": func(t *testing.T, source artifact.SourceTrust) {
			releaseGit(t, source.Repository, "tag", "-d", "v1.2.3")
			releaseGit(t, source.Repository, "-c", "tag.gpgsign=false", "tag", "v1.2.3")
		},
		"unsigned source": func(t *testing.T, source artifact.SourceTrust) {
			releaseGit(t, source.Repository, "commit", "-q", "--allow-empty", "--no-gpg-sign", "-m", "test: unsigned source")
			releaseGit(t, source.Repository, "tag", "-f", "-s", "-m", "Release", "v1.2.3")
		},
		"different version": func(t *testing.T, source artifact.SourceTrust) {
			if err := os.WriteFile(filepath.Join(source.Repository, "VERSION"), []byte("2.0.0\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			releaseGit(t, source.Repository, "add", "VERSION")
			releaseGit(t, source.Repository, "commit", "-q", "-S", "-m", "test: different version")
			releaseGit(t, source.Repository, "tag", "-f", "-s", "-m", "Release", "v1.2.3")
		},
		"unapproved signer": func(t *testing.T, source artifact.SourceTrust) {
			if err := os.WriteFile(source.AllowedSigners, []byte("# No approved signers\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			directory := releaseFixture(t, "1.2.3")
			source := fixtureSource(directory)
			if err := artifact.VerifyProvenance(t.Context(), directory, "v1.2.3", source); err != nil {
				t.Fatal(err)
			}
			mutate(t, source)
			if err := artifact.VerifyProvenance(t.Context(), directory, "v1.2.3", source); err == nil {
				t.Fatal("source admission accepted a source without its required signed identity")
			}
		})
	}
}

func TestPublicationRequiresArtifactAuthorizationBeforeNetwork(t *testing.T) {
	artifacts := releaseFixture(t, "1.2.3")
	trust := fixtureTrust(artifacts)
	for name, publish := range map[string]func(*http.Client, artifact.SignatureTrust) error{
		"github release": func(client *http.Client, trust artifact.SignatureTrust) error {
			_, err := PublishGitHub(context.Background(), client, GitHubConfig{
				APIBase: "https://example.test", Repository: "acme/aigw", Tag: "v1.2.3", Token: "token", Artifacts: artifacts, Trust: trust,
			})
			return err
		},
		"gitlab release": func(client *http.Client, trust artifact.SignatureTrust) error {
			_, err := PublishGitLab(context.Background(), client, GitLabConfig{
				APIBase: "https://example.test", ProjectID: "7", Tag: "v1.2.3", Token: "token", Artifacts: artifacts, Trust: trust,
			})
			return err
		},
		"gitlab upload": func(client *http.Client, trust artifact.SignatureTrust) error {
			return UploadGitLab(context.Background(), client, GitLabConfig{
				APIBase: "https://example.test", ProjectID: "7", Tag: "v1.2.3", Token: "token", Artifacts: artifacts, Trust: trust,
			})
		},
	} {
		for condition, input := range map[string]artifact.SignatureTrust{
			"missing authorization": {},
			"different principal":   {AllowedSigners: trust.AllowedSigners, Principal: "other@test.invalid"},
			"different signing key": fixtureTrust(releaseFixture(t, "1.2.3")),
		} {
			t.Run(name+"/"+condition, func(t *testing.T) {
				requests := 0
				client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					requests++
					return response(http.StatusInternalServerError, "{}"), nil
				})}
				err := publish(client, input)
				if err == nil || requests != 0 {
					t.Fatalf("artifact authorization error=%v; network requests=%d, want 0", err, requests)
				}
			})
		}
	}
}

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestGitHubPublisherCreatesAndVerifiesImmutableRelease(t *testing.T) {
	for _, tc := range []struct {
		version    string
		prerelease bool
	}{
		{"0.1.0-rc.1+build.7", true},
		{"0.1.0+build-rc.1", false},
	} {
		t.Run(tc.version, func(t *testing.T) {
			artifacts := releaseFixture(t, tc.version)
			remote := map[string][]byte{}
			created := false
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.Header.Get("Authorization") != "Bearer secret" {
					t.Fatalf("missing GitHub authorization: %q", request.Header.Get("Authorization"))
				}
				switch {
				case request.Method == http.MethodGet && request.URL.Path == "/repos/acme/aigw/releases/tags/v"+tc.version:
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
					if payload.Prerelease != tc.prerelease {
						t.Errorf("prerelease=%t, want %t", payload.Prerelease, tc.prerelease)
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

			createdNow, err := PublishGitHub(context.Background(), server.Client(), GitHubConfig{
				APIBase:    server.URL,
				Repository: "acme/aigw",
				Tag:        "v" + tc.version,
				Token:      "secret",
				Artifacts:  artifacts,
				Trust:      fixtureTrust(artifacts),
				Source:     fixtureSource(artifacts),
			})
			if err != nil {
				t.Fatal(err)
			}
			if !createdNow || len(remote) != len(artifact.Names(tc.version)) {
				t.Fatalf("created=%v remote assets=%d", createdNow, len(remote))
			}
		})
	}
}

func TestGitHubPublisherRejectsExistingMismatchWithoutMutation(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	remote := readReleaseFixture(t, artifacts, "0.1.0")
	remote[artifact.Names("0.1.0")[0]] = []byte("tampered")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			t.Fatal("existing release must never be mutated")
		}
		if after, ok := strings.CutPrefix(request.URL.Path, "/assets/"); ok {
			_, _ = response.Write(remote[after])
			return
		}
		writeGitHubFixture(t, response, request.Host, remote)
	}))
	defer server.Close()

	if _, err := PublishGitHub(context.Background(), server.Client(), GitHubConfig{
		APIBase: server.URL, Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
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
		if after, ok := strings.CutPrefix(request.URL.Path, "/assets/"); ok {
			_, _ = response.Write(remote[after])
			return
		}
		writeGitHubFixture(t, response, request.Host, remote)
	}))
	defer server.Close()

	created, err := PublishGitHub(context.Background(), server.Client(), GitHubConfig{
		APIBase: server.URL, Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
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
		if request.Header.Get("Job-Token") != "secret" {
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

	createdNow, err := PublishGitLab(context.Background(), server.Client(), GitLabConfig{
		APIBase: server.URL + "/api/v4", ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
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
		if after, ok := strings.CutPrefix(request.URL.Path, "/packages/"); ok {
			_, _ = response.Write(remote[after])
			return
		}
		writeGitLabFixture(t, response, "http://"+request.Host, remote)
	}))
	defer server.Close()

	created, err := PublishGitLab(context.Background(), server.Client(), GitLabConfig{
		APIBase: server.URL + "/api/v4", ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
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

func TestPublicationValidatesSemanticIdentityBeforeUpload(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "asset.txt"), []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"v01.2.3", "v1.2.3-rc.01", "vno.real.version"} {
		t.Run(tag, func(t *testing.T) {
			requests := 0
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests++
				return response(http.StatusCreated, ""), nil
			})}
			err := UploadGitLab(t.Context(), client, GitLabConfig{
				APIBase: "https://example.test", ProjectID: "7", Tag: tag, Token: "token", Artifacts: directory,
			})
			if err == nil || !strings.Contains(err.Error(), "semver") || requests != 0 {
				t.Fatalf("publication error=%v requests=%d", err, requests)
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
	if _, err := PublishGitHub(context.Background(), server.Client(), GitHubConfig{
		APIBase: server.URL, Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
	}); err == nil {
		t.Fatal("GitHub 500 accepted")
	}
	if _, err := PublishGitLab(context.Background(), server.Client(), GitLabConfig{
		APIBase: server.URL, ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
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
			_, err := PublishGitHub(context.Background(), client, GitHubConfig{
				APIBase: "https://example.test", Repository: "acme/aigw", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
			})
			return err
		},
		"gitlab": func(artifacts string, client *http.Client) error {
			_, err := PublishGitLab(context.Background(), client, GitLabConfig{
				APIBase: "https://example.test/api/v4", ProjectID: "7", Tag: "v0.1.0", Token: "secret", Artifacts: artifacts, Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
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
