package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestForgeContracts(t *testing.T) {
	fixture := forgeFixture(t)
	if err := run([]string{"commits", "--repository", fixture.repository, "--provider", "gitlab", "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"tag", "--repository", fixture.repository, "--provider", "gitlab", "--tag", "v1.2.3", "--allowed-signers", fixture.allowedSigners}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"tags", "--repository", fixture.repository, "--mode", "gitlab", "--gitlab-allowed-signers", fixture.allowedSigners}); err != nil {
		t.Fatal(err)
	}
	gitTest(t, fixture.repository, "branch", "dev")
	if err := run([]string{"sync", "--repository", fixture.repository, "--canonical", "main", "--peer", "dev:dev:commit"}); err != nil {
		t.Fatal(err)
	}
	gitTest(t, fixture.repository, "branch", "work/test")
	if err := run([]string{"closeout", "--repository", fixture.repository, "--source", "work/test", "--canonical", "main", "--peer", "dev:dev:commit"}); err != nil {
		t.Fatal(err)
	}
}

func TestForgeContractsRejectInvalidInputs(t *testing.T) {
	fixture := forgeFixture(t)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"unknown", []string{"unknown"}, "unknown forge command"},
		{"commits usage", []string{"commits"}, "usage"},
		{"provider", []string{"commits", "--provider", "other", "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}, "provider"},
		{"email", []string{"commits", "--provider", "gitlab", "--email", "bad", "--allowed-signers", fixture.allowedSigners}, "malformed"},
		{"trust", []string{"commits", "--provider", "gitlab", "--email", fixture.email, "--allowed-signers", "missing"}, "missing"},
		{"tag provider", []string{"tag", "--provider", "other", "--tag", "v1.2.3", "--allowed-signers", fixture.allowedSigners}, "provider"},
		{"tag shape", []string{"tag", "--provider", "gitlab", "--tag", "latest", "--allowed-signers", fixture.allowedSigners}, "malformed"},
		{"qualified tag", []string{"tag", "--provider", "gitlab", "--tag", "github/v1.2.3", "--allowed-signers", fixture.allowedSigners}, "requires github"},
		{"tags mode", []string{"tags", "--mode", "other"}, "mode"},
		{"tags trust", []string{"tags", "--mode", "local"}, "GitLab trust"},
		{"sync peer", []string{"sync", "--repository", fixture.repository, "--peer", "bad"}, "peer specification"},
		{"closeout source", []string{"closeout", "--repository", fixture.repository, "--peer", "dev:main:commit"}, "usage"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := run(test.args); err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.want)) {
				t.Fatalf("error=%v want=%q", err, test.want)
			}
		})
	}
}

func TestForgeFlagParsersRejectMalformedFlags(t *testing.T) {
	for name, run := range map[string]func() error{
		"commits": func() error { return runCommitProvenance([]string{"--unknown"}) },
		"tag":     func() error { return runTagSignature([]string{"--unknown"}) },
		"tags":    func() error { return runTagNamespace([]string{"--unknown"}) },
		"sync":    func() error { return runSync([]string{"--unknown"}, false) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(); err == nil {
				t.Fatal("malformed flag accepted")
			}
		})
	}
}

func TestForgeRejectsProvenanceAndTopologyDrift(t *testing.T) {
	fixture := forgeFixture(t)
	gitTest(t, fixture.repository, "config", "user.email", "wrong@example.com")
	writeCommit(t, fixture.repository, "drift", "drift\n", "drift")
	if err := run([]string{"commits", "--repository", fixture.repository, "--provider", "gitlab", "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}); err == nil || !strings.Contains(err.Error(), "must use") {
		t.Fatalf("identity drift=%v", err)
	}

	fixture = forgeFixture(t)
	gitTest(t, fixture.repository, "branch", "dev")
	gitTest(t, fixture.repository, "checkout", "-q", "dev")
	writeCommit(t, fixture.repository, "dev", "dev\n", "dev")
	if err := run([]string{"sync", "--repository", fixture.repository, "--canonical", "main", "--peer", "dev:dev:commit"}); err == nil || !strings.Contains(err.Error(), "does not exactly match") {
		t.Fatalf("sync drift=%v", err)
	}
	if err := run([]string{"sync", "--repository", fixture.repository, "--canonical", "main", "--peer", "dev:dev:tree"}); err == nil || !strings.Contains(err.Error(), "ordered source-tree") {
		t.Fatalf("tree drift=%v", err)
	}
}

func TestForgeTagNamespaceBoundaries(t *testing.T) {
	fixture := forgeFixture(t)
	gitTest(t, fixture.repository, "tag", "-d", "v1.2.3")
	gitTest(t, fixture.repository, "-c", "user.signingkey="+fixture.key, "-c", "gpg.format=ssh", "tag", "-s", "-a", "github/v1.2.3", "-m", "github")
	if err := run([]string{"tags", "--repository", fixture.repository, "--mode", "gitlab", "--gitlab-allowed-signers", fixture.allowedSigners}); err == nil || !strings.Contains(err.Error(), "qualified GitHub provenance") {
		t.Fatalf("qualified topology=%v", err)
	}
	if err := run([]string{"tags", "--repository", fixture.repository, "--mode", "local", "--gitlab-allowed-signers", fixture.allowedSigners, "--github-allowed-signers", fixture.allowedSigners}); err != nil {
		t.Fatal(err)
	}
}

func TestCommitProvenanceRejectsRepositoryAndTrustDrift(t *testing.T) {
	fixture := forgeFixture(t)
	if err := os.WriteFile(filepath.Join(fixture.repository, ".mailmap"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runCommitProvenance([]string{"--repository", fixture.repository, "--provider", "gitlab", "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}); err == nil || !strings.Contains(err.Error(), ".mailmap") {
		t.Fatalf("mailmap=%v", err)
	}
	if err := os.Remove(filepath.Join(fixture.repository, ".mailmap")); err != nil {
		t.Fatal(err)
	}
	rogue := forgeKey(t, "rogue@example.invalid")
	if err := runCommitProvenance([]string{"--repository", fixture.repository, "--provider", "gitlab", "--email", fixture.email, "--allowed-signers", rogue.allowedSigners}); err == nil || !strings.Contains(err.Error(), "trusted signature") {
		t.Fatalf("untrusted=%v", err)
	}
	if err := runCommitProvenance([]string{"--repository", filepath.Join(t.TempDir(), "missing"), "--provider", "gitlab", "--email", fixture.email, "--allowed-signers", fixture.allowedSigners}); err == nil || !strings.Contains(err.Error(), "not a Git repository") {
		t.Fatalf("repository=%v", err)
	}
}

func TestTagSignatureRejectsMissingLightweightUnsignedAndUntrusted(t *testing.T) {
	fixture := forgeFixture(t)
	base := []string{"--repository", fixture.repository, "--provider", "gitlab", "--allowed-signers", fixture.allowedSigners}
	if err := runTagSignature(append(base, "--tag", "v9.9.9")); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing=%v", err)
	}
	gitTest(t, fixture.repository, "tag", "v1.2.4")
	if err := runTagSignature(append(base, "--tag", "v1.2.4")); err == nil || !strings.Contains(err.Error(), "annotated") {
		t.Fatalf("lightweight=%v", err)
	}
	gitTest(t, fixture.repository, "-c", "tag.gpgsign=false", "tag", "-a", "v1.2.5", "-m", "unsigned")
	if err := runTagSignature(append(base, "--tag", "v1.2.5")); err == nil || !strings.Contains(err.Error(), "not SSH signed") {
		t.Fatalf("unsigned=%v", err)
	}
	rogue := forgeKey(t, "rogue@example.invalid")
	if err := runTagSignature([]string{"--repository", fixture.repository, "--provider", "gitlab", "--tag", "v1.2.3", "--allowed-signers", rogue.allowedSigners}); err == nil {
		t.Fatal("untrusted tag accepted")
	}
	for _, mode := range []string{"rev-parse", "cat-file"} {
		t.Run(mode, func(t *testing.T) {
			fixture := forgeFixture(t)
			useGitWrapper(t, mode)
			if err := runTagSignature([]string{"--repository", fixture.repository, "--provider", "gitlab", "--tag", "v1.2.3", "--allowed-signers", fixture.allowedSigners}); err == nil {
				t.Fatalf("%s failure accepted", mode)
			}
		})
	}
}

func TestTagNamespaceRejectsUnexpectedAndUntrustedTags(t *testing.T) {
	fixture := forgeFixture(t)
	gitTest(t, fixture.repository, "tag", "unexpected")
	if err := runTagNamespace([]string{"--repository", fixture.repository, "--mode", "gitlab", "--gitlab-allowed-signers", fixture.allowedSigners}); err == nil || !strings.Contains(err.Error(), "unexpected release tag namespace") {
		t.Fatalf("unexpected=%v", err)
	}
	gitTest(t, fixture.repository, "tag", "-d", "unexpected")
	rogue := forgeKey(t, "rogue@example.invalid")
	if err := runTagNamespace([]string{"--repository", fixture.repository, "--mode", "gitlab", "--gitlab-allowed-signers", rogue.allowedSigners}); err == nil || !strings.Contains(err.Error(), "does not verify") {
		t.Fatalf("untrusted=%v", err)
	}
	if err := runTagNamespace([]string{"--repository", fixture.repository, "--mode", "github", "--github-allowed-signers", fixture.allowedSigners}); err != nil {
		t.Fatal(err)
	}
}

func TestSyncAndCloseoutRejectInvalidLifecycleState(t *testing.T) {
	fixture := forgeFixture(t)
	gitTest(t, fixture.repository, "branch", "work/test")
	if err := runSync([]string{"--repository", fixture.repository, "--source", "missing", "--canonical", "main", "--peer", "peer:main:commit"}, true); err == nil || !strings.Contains(err.Error(), "source ref") {
		t.Fatalf("missing source=%v", err)
	}
	if err := runSync([]string{"--repository", fixture.repository, "--source", "main", "--peer", "peer:main:commit"}, true); err == nil || !strings.Contains(err.Error(), "must differ") {
		t.Fatalf("same branch=%v", err)
	}
	gitTest(t, fixture.repository, "checkout", "-q", "work/test")
	if err := os.WriteFile(filepath.Join(fixture.repository, "dirty"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runSync([]string{"--repository", fixture.repository, "--source", "work/test", "--canonical", "main", "--peer", "peer:main:commit"}, true); err == nil || !strings.Contains(err.Error(), "not clean") {
		t.Fatalf("dirty=%v", err)
	}
	if err := os.Remove(filepath.Join(fixture.repository, "dirty")); err != nil {
		t.Fatal(err)
	}
	writeCommit(t, fixture.repository, "work", "work\n", "work")
	if err := runSync([]string{"--repository", fixture.repository, "--source", "work/test", "--canonical", "main", "--peer", "peer:main:commit"}, true); err == nil || !strings.Contains(err.Error(), "does not contain source tip") {
		t.Fatalf("unmerged=%v", err)
	}
	if _, err := localBranch(fixture.repository, "source", "missing"); err == nil {
		t.Fatal("missing branch accepted")
	}
}

func TestForgeGitFailureBoundaries(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T, forgeTestFixture) error
	}{
		{"commit-head", func(t *testing.T, f forgeTestFixture) error {
			useGitWrapper(t, "rev-parse")
			return runCommitProvenance([]string{"--repository", f.repository, "--provider", "gitlab", "--email", f.email, "--allowed-signers", f.allowedSigners})
		}},
		{"commit-list", func(t *testing.T, f forgeTestFixture) error {
			useGitWrapper(t, "rev-list")
			return runCommitProvenance([]string{"--repository", f.repository, "--provider", "gitlab", "--email", f.email, "--allowed-signers", f.allowedSigners})
		}},
		{"commit-show", func(t *testing.T, f forgeTestFixture) error {
			useGitWrapper(t, "show")
			return runCommitProvenance([]string{"--repository", f.repository, "--provider", "gitlab", "--email", f.email, "--allowed-signers", f.allowedSigners})
		}},
		{"tag-read", func(t *testing.T, f forgeTestFixture) error {
			useGitWrapper(t, "cat-file")
			return runTagSignature([]string{"--repository", f.repository, "--provider", "gitlab", "--tag", "v1.2.3", "--allowed-signers", f.allowedSigners})
		}},
		{"tag-list", func(t *testing.T, f forgeTestFixture) error {
			useGitWrapper(t, "for-each-ref")
			return runTagNamespace([]string{"--repository", f.repository, "--mode", "gitlab", "--gitlab-allowed-signers", f.allowedSigners})
		}},
		{"tags-list", func(t *testing.T, f forgeTestFixture) error {
			useGitWrapper(t, "for-each-ref")
			return runTagNamespace([]string{"--repository", f.repository, "--mode", "gitlab", "--gitlab-allowed-signers", f.allowedSigners})
		}},
		{"sync-canonical-ref", func(t *testing.T, f forgeTestFixture) error {
			useGitWrapper(t, "rev-parse-symbolic")
			return runSync([]string{"--repository", f.repository, "--peer", "peer:main:commit"}, false)
		}},
		{"sync-canonical-commit", func(t *testing.T, f forgeTestFixture) error {
			useGitWrapper(t, "rev-parse-commit")
			return runSync([]string{"--repository", f.repository, "--peer", "peer:main:commit"}, false)
		}},
		{"sync-canonical", func(t *testing.T, f forgeTestFixture) error {
			useGitWrapper(t, "log")
			return runSync([]string{"--repository", f.repository, "--peer", "peer:main:commit"}, false)
		}},
		{"sync-peer", func(t *testing.T, f forgeTestFixture) error {
			useGitWrapper(t, "rev-parse-peer")
			return runSync([]string{"--repository", f.repository, "--peer", "peer:main:commit"}, false)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := forgeFixture(t)
			if err := test.run(t, fixture); err == nil {
				t.Fatal("injected Git failure accepted")
			}
		})
	}
}

func TestCloseoutGitFailureBoundaries(t *testing.T) {
	for _, mode := range []string{"for-each-ref", "status", "merge-base", "log", "rev-parse-peer"} {
		t.Run(mode, func(t *testing.T) {
			fixture := forgeFixture(t)
			gitTest(t, fixture.repository, "branch", "work/test")
			gitTest(t, fixture.repository, "checkout", "-q", "work/test")
			useGitWrapper(t, mode)
			if err := runSync([]string{"--repository", fixture.repository, "--source", "work/test", "--canonical", "main", "--peer", "peer:main:commit"}, true); err == nil {
				t.Fatalf("%s failure accepted", mode)
			}
		})
	}
}

type forgeTestFixture struct{ repository, email, key, allowedSigners string }

type forgeTestKey struct{ key, allowedSigners string }

func forgeKey(t *testing.T, email string) forgeTestKey {
	t.Helper()
	key := filepath.Join(t.TempDir(), "signing")
	runExternal(t, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key)
	public, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(public))
	allowed := filepath.Join(t.TempDir(), "allowed-signers")
	if err := os.WriteFile(allowed, []byte(email+" namespaces=\"git\" "+fields[0]+" "+fields[1]+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return forgeTestKey{key: key, allowedSigners: allowed}
}

func forgeFixture(t *testing.T) forgeTestFixture {
	t.Helper()
	repository := t.TempDir()
	identity := forgeKey(t, "forge@example.invalid")
	key := identity.key
	gitTest(t, repository, "init", "-q", "-b", "main")
	gitTest(t, repository, "config", "user.name", "Forge Fixture")
	email := "forge@example.invalid"
	gitTest(t, repository, "config", "user.email", email)
	gitTest(t, repository, "config", "gpg.format", "ssh")
	gitTest(t, repository, "config", "gpg.ssh.program", "ssh-keygen")
	gitTest(t, repository, "config", "core.hooksPath", filepath.Join(repository, ".disabled-hooks"))
	gitTest(t, repository, "config", "commit.gpgsign", "true")
	gitTest(t, repository, "config", "user.signingkey", key)
	allowed := identity.allowedSigners
	writeCommit(t, repository, "file", "value\n", "initial")
	gitTest(t, repository, "tag", "-s", "-a", "v1.2.3", "-m", "release")
	return forgeTestFixture{repository: repository, email: email, key: key, allowedSigners: allowed}
}

func runExternal(t *testing.T, name string, args ...string) {
	t.Helper()
	if output, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v: %s", name, err, output)
	}
}
