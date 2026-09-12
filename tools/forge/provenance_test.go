package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductObjectVerificationHasNoForgeIdentity(t *testing.T) {
	fixture := newForgeFixture(t)

	if err := run([]string{
		"commits", "--repository", fixture.repository,
		"--email", fixture.email,
		"--allowed-signers", fixture.allowedSigners,
	}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{
		"tag", "--repository", fixture.repository,
		"--tag", "v1.2.3",
		"--allowed-signers", fixture.allowedSigners,
	}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{
		"tags", "--repository", fixture.repository,
		"--allowed-signers", fixture.allowedSigners,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCommitVerificationRejectsInvalidInputsAndHistory(t *testing.T) {
	fixture := newForgeFixture(t)
	for name, arguments := range map[string][]string{
		"usage":      {"commits"},
		"email":      {"commits", "--email", "invalid", "--allowed-signers", fixture.allowedSigners},
		"trust":      {"commits", "--email", fixture.email, "--allowed-signers", "missing"},
		"repository": {"commits", "--repository", t.TempDir(), "--email", fixture.email, "--allowed-signers", fixture.allowedSigners},
		"revision":   {"commits", "--repository", fixture.repository, "--revision", "missing", "--email", fixture.email, "--allowed-signers", fixture.allowedSigners},
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(arguments); err == nil {
				t.Fatal("invalid commit verification accepted")
			}
		})
	}
	writeCommitForTest(t, fixture.repository, ".mailmap", "x\n")
	if err := run([]string{"commits", "--repository", fixture.repository, "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}); err == nil || !strings.Contains(err.Error(), ".mailmap") {
		t.Fatalf("mailmap: %v", err)
	}
	if err := os.Remove(filepath.Join(fixture.repository, ".mailmap")); err != nil {
		t.Fatal(err)
	}
	gitTest(t, fixture.repository, "add", ".mailmap")
	gitTest(t, fixture.repository, "config", "user.email", "wrong@example.invalid")
	writeCommitForTest(t, fixture.repository, "drift", "drift\n")
	if err := run([]string{"commits", "--repository", fixture.repository, "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}); err == nil || !strings.Contains(err.Error(), "must use") {
		t.Fatalf("identity drift: %v", err)
	}
}

func TestCommitVerificationEnforcesTrackedSubjectPolicyOnTheIntegrationRange(t *testing.T) {
	fixture := newForgeFixture(t)
	base := gitOutputForTest(t, fixture.repository, "rev-parse", "HEAD")
	policy := `[commit_policy]
subject_pattern = "^(feat|fix|docs|test|refactor|perf|build|ci|chore|revert)(\\([a-z0-9-]+\\))?!?: .+"
signing_required = true
signing_format = "ssh"
`
	if err := os.MkdirAll(filepath.Join(fixture.repository, ".ethos"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.repository, ".ethos", "workspace.toml"), []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	writeCommitWithSubjectForTest(t, fixture.repository, "plain", "plain\n", "fix: accept a plain subject")
	writeCommitWithSubjectForTest(t, fixture.repository, "scoped", "scoped\n", "docs(cli): accept a scoped subject")
	writeCommitWithSubjectForTest(t, fixture.repository, "breaking", "breaking\n", "feat(setup)!: accept a breaking subject")

	arguments := []string{
		"commits", "--repository", fixture.repository,
		"--base", base,
		"--email", fixture.email,
		"--allowed-signers", fixture.allowedSigners,
	}
	if err := run(arguments); err != nil {
		t.Fatalf("valid range: %v", err)
	}

	invalidBase := gitOutputForTest(t, fixture.repository, "rev-parse", "HEAD")
	writeCommitWithSubjectForTest(t, fixture.repository, "invalid", "invalid\n", "update everything")
	arguments = []string{
		"commits", "--repository", fixture.repository,
		"--base", invalidBase,
		"--email", fixture.email,
		"--allowed-signers", fixture.allowedSigners,
	}
	if err := run(arguments); err == nil || !strings.Contains(err.Error(), "subject") {
		t.Fatalf("invalid subject: %v", err)
	}
}

func TestCommitVerificationDoesNotApplyNewPolicyToHistoryBeforeBase(t *testing.T) {
	fixture := newForgeFixture(t)
	writeCommitWithSubjectForTest(t, fixture.repository, "legacy", "legacy\n", "old history")
	base := gitOutputForTest(t, fixture.repository, "rev-parse", "HEAD")
	policy := `[commit_policy]
subject_pattern = "^fix: .+"
signing_required = true
signing_format = "ssh"
`
	writeCommitWithSubjectForTest(t, fixture.repository, ".ethos/workspace.toml", policy, "fix: verify only the integration range")
	if err := os.WriteFile(filepath.Join(fixture.repository, ".mailmap"), []byte("untracked checkout metadata\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{
		"commits", "--repository", fixture.repository,
		"--base", base,
		"--email", fixture.email,
		"--allowed-signers", fixture.allowedSigners,
	}); err != nil {
		t.Fatalf("legacy history before base blocked the integration range: %v", err)
	}
}

func TestCommitVerificationRejectsMissingOrUnrelatedBase(t *testing.T) {
	fixture := newForgeFixture(t)
	other := newForgeFixture(t)
	for name, base := range map[string]string{
		"missing":   "missing",
		"unrelated": gitOutputForTest(t, other.repository, "rev-parse", "HEAD"),
	} {
		t.Run(name, func(t *testing.T) {
			if err := run([]string{
				"commits", "--repository", fixture.repository,
				"--base", base,
				"--email", fixture.email,
				"--allowed-signers", fixture.allowedSigners,
			}); err == nil || !strings.Contains(err.Error(), "base") {
				t.Fatalf("base %q: %v", name, err)
			}
		})
	}
}

func TestGitObjectOperationsUseTheSelectedRevisionPolicy(t *testing.T) {
	for _, operation := range []string{"commits", "project"} {
		t.Run(operation, func(t *testing.T) {
			fixture := newForgeFixture(t)
			remote := newBareRepository(t)
			gitTest(t, fixture.repository, "remote", "add", "peer", remote)
			selected := gitOutputForTest(t, fixture.repository, "rev-parse", "main")
			gitTest(t, fixture.repository, "checkout", "-q", "-b", "other")
			policy := `[commit_policy]
subject_pattern = "^chore: .+"
signing_required = true
signing_format = "ssh"
`
			writeCommitWithSubjectForTest(t, fixture.repository, ".ethos/workspace.toml", policy, "chore: change another branch policy")
			arguments := []string{operation, "--repository", fixture.repository, "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}
			if operation == "commits" {
				arguments = append(arguments, "--revision", selected)
			} else {
				arguments = append(arguments, "--source", "main", "--remote", "peer")
			}
			if err := run(arguments); err != nil {
				t.Fatalf("selected revision was checked against another checkout: %v", err)
			}
			if operation == "project" {
				for _, branch := range []string{"main", "dev"} {
					if got := gitOutputForTest(t, remote, "rev-parse", branch); got != selected {
						t.Fatalf("published %s = %s, want %s", branch, got, selected)
					}
				}
			}
		})
	}
}

func TestCommitVerificationKeepsPolicyInTheVerifiedObject(t *testing.T) {
	fixture := newForgeFixture(t)
	base := gitOutputForTest(t, fixture.repository, "rev-parse", "HEAD")
	writeCommitWithSubjectForTest(t, fixture.repository, "invalid", "invalid\n", "update everything")
	policy := `[commit_policy]
subject_pattern = ".*"
signing_required = true
signing_format = "ssh"
`
	if err := os.WriteFile(filepath.Join(fixture.repository, ".ethos", "workspace.toml"), []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{
		"commits", "--repository", fixture.repository, "--base", base,
		"--email", fixture.email, "--allowed-signers", fixture.allowedSigners,
	}); err == nil || !strings.Contains(err.Error(), "subject") {
		t.Fatalf("uncommitted policy replaced the verified policy: %v", err)
	}
}

func TestCommitVerificationRequiresAUsableTrackedPolicy(t *testing.T) {
	fixture := newForgeFixture(t)
	original, err := os.ReadFile(filepath.Join(fixture.repository, ".ethos", "workspace.toml"))
	if err != nil {
		t.Fatal(err)
	}
	for name, entry := range map[string]struct {
		policy string
		want   string
	}{
		"syntax": {"[commit_policy", "parse commit policy"},
		"pattern": {
			strings.Replace(string(original), "^(", "[", 1),
			"compile commit policy",
		},
		"empty": {"[commit_policy]\nsubject_pattern = ''", "subject_pattern"},
		"signing": {
			strings.Replace(string(original), "signing_required = true", "signing_required = false", 1),
			"require SSH signing",
		},
		"format": {
			strings.Replace(string(original), `signing_format = "ssh"`, `signing_format = "openpgp"`, 1),
			"require SSH signing",
		},
	} {
		t.Run(name, func(t *testing.T) {
			base := gitOutputForTest(t, fixture.repository, "rev-parse", "HEAD")
			writeCommitWithSubjectForTest(t, fixture.repository, ".ethos/workspace.toml", entry.policy, "test: exercise tracked policy admission")
			if err := run([]string{
				"commits", "--repository", fixture.repository, "--base", base,
				"--email", fixture.email, "--allowed-signers", fixture.allowedSigners,
			}); err == nil || !strings.Contains(err.Error(), entry.want) {
				t.Fatalf("tracked policy admission = %v, want %s", err, entry.want)
			}
		})
	}
	gitTest(t, fixture.repository, "rm", ".ethos/workspace.toml")
	gitTest(t, fixture.repository, "commit", "-q", "-m", "test: remove tracked policy")
	if err := os.MkdirAll(filepath.Join(fixture.repository, ".ethos"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.repository, ".ethos", "workspace.toml"), original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{
		"commits", "--repository", fixture.repository,
		"--email", fixture.email, "--allowed-signers", fixture.allowedSigners,
	}); err == nil || !strings.Contains(err.Error(), "read commit policy") {
		t.Fatalf("untracked policy substituted for missing object policy: %v", err)
	}
}

func TestCommitSubjectPolicyMatchesTheCompleteSubject(t *testing.T) {
	fixture := newForgeFixture(t)
	base := gitOutputForTest(t, fixture.repository, "rev-parse", "HEAD")
	policy := `[commit_policy]
subject_pattern = "fix: accepted|docs: accepted"
signing_required = true
signing_format = "ssh"
`
	writeCommitWithSubjectForTest(t, fixture.repository, ".ethos/workspace.toml", policy, "fix: accepted")
	if _, err := verifyCommits(fixture.repository, "HEAD", base, fixture.email, fixture.allowedSigners); err != nil {
		t.Fatalf("complete matching subject: %v", err)
	}
	for _, subject := range []string{"prefix fix: accepted", "docs: accepted suffix"} {
		base = gitOutputForTest(t, fixture.repository, "rev-parse", "HEAD")
		writeCommitWithSubjectForTest(t, fixture.repository, "change", subject, subject)
		if _, err := verifyCommits(fixture.repository, "HEAD", base, fixture.email, fixture.allowedSigners); err == nil || !strings.Contains(err.Error(), "subject") {
			t.Fatalf("partial subject %q was not rejected: %v", subject, err)
		}
	}
}

func TestReleaseTagUsesTheSharedSemanticVersionGrammar(t *testing.T) {
	fixture := newForgeFixture(t)
	for _, test := range []struct {
		tag   string
		valid bool
	}{
		{"v1.2.3-rc.1", true},
		{"v1.2.3+build.7", true},
		{"v1.2.3-01", false},
		{"v01.2.3", false},
		{"1.2.3", false},
	} {
		t.Run(test.tag, func(t *testing.T) {
			gitTest(t, fixture.repository, "tag", "-s", "-a", test.tag, "-m", "release "+test.tag)
			t.Cleanup(func() { gitTest(t, fixture.repository, "tag", "-d", test.tag) })
			err := verifyTag(fixture.repository, test.tag, fixture.allowedSigners)
			if (err == nil) != test.valid {
				t.Fatalf("tag %q valid=%t: %v", test.tag, test.valid, err)
			}
			err = runTagSetVerification([]string{"--repository", fixture.repository, "--allowed-signers", fixture.allowedSigners})
			if (err == nil) != test.valid {
				t.Fatalf("tag set containing %q valid=%t: %v", test.tag, test.valid, err)
			}
		})
	}
}

func TestTagVerificationRejectsInvalidShapesAndTrust(t *testing.T) {
	fixture := newForgeFixture(t)
	for name, arguments := range map[string][]string{
		"usage":      {"tag"},
		"malformed":  {"tag", "--repository", fixture.repository, "--tag", "latest", "--allowed-signers", fixture.allowedSigners},
		"missing":    {"tag", "--repository", fixture.repository, "--tag", "v9.9.9", "--allowed-signers", fixture.allowedSigners},
		"trust":      {"tag", "--repository", fixture.repository, "--tag", "v1.2.3", "--allowed-signers", "missing"},
		"tags usage": {"tags"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(arguments); err == nil {
				t.Fatal("invalid tag verification accepted")
			}
		})
	}
	gitTest(t, fixture.repository, "-c", "tag.gpgsign=false", "tag", "v1.2.4")
	if err := run([]string{"tag", "--repository", fixture.repository, "--tag", "v1.2.4", "--allowed-signers", fixture.allowedSigners}); err == nil || !strings.Contains(err.Error(), "annotated") {
		t.Fatalf("lightweight tag: %v", err)
	}
	rogue := newSigningIdentity(t, "rogue@example.invalid")
	if err := run([]string{"tag", "--repository", fixture.repository, "--tag", "v1.2.3", "--allowed-signers", rogue.allowedSigners}); err == nil || !strings.Contains(err.Error(), "trusted signature") {
		t.Fatalf("untrusted tag: %v", err)
	}
}

func TestCommitVerificationRejectsUntrustedSignature(t *testing.T) {
	fixture := newForgeFixture(t)
	rogue := newSigningIdentity(t, fixture.email)
	if err := run([]string{"commits", "--repository", fixture.repository, "--email", fixture.email, "--allowed-signers", rogue.allowedSigners}); err == nil || !strings.Contains(err.Error(), "trusted signature") {
		t.Fatalf("untrusted commit: %v", err)
	}
}

func TestEmptyTagSetAndUntrustedTagSet(t *testing.T) {
	fixture := newForgeFixture(t)
	gitTest(t, fixture.repository, "tag", "-d", "v1.2.3")
	if err := run([]string{"tags", "--repository", fixture.repository, "--allowed-signers", fixture.allowedSigners}); err != nil {
		t.Fatalf("empty tag set: %v", err)
	}
	gitTest(t, fixture.repository, "tag", "-s", "-a", "v1.2.3", "-m", "release")
	rogue := newSigningIdentity(t, fixture.email)
	if err := run([]string{"tags", "--repository", fixture.repository, "--allowed-signers", rogue.allowedSigners}); err == nil || !strings.Contains(err.Error(), "trusted signature") {
		t.Fatalf("untrusted tag set: %v", err)
	}
}
