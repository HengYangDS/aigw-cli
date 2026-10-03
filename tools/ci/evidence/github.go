// Package evidence admits exact peer-local hosted verification without
// substituting another Forge's result or repeating native jobs.
package evidence

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// GitHubTagVerifier reads one selected repository's completed tag workflow.
type GitHubTagVerifier struct {
	Client     *http.Client
	APIBase    string
	Repository string
	Workflow   string
	Token      string
}

// TagEvidence identifies the exact successful workflow attempt admitted.
type TagEvidence struct {
	RunID   int64
	Attempt int
}

type workflowRun struct {
	ID         int64  `json:"id"`
	Attempt    int    `json:"run_attempt"`
	HeadBranch string `json:"head_branch"`
	HeadSHA    string `json:"head_sha"`
	Path       string `json:"path"`
	Event      string `json:"event"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

type workflowJob struct {
	Name       string `json:"name"`
	RunID      int64  `json:"run_id"`
	Attempt    int    `json:"run_attempt"`
	HeadSHA    string `json:"head_sha"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// Verify requires the latest tag-push run and every named job to succeed on
// the exact release commit and current run attempt.
func (verifier GitHubTagVerifier) Verify(ctx context.Context, tag, sha string, required []string) (TagEvidence, error) {
	base, owner, repository, err := verifier.validatedInputs(tag, sha, required)
	if err != nil {
		return TagEvidence{}, err
	}
	runsURL := base.JoinPath("repos", owner, repository, "actions", "workflows", verifier.Workflow, "runs")
	query := runsURL.Query()
	query.Set("head_sha", sha)
	query.Set("event", "push")
	query.Set("per_page", "100")
	runsURL.RawQuery = query.Encode()
	var runs struct {
		TotalCount   *int          `json:"total_count"`
		WorkflowRuns []workflowRun `json:"workflow_runs"`
	}
	if err := verifier.getJSON(ctx, runsURL.String(), &runs); err != nil {
		return TagEvidence{}, fmt.Errorf("read GitHub tag workflow: %w", err)
	}
	if runs.TotalCount == nil || *runs.TotalCount != len(runs.WorkflowRuns) {
		return TagEvidence{}, errors.New("GitHub tag workflow inventory is incomplete")
	}
	selected, err := latestTagRun(runs.WorkflowRuns, tag, sha, verifier.Workflow)
	if err != nil {
		return TagEvidence{}, err
	}
	jobsURL := base.JoinPath("repos", owner, repository, "actions", "runs", strconv.FormatInt(selected.ID, 10), "attempts", strconv.Itoa(selected.Attempt), "jobs")
	query = jobsURL.Query()
	query.Set("per_page", "100")
	jobsURL.RawQuery = query.Encode()
	var jobs struct {
		TotalCount *int          `json:"total_count"`
		Jobs       []workflowJob `json:"jobs"`
	}
	if err := verifier.getJSON(ctx, jobsURL.String(), &jobs); err != nil {
		return TagEvidence{}, fmt.Errorf("read GitHub tag jobs: %w", err)
	}
	if jobs.TotalCount == nil || *jobs.TotalCount != len(jobs.Jobs) {
		return TagEvidence{}, errors.New("GitHub tag job inventory is incomplete")
	}
	if err := verifyTagJobs(jobs.Jobs, selected, sha, required); err != nil {
		return TagEvidence{}, err
	}
	return TagEvidence{RunID: selected.ID, Attempt: selected.Attempt}, nil
}

func latestTagRun(runs []workflowRun, tag, sha, workflow string) (workflowRun, error) {
	var selected workflowRun
	for _, run := range runs {
		if run.HeadBranch == tag && run.HeadSHA == sha && run.Path == ".github/workflows/"+workflow && run.Event == "push" && run.ID > selected.ID {
			selected = run
		}
	}
	if selected.ID == 0 || selected.Attempt < 1 || selected.Status != "completed" || selected.Conclusion != "success" {
		return workflowRun{}, errors.New("latest exact-tag GitHub verification is unavailable or unsuccessful")
	}
	return selected, nil
}

func verifyTagJobs(jobs []workflowJob, run workflowRun, sha string, required []string) error {
	needed := make(map[string]bool, len(required))
	for _, name := range required {
		needed[name] = false
	}
	for _, job := range jobs {
		seen, expected := needed[job.Name]
		if !expected {
			continue
		}
		if seen || job.RunID != run.ID || job.Attempt != run.Attempt || job.HeadSHA != sha || job.Status != "completed" || job.Conclusion != "success" {
			return fmt.Errorf("GitHub tag job %q is duplicated or unproved", job.Name)
		}
		needed[job.Name] = true
	}
	for name, seen := range needed {
		if !seen {
			return fmt.Errorf("GitHub tag job %q is missing", name)
		}
	}
	return nil
}

func (verifier GitHubTagVerifier) validatedInputs(tag, sha string, required []string) (*url.URL, string, string, error) {
	base, err := url.Parse(verifier.APIBase)
	if err != nil || base.Scheme != "https" || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, "", "", errors.New("GitHub API endpoint must be HTTPS")
	}
	owner, repository, ok := strings.Cut(verifier.Repository, "/")
	if !ok || owner == "" || repository == "" || strings.Contains(repository, "/") || owner == "." || owner == ".." || repository == "." || repository == ".." {
		return nil, "", "", errors.New("GitHub repository must be owner/name")
	}
	if verifier.Workflow == "" || strings.ContainsAny(verifier.Workflow, "/\\") || !strings.HasSuffix(verifier.Workflow, ".yml") {
		return nil, "", "", errors.New("GitHub workflow must be one YAML filename")
	}
	if err := validateReleaseIdentity(tag, sha); err != nil {
		return nil, "", "", err
	}
	if err := validateJobNames(required); err != nil {
		return nil, "", "", err
	}
	return base, owner, repository, nil
}

func validateReleaseIdentity(tag, sha string) error {
	if !strings.HasPrefix(tag, "v") || strings.ContainsAny(tag, "/\\ \t\n") || (len(sha) != 40 && len(sha) != 64) {
		return errors.New("release tag or commit identity is invalid")
	}
	if _, err := hex.DecodeString(sha); err != nil {
		return errors.New("release commit identity is invalid")
	}
	return nil
}

func validateJobNames(required []string) error {
	if len(required) == 0 {
		return errors.New("release evidence requires named jobs")
	}
	seen := make(map[string]bool, len(required))
	for _, name := range required {
		if strings.TrimSpace(name) != name || name == "" || seen[name] {
			return errors.New("release evidence job names must be distinct and nonempty")
		}
		seen[name] = true
	}
	return nil
}

func (verifier GitHubTagVerifier) getJSON(ctx context.Context, endpoint string, target any) (result error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-Github-Api-Version", "2022-11-28")
	if verifier.Token != "" {
		request.Header.Set("Authorization", "Bearer "+verifier.Token)
	}
	client := verifier.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	bounded := *client
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := bounded.Do(request)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, response.Body.Close()) }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(target); err != nil {
		return fmt.Errorf("decode GitHub API response: %w", err)
	}
	return nil
}
