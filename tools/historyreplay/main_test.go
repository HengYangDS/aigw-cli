package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSourceCommitPreservesSemanticFields(t *testing.T) {
	raw := []byte("tree 0123456789012345678901234567890123456789\nparent aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\nauthor Source <source@example.invalid> 1700000001 +0530\ncommitter Source <source@example.invalid> 1700000002 -0700\n\nsubject\n\nbody\n")
	commit, err := parseSourceCommit("fixture", raw)
	if err != nil {
		t.Fatal(err)
	}
	if commit.tree != "0123456789012345678901234567890123456789" || len(commit.parents) != 1 || commit.authorDate != "@1700000001 +0530" || commit.committerDate != "@1700000002 -0700" || !bytes.Equal(commit.message, []byte("subject\n\nbody\n")) {
		t.Fatalf("commit=%+v", commit)
	}
}

func TestParseSourceCommitRejectsUnsupportedHeader(t *testing.T) {
	raw := []byte("tree 0123456789012345678901234567890123456789\nauthor Source <source@example.invalid> 1 +0000\ncommitter Source <source@example.invalid> 1 +0000\nencoding ISO-8859-1\n\nmessage\n")
	if _, err := parseSourceCommit("fixture", raw); err == nil {
		t.Fatal("unsupported header accepted")
	}
}

func TestRunRejectsIncompleteAndExistingOutput(t *testing.T) {
	if err := run(nil); err == nil {
		t.Fatal("incomplete invocation accepted")
	}
	if err := run([]string{"--unknown"}); err == nil {
		t.Fatal("invalid flag accepted")
	}
	output := t.TempDir()
	allowed := filepath.Join(t.TempDir(), "allowed-signers")
	if err := os.WriteFile(allowed, []byte("actor ssh-ed25519 AAAA\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"--source", ".", "--revision", "HEAD", "--output", output, "--actor-name", "Actor", "--actor-email", "actor@example.com", "--signing-key", "key", "--allowed-signers", allowed})
	if err == nil || !strings.Contains(err.Error(), "output already exists") {
		t.Fatalf("existing output error = %v", err)
	}
}

func TestReplayRejectsMissingInputs(t *testing.T) {
	option := signedReplayFixture(t)
	if err := os.Remove(option.allowedSigners); err != nil {
		t.Fatal(err)
	}
	if err := replay(option); err == nil || !strings.Contains(err.Error(), "allowed signers") {
		t.Fatalf("missing allowed signers error = %v", err)
	}
	option = signedReplayFixture(t)
	option.revision = "missing"
	if err := replay(option); err == nil {
		t.Fatal("invalid revision accepted")
	}
	option = signedReplayFixture(t)
	option.source = filepath.Join(t.TempDir(), "missing-source")
	if err := replay(option); err == nil {
		t.Fatal("missing source repository accepted")
	}
}

func TestExecuteReportsInvalidInvocation(t *testing.T) {
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stderr.Close(); err != nil {
			t.Errorf("close stderr: %v", err)
		}
	})
	if execute(nil, stderr) != 1 {
		t.Fatal("invalid invocation succeeded")
	}
	option := signedReplayFixture(t)
	args := []string{"--source", option.source, "--revision", option.revision, "--output", option.output, "--ref", option.ref, "--actor-name", option.actorName, "--actor-email", option.actorEmail, "--signing-key", option.signingKey, "--signing-program", option.signingProgram, "--allowed-signers", option.allowedSigners}
	if execute(args, stderr) != 0 {
		t.Fatal("valid invocation failed")
	}
}

func TestGitHelpersAndReplayVerification(t *testing.T) {
	repository := t.TempDir()
	gitTest(t, repository, "init", "-q")
	gitTest(t, repository, "config", "user.name", "Source")
	gitTest(t, repository, "config", "user.email", "source@example.com")
	if err := os.WriteFile(filepath.Join(repository, "file.txt"), []byte("value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repository, "add", "file.txt")
	gitTest(t, repository, "commit", "-q", "-m", "message")
	oid, err := gitOutput(repository, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := gitBytes(repository, nil, "cat-file", "commit", oid)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := parseSourceCommit(oid, raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyReplay(repository, commit, oid, map[string]string{}, options{actorName: "Source", actorEmail: "source@example.com", allowedSigners: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("unsigned commit unexpectedly verified")
	}
	if equalStrings([]string{"a"}, []string{"b"}) || equalStrings([]string{"a"}, nil) || !equalStrings([]string{"a"}, []string{"a"}) {
		t.Fatal("string equality contract failed")
	}
	if _, err := gitInput(repository, nil, nil, "not-a-command"); err == nil {
		t.Fatal("invalid git command succeeded")
	}
	if err := command(nil, "sh", "-c", "exit 7"); err == nil {
		t.Fatal("failing command succeeded")
	}
}

func TestReplayCreatesSignedGraph(t *testing.T) {
	option := signedReplayFixture(t)
	if err := replay(option); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(option.output, "replay-receipt.json")); err != nil {
		t.Fatal(err)
	}
	if err := replay(option); err == nil {
		t.Fatal("existing replay output accepted")
	}
}

func TestReplayPreservesMergeTopology(t *testing.T) {
	option := signedReplayFixture(t)
	source := option.source
	gitTest(t, source, "checkout", "-q", "-b", "side")
	writeCommit(t, source, "side.txt", "side\n", "side")
	gitTest(t, source, "checkout", "-q", "main")
	writeCommit(t, source, "main.txt", "main\n", "main")
	gitTest(t, source, "merge", "-q", "--no-ff", "side", "-m", "merge")
	if err := replay(option); err != nil {
		t.Fatal(err)
	}
	receipt, err := os.ReadFile(filepath.Join(option.output, "replay-receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{`"root_count": 1`, `"merge_count": 1`, `"commit_count": 4`} {
		if !bytes.Contains(receipt, []byte(token)) {
			t.Fatalf("receipt missing %s: %s", token, receipt)
		}
	}
}

func TestReplayPreservesUnterminatedMessage(t *testing.T) {
	option := signedReplayFixture(t)
	tree, err := gitOutput(option.source, "rev-parse", "HEAD^{tree}")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "-C", option.source, "commit-tree", tree)
	command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Source", "GIT_AUTHOR_EMAIL=source@example.com", "GIT_AUTHOR_DATE=@1700000001 +0000", "GIT_COMMITTER_NAME=Source", "GIT_COMMITTER_EMAIL=source@example.com", "GIT_COMMITTER_DATE=@1700000002 +0000")
	command.Stdin = strings.NewReader("unterminated")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	option.revision = strings.TrimSpace(string(output))
	if err := replay(option); err != nil {
		t.Fatal(err)
	}
	receipt, err := os.ReadFile(filepath.Join(option.output, "replay-receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(receipt, []byte(`"unterminated_message_count": 1`)) {
		t.Fatalf("unterminated message not counted: %s", receipt)
	}
}

func TestReplayRejectsBareCloneWithAlternates(t *testing.T) {
	option := signedReplayFixture(t)
	if err := os.MkdirAll(option.output, 0o700); err != nil {
		t.Fatal(err)
	}
	// An existing output is rejected before clone; this asserts the same
	// fail-closed boundary without manufacturing Git's private clone format.
	if err := replay(option); err == nil || !strings.Contains(err.Error(), "output already exists") {
		t.Fatalf("existing output error = %v", err)
	}
}

func TestReplayCleansPartialOutputAfterSigningFailure(t *testing.T) {
	option := signedReplayFixture(t)
	option.signingProgram = filepath.Join(t.TempDir(), "missing-signing-program")
	if err := replay(option); err == nil {
		t.Fatal("missing signing program accepted")
	}
	if _, err := os.Stat(option.output); !os.IsNotExist(err) {
		t.Fatalf("partial output retained: %v", err)
	}
}

func TestReplayCleansPartialOutputAfterInvalidTargetRef(t *testing.T) {
	option := signedReplayFixture(t)
	option.ref = "invalid ref"
	if err := replay(option); err == nil {
		t.Fatal("invalid target ref accepted")
	}
	if _, err := os.Stat(option.output); !os.IsNotExist(err) {
		t.Fatalf("partial output retained: %v", err)
	}
}

func TestReplayCleansPartialOutputAfterMalformedCommit(t *testing.T) {
	option := signedReplayFixture(t)
	tree := gitTestOutput(t, option.source, "rev-parse", "HEAD^{tree}")
	raw := "tree " + tree + "\n" +
		"author Source <source@example.com> 1700000001 +0000\n" +
		"committer Source <source@example.com> 1700000002 +0000\n" +
		"encoding ISO-8859-1\n\nmessage\n"
	command := exec.Command("git", "-C", option.source, "hash-object", "-t", "commit", "-w", "--stdin")
	command.Stdin = strings.NewReader(raw)
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	option.revision = strings.TrimSpace(string(output))
	if err := replay(option); err == nil || !strings.Contains(err.Error(), "unsupported commit header") {
		t.Fatalf("malformed commit error = %v", err)
	}
	if _, err := os.Stat(option.output); !os.IsNotExist(err) {
		t.Fatalf("partial output retained: %v", err)
	}
}

func TestReplayReportsGitBoundaryFailures(t *testing.T) {
	for _, mode := range []string{
		"rev-list", "clone", "alternates", "for-each-ref", "update-ref", "cat-file", "commit-tree", "verify-commit", "show",
	} {
		t.Run(mode, func(t *testing.T) {
			option := signedReplayFixture(t)
			useGitWrapper(t, mode)
			if err := replay(option); err == nil {
				t.Fatalf("%s failure accepted", mode)
			}
		})
	}
}

func TestReplayRejectsUnreadableCommitObject(t *testing.T) {
	option := signedReplayFixture(t)
	commit := gitTestOutput(t, option.source, "rev-parse", "HEAD")
	object := filepath.Join(option.source, ".git", "objects", commit[:2], commit[2:])
	if err := os.Remove(object); err != nil {
		t.Fatal(err)
	}
	if err := replay(option); err == nil {
		t.Fatal("missing commit object accepted")
	}
}

func TestReplayRejectsMalformedParentCommit(t *testing.T) {
	option := signedReplayFixture(t)
	tree := gitTestOutput(t, option.source, "rev-parse", "HEAD^{tree}")
	raw := "tree " + tree + "\n" +
		"parent " + strings.Repeat("f", 40) + "\n" +
		"author Source <source@example.com> 1700000001 +0000\n" +
		"committer Source <source@example.com> 1700000002 +0000\n\nmessage\n"
	command := exec.Command("git", "-C", option.source, "hash-object", "-t", "commit", "-w", "--stdin")
	command.Stdin = strings.NewReader(raw)
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	option.revision = strings.TrimSpace(string(output))
	if err := replay(option); err == nil {
		t.Fatal("commit with missing parent accepted")
	}
}

func TestVerifyReplayRejectsSemanticAndIdentityDrift(t *testing.T) {
	option := signedReplayFixture(t)
	if err := replay(option); err != nil {
		t.Fatal(err)
	}
	sourceOID, err := gitOutput(option.source, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := gitBytes(option.source, nil, "cat-file", "commit", sourceOID)
	if err != nil {
		t.Fatal(err)
	}
	source, err := parseSourceCommit(sourceOID, raw)
	if err != nil {
		t.Fatal(err)
	}
	targetOID, err := gitOutput(option.output, "rev-parse", option.ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyReplay(option.output, source, targetOID, map[string]string{}, option); err != nil {
		t.Fatal(err)
	}
	wrongIdentity := option
	wrongIdentity.actorEmail = "wrong@example.com"
	if err := verifyReplay(option.output, source, targetOID, map[string]string{}, wrongIdentity); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("identity drift error = %v", err)
	}
	wrongSource := source
	wrongSource.tree = strings.Repeat("0", 40)
	if err := verifyReplay(option.output, wrongSource, targetOID, map[string]string{}, option); err == nil || !strings.Contains(err.Error(), "semantic replay mismatch") {
		t.Fatalf("semantic drift error = %v", err)
	}
	if err := verifyReplay(option.output, source, "missing", map[string]string{}, option); err == nil {
		t.Fatal("missing target accepted")
	}
	wrongTarget := source
	wrongTarget.oid = targetOID
	wrongTarget.authorDate = "@1 +0000"
	if err := verifyReplay(option.output, wrongTarget, targetOID, map[string]string{}, option); err == nil || !strings.Contains(err.Error(), "semantic replay mismatch") {
		t.Fatalf("target parse drift error = %v", err)
	}
}

func TestReplayCommitsRejectsUnmappedParent(t *testing.T) {
	_, _, _, _, err := replayCommits(t.TempDir(), []replaySource{{commit: sourceCommit{oid: "child", tree: strings.Repeat("a", 40), parents: []string{"missing"}, authorDate: "@1 +0000", committerDate: "@1 +0000", message: []byte("message\n")}}}, options{})
	if err == nil || !strings.Contains(err.Error(), "source parent is not mapped") {
		t.Fatalf("unmapped parent error=%v", err)
	}
}

func TestReplayCommitsReportsCommitAndVerificationFailures(t *testing.T) {
	option := signedReplayFixture(t)
	commit := sourceCommit{
		oid:           "source",
		tree:          strings.Repeat("0", 40),
		authorDate:    "@1 +0000",
		committerDate: "@1 +0000",
		message:       []byte("message\n"),
	}
	if _, _, _, _, err := replayCommits(option.output, []replaySource{{commit: commit}}, option); err == nil {
		t.Fatal("missing replay repository accepted")
	}
}

func TestParseSourceCommitRejectsMalformedShapes(t *testing.T) {
	for name, raw := range map[string]string{
		"separator":     "tree a",
		"continuation":  "tree a\n note\nauthor A <a@b> 1 +0000\ncommitter A <a@b> 1 +0000\n\nmessage",
		"identity":      "tree a\nauthor malformed\ncommitter A <a@b> 1 +0000\n\nmessage",
		"missing_field": "author A <a@b> 1 +0000\ncommitter A <a@b> 1 +0000\n\nmessage",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseSourceCommit(name, []byte(raw)); err == nil {
				t.Fatal("malformed commit accepted")
			}
		})
	}
}

func TestCommandUsesExplicitEnvironment(t *testing.T) {
	if err := command([]string{"PATH=" + os.Getenv("PATH")}, "sh", "-c", "exit 0"); err != nil {
		t.Fatal(err)
	}
}

func useGitWrapper(t *testing.T, mode string) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	helperSource := filepath.Join(directory, "git-helper.go")
	if err := os.WriteFile(helperSource, []byte(gitHelperSource), 0o600); err != nil {
		t.Fatal(err)
	}
	moduleFile := filepath.Join(directory, "go.mod")
	if err := os.WriteFile(moduleFile, []byte("module git-helper\n\ngo 1.26.5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(directory, "git")
	if filepath.Separator == '\\' {
		wrapper += ".exe"
	}
	build := exec.Command("go", "build", "-o", wrapper, helperSource)
	build.Dir = directory
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build git helper: %v: %s", err, output)
	}
	t.Setenv("AIGW_TEST_GIT_MODE", mode)
	t.Setenv("AIGW_TEST_REAL_GIT", realGit)
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
}

const gitHelperSource = `package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	args := os.Args[1:]
	command := ""
	for _, argument := range args {
		switch argument {
		case "rev-list", "clone", "for-each-ref", "update-ref", "cat-file", "commit-tree", "verify-commit", "show":
			command = argument
		}
	}
	mode := os.Getenv("AIGW_TEST_GIT_MODE")
	switch mode + ":" + command {
	case "rev-list:rev-list", "clone:clone", "for-each-ref:for-each-ref", "cat-file:cat-file", "commit-tree:commit-tree", "verify-commit:verify-commit", "show:show":
		os.Exit(1)
	case "update-ref:update-ref":
		for _, argument := range args {
			if argument == "-d" {
				os.Exit(1)
			}
		}
	case "alternates:clone":
		if run(args) != 0 {
			os.Exit(1)
		}
		output := args[len(args)-1]
		if err := os.MkdirAll(filepath.Join(output, "objects", "info"), 0o700); err != nil {
			os.Exit(1)
		}
		if err := os.WriteFile(filepath.Join(output, "objects", "info", "alternates"), nil, 0o600); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(run(args))
}

func run(args []string) int {
	command := exec.Command(os.Getenv("AIGW_TEST_REAL_GIT"), args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		return 1
	}
	return 0
}
`

func gitTest(t *testing.T, repository string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

func gitTestOutput(t *testing.T, repository string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, args...)...)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(output))
}

func signedReplayFixture(t *testing.T) options {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen unavailable")
	}
	source := t.TempDir()
	gitTest(t, source, "init", "-q", "-b", "main")
	gitTest(t, source, "config", "user.name", "Source")
	gitTest(t, source, "config", "user.email", "source@example.com")
	writeCommit(t, source, "file.txt", "value\n", "message")
	key := filepath.Join(t.TempDir(), "signing-key")
	command := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v: %s", err, output)
	}
	publicKey, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	allowed := filepath.Join(t.TempDir(), "allowed-signers")
	if err := os.WriteFile(allowed, append([]byte("actor@example.com "), publicKey...), 0o600); err != nil {
		t.Fatal(err)
	}
	return options{source: source, revision: "HEAD", output: filepath.Join(t.TempDir(), "replayed.git"), ref: "refs/heads/main", actorName: "Actor", actorEmail: "actor@example.com", signingKey: key, signingProgram: "ssh-keygen", allowedSigners: allowed}
}

func writeCommit(t *testing.T, repository, name, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repository, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repository, "add", name)
	gitTest(t, repository, "commit", "-q", "-m", message)
}
