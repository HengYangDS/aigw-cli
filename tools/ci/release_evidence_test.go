package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
)

func TestReleaseEvidenceCommandRequiresExactInputs(t *testing.T) {
	for _, arguments := range [][]string{
		{"release-evidence"},
		{"release-evidence", "--repository", "example/aigw", "--tag", "v0.3.1", "--sha", strings.Repeat("a", 40), "--workflow", "verify.yml"},
		{"release-evidence", "--repository", "example/aigw", "--tag", "v0.3.1", "--sha", strings.Repeat("a", 40), "--workflow", "verify.yml", "--job", "Quality and governance", "extra"},
	} {
		if err := run(arguments, &bytes.Buffer{}, nil); err == nil || !strings.Contains(err.Error(), "usage: ci release-evidence") {
			t.Fatalf("arguments %q were admitted: %v", arguments, err)
		}
	}
}

func TestReleaseEvidenceCommandForwardsExactInputs(t *testing.T) {
	const sha = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, test := range []struct {
		name, primary, fallback, authorization, conclusion string
		valid                                              bool
	}{
		{"selected Token", "synthetic-primary", "synthetic-fallback", "Bearer synthetic-primary", "success", true},
		{"fallback Token", "", "synthetic-fallback", "Bearer synthetic-fallback", "success", true},
		{"public evidence", "", "", "", "success", true},
		{"unsuccessful named job", "synthetic-primary", "", "Bearer synthetic-primary", "failure", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := serveReleaseEvidence(t, sha, test.authorization, test.conclusion)
			t.Setenv("GH_TOKEN", test.primary)
			t.Setenv("GITHUB_TOKEN", test.fallback)
			var output bytes.Buffer
			err := run([]string{"release-evidence", "--repository", "example/aigw", "--workflow", "verify.yml", "--tag", "v0.3.1", "--sha", sha, "--job", "Quality and governance"}, &output, nil)
			if len(*requests) != 2 || (err == nil) != test.valid {
				t.Fatalf("exact release evidence: requests=%v error=%v", *requests, err)
			}
			if test.valid && output.String() != "GitHub release evidence verified: run 20 attempt 2\n" {
				t.Fatalf("success evidence = %q", output.String())
			}
			if !test.valid && (output.Len() != 0 || !strings.Contains(err.Error(), "GitHub release evidence:")) {
				t.Fatalf("failed evidence was not propagated: output=%q error=%v", output.String(), err)
			}
		})
	}
}

func TestReleaseEvidenceResolvesTheSelectedTagCommit(t *testing.T) {
	repository := newGitRepository(t)
	git := func(arguments ...string) string {
		output, err := exec.Command("git", append([]string{"-C", repository}, arguments...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %q: %v\n%s", arguments, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("-c", "core.hooksPath=", "-c", "commit.gpgsign=false", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "--allow-empty", "-m", "fixture")
	git("tag", "v0.3.1")
	requests := serveReleaseEvidence(t, git("rev-parse", "HEAD"), "Bearer synthetic-primary", "success")
	t.Setenv("GH_TOKEN", "synthetic-primary")
	t.Chdir(repository)
	var output bytes.Buffer
	if err := run([]string{"release-evidence", "--repository", "example/aigw", "--workflow", "verify.yml", "--tag", "v0.3.1", "--job", "Quality and governance"}, &output, nil); err != nil || len(*requests) != 2 {
		t.Fatalf("tag release evidence: requests=%v error=%v", *requests, err)
	}
	if output.String() != "GitHub release evidence verified: run 20 attempt 2\n" {
		t.Fatalf("tag release evidence = %q", output.String())
	}
	*requests = nil
	err := run([]string{"release-evidence", "--repository", "example/aigw", "--workflow", "verify.yml", "--tag", "v0.3.2", "--job", "Quality and governance"}, &output, nil)
	if err == nil || !strings.Contains(err.Error(), "cannot resolve selected release tag v0.3.2") || len(*requests) != 0 {
		t.Fatalf("unresolved release tag reached GitHub: requests=%v error=%v", *requests, err)
	}
}

// serveReleaseEvidence answers the GitHub workflow inventory for one tag commit
// and returns the request paths it served.
func serveReleaseEvidence(t *testing.T, sha, authorization, conclusion string) *[]string {
	t.Helper()
	requests := &[]string{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.Header.Get("Authorization") != authorization {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		*requests = append(*requests, request.URL.Path)
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/repos/example/aigw/actions/workflows/verify.yml/runs":
			query := request.URL.Query()
			if query.Get("head_sha") != sha || query.Get("event") != "push" || query.Get("per_page") != "100" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = fmt.Fprintf(writer, `{"total_count":1,"workflow_runs":[{"id":20,"run_attempt":2,"head_branch":"v0.3.1","head_sha":%q,"path":".github/workflows/verify.yml","event":"push","status":"completed","conclusion":"success"}]}`, sha)
		case "/repos/example/aigw/actions/runs/20/attempts/2/jobs":
			if request.URL.Query().Get("per_page") != "100" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = fmt.Fprintf(writer, `{"total_count":1,"jobs":[{"name":"Quality and governance","run_id":20,"run_attempt":2,"head_sha":%q,"status":"completed","conclusion":%q}]}`, sha, conclusion)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	previous := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	t.Setenv("GITHUB_API_URL", server.URL)
	return requests
}
