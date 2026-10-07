package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
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
			var requests []string
			server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet || request.Header.Get("Authorization") != test.authorization {
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				requests = append(requests, request.URL.Path)
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
					_, _ = fmt.Fprintf(writer, `{"total_count":1,"jobs":[{"name":"Quality and governance","run_id":20,"run_attempt":2,"head_sha":%q,"status":"completed","conclusion":%q}]}`, sha, test.conclusion)
				default:
					writer.WriteHeader(http.StatusNotFound)
				}
			}))
			t.Cleanup(server.Close)
			previous := http.DefaultTransport
			http.DefaultTransport = server.Client().Transport
			t.Cleanup(func() { http.DefaultTransport = previous })
			t.Setenv("GITHUB_API_URL", server.URL)
			t.Setenv("GH_TOKEN", test.primary)
			t.Setenv("GITHUB_TOKEN", test.fallback)
			var output bytes.Buffer
			err := run([]string{"release-evidence", "--repository", "example/aigw", "--workflow", "verify.yml", "--tag", "v0.3.1", "--sha", sha, "--job", "Quality and governance"}, &output, nil)
			if len(requests) != 2 || (err == nil) != test.valid {
				t.Fatalf("exact release evidence: requests=%v error=%v", requests, err)
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
