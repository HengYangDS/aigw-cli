package acceptance_test

import (
	"aigw-cli/internal/upgrade"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUpdateTokenFallbackRequiresExplicitHTTPSGitLabOrigin(t *testing.T) {
	for _, host := range []string{"", "http://gitlab.example.test"} {
		t.Run(host, func(t *testing.T) {
			if host == "" {
				t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "")
			} else {
				t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", host)
			}
			t.Setenv("GITLAB_TOKEN", "test-token")
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("token fallback attempted a request without an explicit HTTPS origin")
				return nil, nil
			})}
			u := upgrade.Updater{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				Executable: filepath.Join(t.TempDir(), "aigw"),
				Runner:     &unavailableRunner{},
				HTTPClient: client,
			}
			_, err := u.Update(context.Background(), "0.2.0")
			if err == nil || (!strings.Contains(err.Error(), "AIGW_GITLAB_RELEASE_ORIGIN") && !strings.Contains(err.Error(), "release source is incomplete") && !strings.Contains(err.Error(), "release origin")) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestUpdateRejectsControlCharacterTokenBeforeGitLabAPIRequest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("GitLab API was called with an invalid token")
	}))
	defer server.Close()
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("GITLAB_TOKEN", "test-token\ninjected")
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     &unavailableRunner{},
		HTTPClient: server.Client(),
	}
	_, err := u.Update(context.Background(), "0.2.0")
	if err == nil || !strings.Contains(err.Error(), "GITLAB_TOKEN contains a control character") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateDoesNotExposeTokenInGitLabAPIError(t *testing.T) {
	const token = "do-not-leak-this-token"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     &unavailableRunner{},
		HTTPClient: server.Client(),
	}
	_, err := u.Update(context.Background(), "0.2.0")
	if err == nil {
		t.Fatal("expected GitLab API error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("GitLab token leaked to error: %v", err)
	}
}

func TestUpdateRejectsGitLabHostWithCredentialsPathQueryOrFragment(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "test-token")
	for _, host := range []string{
		"https://user:password@gitlab.example.test",
		"https://gitlab.example.test/prefix",
		"https://gitlab.example.test?token=leak",
		"https://gitlab.example.test#fragment",
	} {
		t.Run(host, func(t *testing.T) {
			t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", host)
			u := upgrade.Updater{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				Executable: filepath.Join(t.TempDir(), "aigw"),
				Runner:     &unavailableRunner{},
			}
			_, err := u.Update(context.Background(), "0.2.0")
			if err == nil || !strings.Contains(err.Error(), "release origin") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestUpdateGitLabFallbackUsesBoundedHTTPClientTimeout(t *testing.T) {
	for name, client := range map[string]*http.Client{
		"default":         {},
		"caller deadline": {Timeout: 37 * time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			deadline := make(chan time.Duration, 1)
			client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
				when, ok := request.Context().Deadline()
				if !ok {
					t.Fatal("GitLab fallback request has no deadline")
				}
				deadline <- time.Until(when)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v0.2.0"}`)),
					Header:     make(http.Header),
					Request:    request,
				}, nil
			})
			t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", "https://gitlab.example.test")
			t.Setenv("GITLAB_TOKEN", "test-token")
			u := upgrade.Updater{
				GOOS:       "darwin",
				GOARCH:     "arm64",
				Executable: filepath.Join(t.TempDir(), "aigw"),
				Runner:     &unavailableRunner{},
				HTTPClient: client,
			}
			if _, err := u.Update(context.Background(), "0.2.0"); err != nil {
				t.Fatal(err)
			}
			remaining := <-deadline
			if remaining < 25*time.Second || remaining > 40*time.Second {
				t.Fatalf("request deadline remaining = %s, want a bounded 30s default or preserved 37s caller timeout", remaining)
			}
		})
	}
}

func TestUpdateKeepsExistingBinaryWhenGitLabAPIChecksumMismatches(t *testing.T) {
	const token = "test-token"
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Private-Token"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		switch r.URL.Path {
		case "/api/v4/projects/example-group/example-project/releases/permalink/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
		case "/example-group/example-project/-/releases/v0.2.0/downloads/" + archiveName:
			_, _ = w.Write(archive)
		case "/example-group/example-project/-/releases/v0.2.0/downloads/checksums.txt":
			_, _ = w.Write([]byte(strings.Repeat("0", 64) + "  ./" + archiveName + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", server.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: &unavailableRunner{}, HTTPClient: server.Client()}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v", err)
	}
	got, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old-binary" {
		t.Fatalf("old binary replaced after checksum failure: %q", got)
	}
}

func TestUpdateDoesNotForwardGitLabTokenAcrossReleaseRedirect(t *testing.T) {
	const token = "test-token"
	archive := tarGz(t, "aigw_0.2.0_darwin_arm64/aigw", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	archiveName := "aigw_0.2.0_darwin_arm64.tar.gz"
	forwardedToken := make(chan string, 1)
	downloadServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardedToken <- r.Header.Get("Private-Token")
		_, _ = w.Write(archive)
	}))
	defer downloadServer.Close()
	gitLabServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Private-Token"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		switch r.URL.Path {
		case "/api/v4/projects/example-group/example-project/releases/permalink/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
		case "/example-group/example-project/-/releases/v0.2.0/downloads/" + archiveName:
			http.Redirect(w, r, downloadServer.URL+"/asset", http.StatusFound)
		case "/example-group/example-project/-/releases/v0.2.0/downloads/checksums.txt":
			_, _ = fmt.Fprintf(w, "%x  ./%s\n", sum, archiveName)
		default:
			http.NotFound(w, r)
		}
	}))
	defer gitLabServer.Close()
	binary := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(binary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", gitLabServer.URL)
	t.Setenv("GITLAB_TOKEN", token)
	roots := x509.NewCertPool()
	roots.AddCert(gitLabServer.Certificate())
	roots.AddCert(downloadServer.Certificate())
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport}
	u := upgrade.Updater{GOOS: "darwin", GOARCH: "arm64", Executable: binary, Runner: &unavailableRunner{}, HTTPClient: client}
	if _, err := u.Update(context.Background(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	if got := <-forwardedToken; got != "" {
		t.Fatalf("GitLab token was forwarded to redirect target: %q", got)
	}
}

func TestUpdateRejectsHTTPSDowngradeRedirectBeforeFollowingIt(t *testing.T) {
	const token = "test-token"
	redirectTargetCalled := make(chan struct{}, 1)
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		redirectTargetCalled <- struct{}{}
	}))
	defer redirectTarget.Close()
	gitLabServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Private-Token"); got != token {
			t.Fatalf("PRIVATE-TOKEN = %q, want configured token", got)
		}
		switch r.URL.Path {
		case "/api/v4/projects/example-group/example-project/releases/permalink/latest":
			http.Redirect(w, r, redirectTarget.URL+"/latest", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer gitLabServer.Close()
	t.Setenv("AIGW_GITLAB_RELEASE_ORIGIN", gitLabServer.URL)
	t.Setenv("GITLAB_TOKEN", token)
	u := upgrade.Updater{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Runner:     &unavailableRunner{},
		HTTPClient: gitLabServer.Client(),
	}
	_, err := u.Update(context.Background(), "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "HTTPS to HTTP") {
		t.Fatalf("error = %v", err)
	}
	select {
	case <-redirectTargetCalled:
		t.Fatal("client followed HTTPS downgrade redirect")
	default:
	}
}
