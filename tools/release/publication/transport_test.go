package publication

import (
	"aigw-cli/tools/release/artifact"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPublicationMetadataDoesNotForwardCredentialsAcrossRedirects(t *testing.T) {
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			var redirected atomic.Int32
			destination := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				redirected.Add(1)
				if request.Header.Get("Authorization") != "" || request.Header.Get("Job-Token") != "" {
					t.Error("release credential crossed the selected API authority")
				}
				if err := json.NewEncoder(response).Encode(map[string]any{}); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(destination.Close)
			origin := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				http.Redirect(response, request, destination.URL, http.StatusTemporaryRedirect)
			}))
			t.Cleanup(origin.Close)
			client := origin.Client()
			var status int
			var err error
			if provider == "github" {
				_, status, err = githubRequest(t.Context(), client, http.MethodGet, origin.URL, "synthetic-token", nil)
			} else {
				_, status, err = gitLabRequest(t.Context(), client, http.MethodGet, origin.URL, "synthetic-token", nil)
			}
			if err == nil && status == http.StatusOK {
				t.Error("redirected metadata was accepted")
			}
			if redirected.Load() != 0 || client.CheckRedirect != nil {
				t.Fatalf("redirected requests=%d; caller client mutated=%t", redirected.Load(), client.CheckRedirect != nil)
			}
		})
	}
}

func TestReleaseAssetRedirectsKeepCredentialsAtTheInitialAuthority(t *testing.T) {
	var destinationCalls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		destinationCalls.Add(1)
		if request.Header.Get("Authorization") != "" || request.Header.Get("Job-Token") != "" {
			t.Error("asset redirect forwarded publication credentials")
		}
		if _, err := response.Write([]byte("verified artifact")); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(destination.Close)
	origin := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer synthetic-token" || request.Header.Get("Job-Token") != "synthetic-token" {
			t.Error("initial asset request lost publication credentials")
		}
		http.Redirect(response, request, destination.URL, http.StatusFound)
	}))
	t.Cleanup(origin.Close)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, origin.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer synthetic-token")
	request.Header.Set("Job-Token", "synthetic-token")
	client := origin.Client()
	data, err := responseBytes(client, request)
	if err != nil || string(data) != "verified artifact" || destinationCalls.Load() != 1 {
		t.Fatalf("asset redirect: data=%q error=%v destination=%d", data, err, destinationCalls.Load())
	}
	if client.CheckRedirect != nil {
		t.Fatal("asset verification mutated its caller's client")
	}
}

func TestGitHubAssetCredentialsBelongOnlyToTheSelectedAPI(t *testing.T) {
	version := "0.1.0"
	artifacts := releaseFixture(t, version)
	for _, trusted := range []bool{true, false} {
		t.Run(fmt.Sprint(trusted), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if (request.Header.Get("Authorization") == "Bearer synthetic-token") != trusted {
					t.Errorf("credential authority mismatch: trusted=%t", trusted)
				}
				http.ServeFile(response, request, filepath.Join(artifacts, filepath.Base(request.URL.Path)))
			}))
			t.Cleanup(server.Close)
			config := GitHubConfig{APIBase: server.URL, Tag: "v" + version, Token: "synthetic-token", Artifacts: artifacts}
			if !trusted {
				config.APIBase = "http://selected-api.invalid"
			}
			release := githubRelease{}
			for _, name := range artifact.Names(version) {
				release.Assets = append(release.Assets, struct {
					Name string `json:"name"`
					URL  string `json:"url"`
				}{Name: name, URL: server.URL + "/" + name})
			}
			if err := verifyGitHubAssets(t.Context(), server.Client(), config, release, version); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAssetRedirectsPreserveNativeLimitAndCallerPolicy(t *testing.T) {
	for _, policy := range []string{"default", "reject", "allow"} {
		t.Run(policy, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				requests.Add(1)
				http.Redirect(response, request, "/again", http.StatusFound)
			}))
			t.Cleanup(server.Close)
			client := server.Client()
			wantCalls := int32(10)
			cause := errors.New("caller rejected redirect")
			switch policy {
			case "reject":
				client.CheckRedirect = func(*http.Request, []*http.Request) error { return cause }
				wantCalls = 1
			case "allow":
				client.CheckRedirect = func(*http.Request, []*http.Request) error {
					if requests.Load() > 10 {
						return cause
					}
					return nil
				}
			}
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := responseBytes(client, request); err == nil || (policy == "reject" && !errors.Is(err, cause)) {
				t.Fatalf("redirect policy error=%v", err)
			}
			if requests.Load() != wantCalls {
				t.Fatalf("requests=%d, want %d", requests.Load(), wantCalls)
			}
		})
	}
}

func TestGitHubUploadRejectsInsecureOrAmbiguousMetadata(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	for _, endpoint := range []string{"http://uploads.example/asset", "https://user:password@uploads.example/asset", "/relative", "file:///asset"} {
		t.Run(endpoint, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return response(http.StatusCreated, ""), nil
			})}
			config := GitHubConfig{APIBase: "https://api.example", Tag: "v0.1.0", Token: "synthetic-token", Artifacts: artifacts}
			if err := uploadGitHubAssets(t.Context(), client, config, githubRelease{UploadURL: endpoint}); err == nil || calls != 0 {
				t.Fatalf("upload endpoint=%q error=%v calls=%d", endpoint, err, calls)
			}
		})
	}
}

func TestArtifactUploadsDoNotFollowRedirects(t *testing.T) {
	artifacts := releaseFixture(t, "0.1.0")
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			var calls atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				response.WriteHeader(http.StatusCreated)
			}))
			t.Cleanup(target.Close)
			origin := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				http.Redirect(response, request, target.URL, http.StatusTemporaryRedirect)
			}))
			t.Cleanup(origin.Close)
			var err error
			if provider == "github" {
				err = uploadGitHubAssets(t.Context(), origin.Client(), GitHubConfig{
					APIBase: origin.URL, Tag: "v0.1.0", Token: "synthetic-token", Artifacts: artifacts,
				}, githubRelease{UploadURL: origin.URL + "/uploads{?name,label}"})
			} else {
				err = UploadGitLab(t.Context(), origin.Client(), GitLabConfig{
					APIBase: origin.URL, ProjectID: "7", Tag: "v0.1.0", Token: "synthetic-token", Artifacts: artifacts,
					Trust: fixtureTrust(artifacts), Source: fixtureSource(artifacts),
				})
			}
			if err == nil || !strings.Contains(err.Error(), "HTTP 307") || calls.Load() != 0 {
				t.Fatalf("upload redirect error=%v target requests=%d", err, calls.Load())
			}
		})
	}
}

func TestAssetRedirectRejectsHTTPSDowngrade(t *testing.T) {
	var calls atomic.Int32
	plain := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	t.Cleanup(plain.Close)
	secure := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, plain.URL, http.StatusFound)
	}))
	t.Cleanup(secure.Close)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, secure.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := responseBytes(secure.Client(), request); err == nil || !strings.Contains(err.Error(), "downgrade HTTPS") {
		t.Fatalf("HTTPS downgrade error=%v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("HTTPS asset verification reached an insecure endpoint")
	}
}

func TestAssetCredentialsStayAbsentAfterReturningToOrigin(t *testing.T) {
	var originURL string
	foreign := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "" || request.Header.Get("Job-Token") != "" {
			t.Error("foreign hop received publication credentials")
		}
		http.Redirect(response, request, originURL+"/return", http.StatusFound)
	}))
	t.Cleanup(foreign.Close)
	origin := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/" {
			http.Redirect(response, request, foreign.URL, http.StatusFound)
			return
		}
		if request.Header.Get("Authorization") != "" || request.Header.Get("Job-Token") != "" {
			t.Error("return hop restored publication credentials")
		}
		if _, err := response.Write([]byte("artifact")); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(origin.Close)
	originURL = origin.URL
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, origin.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer synthetic-token")
	request.Header.Set("Job-Token", "synthetic-token")
	if data, err := responseBytes(origin.Client(), request); err != nil || string(data) != "artifact" {
		t.Fatalf("return hop: data=%q error=%v", data, err)
	}
}

func TestGitLabAssetVerificationRejectsInvalidAuthorityAndDownload(t *testing.T) {
	version := "0.1.0"
	expected := releaseDocument("v"+version, "https://example.test/packages")
	config := GitLabConfig{APIBase: "https://example.test", Tag: "v" + version, Token: "secret", Artifacts: releaseFixture(t, version)}

	relative := remoteRelease{TagName: config.Tag}
	for _, name := range artifact.Names(version) {
		relative.Assets.Links = append(relative.Assets.Links, struct {
			URL string `json:"url"`
		}{URL: name})
	}
	if err := verifyGitLabAssets(context.Background(), http.DefaultClient, config, expected, relative, version); err == nil || !strings.Contains(err.Error(), "invalid HTTP URL") {
		t.Fatalf("relative asset authority error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	remote := remoteRelease{TagName: config.Tag}
	for _, name := range artifact.Names(version) {
		remote.Assets.Links = append(remote.Assets.Links, struct {
			URL string `json:"url"`
		}{URL: server.URL + "/" + name})
	}
	config.APIBase = server.URL
	if err := verifyGitLabAssets(context.Background(), server.Client(), config, expected, remote, version); err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("asset download error = %v", err)
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

func TestAuthorityNormalizesDefaultPorts(t *testing.T) {
	same, err := assetAuthority("https://gitlab.example/api/v4", "https://gitlab.example:443/assets/a")
	if err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Fatal("default HTTPS port should identify the same authority")
	}
	got, err := authority("http://EXAMPLE.test./asset")
	if err != nil || got != "http://example.test:80" {
		t.Fatalf("authority=%q err=%v", got, err)
	}
}

func TestAuthorityRejectsInvalidInputs(t *testing.T) {
	if _, err := authority("https://example.test:bad"); err == nil {
		t.Fatal("invalid port accepted")
	}
	if _, err := assetAuthority("bad", "https://example.test"); err == nil {
		t.Fatal("invalid left authority accepted")
	}
	if _, err := assetAuthority("https://example.test", "bad"); err == nil {
		t.Fatal("invalid right authority accepted")
	}
}
