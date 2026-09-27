package evidence

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const testSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

var requiredTagJobs = []string{
	"Quality and governance",
	"Native macOS acceptance",
	"Native Linux acceptance",
	"Native Windows acceptance",
	"Release version",
}

func tagRunFixture() string {
	return strings.ReplaceAll(`{"total_count":3,"workflow_runs":[
		{"id":10,"run_attempt":1,"head_branch":"main","head_sha":"SHA","path":".github/workflows/verify.yml","event":"push","status":"completed","conclusion":"success"},
		{"id":19,"run_attempt":1,"head_branch":"v0.3.1","head_sha":"SHA","path":".github/workflows/verify.yml","event":"push","status":"completed","conclusion":"success"},
		{"id":20,"run_attempt":2,"head_branch":"v0.3.1","head_sha":"SHA","path":".github/workflows/verify.yml","event":"push","status":"completed","conclusion":"success"}
	]}`, "SHA", testSHA)
}

func failedLatestRunFixture() string {
	runs := tagRunFixture()
	const success = `"conclusion":"success"`
	before, after, found := strings.CutLast(runs, success)
	if !found {
		panic("tag run fixture has no successful conclusion")
	}
	return before + `"conclusion":"failure"` + after
}

func tagJobsFixture() string {
	jobs := make([]string, 0, len(requiredTagJobs)+1)
	for _, name := range requiredTagJobs {
		jobs = append(jobs, fmt.Sprintf(`{"name":%q,"run_id":20,"run_attempt":2,"head_sha":%q,"status":"completed","conclusion":"success"}`, name, testSHA))
	}
	jobs = append(jobs, fmt.Sprintf(`{"name":"Accepted ref parity","run_id":20,"run_attempt":2,"head_sha":%q,"status":"completed","conclusion":"skipped"}`, testSHA))
	return fmt.Sprintf(`{"total_count":%d,"jobs":[%s]}`, len(jobs), strings.Join(jobs, ","))
}

func githubFixture(t *testing.T, runs, jobs string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer synthetic-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/repos/example/aigw/actions/workflows/verify.yml/runs":
			query := request.URL.Query()
			if query.Get("head_sha") != testSHA || query.Get("event") != "push" || query.Get("per_page") != "100" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.WriteHeader(status)
			_, _ = writer.Write([]byte(runs))
		case "/repos/example/aigw/actions/runs/20/attempts/2/jobs":
			if request.URL.Query().Get("per_page") != "100" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(jobs))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestGitHubTagEvidenceSelectsLatestExactTagAttempt(t *testing.T) {
	server := githubFixture(t, tagRunFixture(), tagJobsFixture(), http.StatusOK)
	defer server.Close()
	verifier := GitHubTagVerifier{Client: server.Client(), APIBase: server.URL, Repository: "example/aigw", Workflow: "verify.yml", Token: "synthetic-token"}
	result, err := verifier.Verify(context.Background(), "v0.3.1", testSHA, requiredTagJobs)
	if err != nil || result.RunID != 20 || result.Attempt != 2 {
		t.Fatalf("tag evidence = %+v, %v", result, err)
	}
}

func TestGitHubTagEvidenceRejectsStaleOrPartialProof(t *testing.T) {
	for _, test := range []struct {
		name   string
		runs   string
		jobs   string
		status int
	}{
		{name: "latest run failed", runs: failedLatestRunFixture(), jobs: tagJobsFixture(), status: http.StatusOK},
		{name: "wrong tag", runs: strings.ReplaceAll(tagRunFixture(), `"head_branch":"v0.3.1"`, `"head_branch":"main"`), jobs: tagJobsFixture(), status: http.StatusOK},
		{name: "wrong workflow", runs: strings.ReplaceAll(tagRunFixture(), `"path":".github/workflows/verify.yml"`, `"path":".github/workflows/other.yml"`), jobs: tagJobsFixture(), status: http.StatusOK},
		{name: "incomplete runs", runs: strings.Replace(tagRunFixture(), `"total_count":3`, `"total_count":0`, 1), jobs: tagJobsFixture(), status: http.StatusOK},
		{name: "missing native job", runs: tagRunFixture(), jobs: strings.Replace(tagJobsFixture(), `"name":"Native Windows acceptance"`, `"name":"other"`, 1), status: http.StatusOK},
		{name: "wrong job attempt", runs: tagRunFixture(), jobs: strings.Replace(tagJobsFixture(), `"name":"Native Linux acceptance","run_id":20,"run_attempt":2`, `"name":"Native Linux acceptance","run_id":20,"run_attempt":1`, 1), status: http.StatusOK},
		{name: "wrong job SHA", runs: tagRunFixture(), jobs: strings.Replace(tagJobsFixture(), `"name":"Release version","run_id":20,"run_attempt":2,"head_sha":"`+testSHA+`"`, `"name":"Release version","run_id":20,"run_attempt":2,"head_sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"`, 1), status: http.StatusOK},
		{name: "duplicate job", runs: tagRunFixture(), jobs: strings.Replace(tagJobsFixture(), `"name":"Accepted ref parity"`, `"name":"Release version"`, 1), status: http.StatusOK},
		{name: "incomplete jobs", runs: tagRunFixture(), jobs: strings.Replace(tagJobsFixture(), `"total_count":6`, `"total_count":0`, 1), status: http.StatusOK},
		{name: "GitHub denied", runs: tagRunFixture(), jobs: tagJobsFixture(), status: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := githubFixture(t, test.runs, test.jobs, test.status)
			defer server.Close()
			verifier := GitHubTagVerifier{Client: server.Client(), APIBase: server.URL, Repository: "example/aigw", Workflow: "verify.yml", Token: "synthetic-token"}
			if result, err := verifier.Verify(context.Background(), "v0.3.1", testSHA, requiredTagJobs); err == nil {
				t.Fatalf("unproved tag evidence accepted: %+v", result)
			}
		})
	}
}

func TestGitHubTagEvidenceDoesNotFollowAPIBaseRedirect(t *testing.T) {
	var followed atomic.Int32
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/leak" {
			followed.Add(1)
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(tagRunFixture()))
			return
		}
		http.Redirect(writer, request, server.URL+"/leak", http.StatusFound)
	}))
	defer server.Close()
	verifier := GitHubTagVerifier{Client: server.Client(), APIBase: server.URL, Repository: "example/aigw", Workflow: "verify.yml", Token: "synthetic-token"}
	if _, err := verifier.Verify(context.Background(), "v0.3.1", testSHA, requiredTagJobs); err == nil || followed.Load() != 0 {
		t.Fatalf("redirected API evidence was accepted or followed: %v, count=%d", err, followed.Load())
	}
}

func TestGitHubTagEvidenceRejectsInvalidInputs(t *testing.T) {
	for _, test := range []struct {
		name     string
		apiBase  string
		repo     string
		workflow string
		sha      string
		jobs     []string
	}{
		{name: "insecure API", apiBase: "http://example.test", repo: "example/aigw", workflow: "verify.yml", sha: testSHA, jobs: requiredTagJobs},
		{name: "bad repository", apiBase: "https://api.github.com", repo: "example", workflow: "verify.yml", sha: testSHA, jobs: requiredTagJobs},
		{name: "bad workflow", apiBase: "https://api.github.com", repo: "example/aigw", workflow: "../verify.yml", sha: testSHA, jobs: requiredTagJobs},
		{name: "bad SHA", apiBase: "https://api.github.com", repo: "example/aigw", workflow: "verify.yml", sha: "not-a-sha", jobs: requiredTagJobs},
		{name: "duplicate job", apiBase: "https://api.github.com", repo: "example/aigw", workflow: "verify.yml", sha: testSHA, jobs: []string{"quality", "quality"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			verifier := GitHubTagVerifier{APIBase: test.apiBase, Repository: test.repo, Workflow: test.workflow}
			if _, err := verifier.Verify(context.Background(), "v0.3.1", test.sha, test.jobs); err == nil {
				t.Fatal("invalid evidence request was accepted")
			}
		})
	}
}
