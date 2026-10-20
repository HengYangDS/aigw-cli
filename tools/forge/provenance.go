package main

import (
	"errors"
	"fmt"
	"net/mail"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/pelletier/go-toml/v2"
)

func verifyCommits(repository, revision, base, email, allowedSigners string) (int, error) {
	if err := validateEmail(email); err != nil {
		return 0, err
	}
	if err := requireRegularFile(allowedSigners, "allowed signers"); err != nil {
		return 0, err
	}
	if _, err := gitOutput(repository, "rev-parse", "--is-inside-work-tree"); err != nil {
		return 0, fmt.Errorf("not a Git repository: %s", repository)
	}
	head, err := gitOutput(repository, "rev-parse", "--verify", revision+"^{commit}")
	if err != nil {
		return 0, err
	}
	metadata, err := gitOutput(repository, "ls-tree", "--name-only", head, "--", ".mailmap")
	if err != nil {
		return 0, err
	}
	if metadata != "" {
		return 0, errors.New(".mailmap is forbidden because product identities belong in commit objects")
	}
	revisionRange := head
	if base != "" {
		baseCommit, resolveErr := gitOutput(repository, "rev-parse", "--verify", base+"^{commit}")
		if resolveErr != nil {
			return 0, fmt.Errorf("base commit does not exist: %s", base)
		}
		if ancestorErr := gitRun(repository, "merge-base", "--is-ancestor", baseCommit, head); ancestorErr != nil {
			return 0, fmt.Errorf("base commit %s is not an ancestor of revision %s", baseCommit, head)
		}
		revisionRange = baseCommit + ".." + head
	}
	subjectPolicy, err := loadCommitSubjectPolicy(repository, head)
	if err != nil {
		return 0, err
	}
	list, err := gitOutput(repository, "rev-list", "--reverse", "--topo-order", revisionRange)
	if err != nil {
		return 0, err
	}
	commits := strings.Fields(list)
	for _, commit := range commits {
		identity, err := gitOutput(repository, "show", "-s", "--format=%ae%x00%ce", commit)
		if err != nil {
			return 0, err
		}
		if identity != email+"\x00"+email {
			return 0, fmt.Errorf("product commit %s must use %s for author and committer", commit, email)
		}
		if err := verifySSH(repository, allowedSigners, "verify-commit", commit); err != nil {
			return 0, fmt.Errorf("product commit %s does not have a trusted signature: %w", commit, err)
		}
		subject, err := gitOutput(repository, "show", "-s", "--format=%s", commit)
		if err != nil {
			return 0, err
		}
		if !subjectPolicy.MatchString(subject) {
			return 0, fmt.Errorf("product commit %s subject does not match commit policy: %s", commit, subject)
		}
	}
	return len(commits), nil
}

// The fixed grammar projects ETHOS contracts/paradigm/paradigm.toml [commit].
// Repository declarations may restrict scopes; they do not replace that grammar.
const sharedCommitTypes = "build|chore|ci|docs|feat|fix|perf|refactor|revert|test"
const sharedCommitScope = `[a-z0-9]+(?:-[a-z0-9]+)*`

// The paradigm requires an SSH signature on every commit, so a commit without
// .ethos/workspace.toml uses the shared grammar. A declared table may only
// restrict scopes; it cannot restate or relax signing.
func loadCommitSubjectPolicy(repository, commit string) (*regexp.Regexp, error) {
	var workspace struct {
		CommitPolicy struct {
			SubjectPattern  *string  `toml:"subject_pattern"`
			Scopes          []string `toml:"scopes"`
			SigningRequired *bool    `toml:"signing_required"`
			SigningFormat   *string  `toml:"signing_format"`
		} `toml:"commit_policy"`
	}
	present, err := gitOutput(repository, "ls-tree", "--name-only", commit, "--", ".ethos/workspace.toml")
	if err != nil {
		return nil, fmt.Errorf("read commit policy: %w", err)
	}
	if present != "" {
		data, err := gitBytes(repository, "show", commit+":.ethos/workspace.toml")
		if err != nil {
			return nil, fmt.Errorf("read commit policy: %w", err)
		}
		if err := toml.Unmarshal(data, &workspace); err != nil {
			return nil, fmt.Errorf("parse commit policy: %w", err)
		}
	}
	policy := workspace.CommitPolicy
	if policy.SubjectPattern != nil {
		return nil, errors.New("commit policy subject_pattern must be replaced by the shared grammar")
	}
	if policy.SigningRequired != nil || policy.SigningFormat != nil {
		return nil, errors.New("commit policy signing is shared; remove signing_required and signing_format")
	}
	scope := sharedCommitScope
	if policy.Scopes != nil {
		scopes := policy.Scopes
		valid := regexp.MustCompile(`\A(?:` + sharedCommitScope + `)\z`)
		if len(scopes) == 0 {
			return nil, errors.New("commit policy scopes must be non-empty and unique valid scopes")
		}
		for index, name := range scopes {
			if !valid.MatchString(name) || slices.Contains(scopes[:index], name) {
				return nil, errors.New("commit policy scopes must be non-empty and unique valid scopes")
			}
		}
		scope = strings.Join(scopes, "|")
	}
	compiled, err := regexp.Compile(`\A(?:` + sharedCommitTypes + `)(?:\((?:` + scope + `)\))?!?: .+\z`)
	if err != nil {
		return nil, fmt.Errorf("compile shared commit grammar: %w", err)
	}
	return compiled, nil
}

func verifyTag(repository, tag, allowedSigners string) error {
	_, err := semver.StrictNewVersion(strings.TrimPrefix(tag, "v"))
	if err != nil || !strings.HasPrefix(tag, "v") {
		return fmt.Errorf("release tag is malformed: %s", tag)
	}
	if err := requireRegularFile(allowedSigners, "release tag trust input"); err != nil {
		return err
	}
	ref := "refs/tags/" + tag
	if _, err := gitOutput(repository, "rev-parse", "--verify", ref); err != nil {
		return fmt.Errorf("release tag does not exist: %s", tag)
	}
	kind, err := gitOutput(repository, "cat-file", "-t", ref)
	if err != nil || kind != "tag" {
		return fmt.Errorf("release tag must be annotated: %s", tag)
	}
	if err := verifySSH(repository, allowedSigners, "verify-tag", ref); err != nil {
		return fmt.Errorf("release tag does not have a trusted signature: %s: %w", tag, err)
	}
	return nil
}

func verifySSH(repository, allowedSigners, operation, object string) error {
	return gitRun(
		repository,
		"-c", "gpg.format=ssh",
		"-c", "gpg.ssh.program=ssh-keygen",
		"-c", "gpg.ssh.allowedSignersFile="+allowedSigners,
		operation, object,
	)
}

func validateEmail(value string) error {
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || !strings.Contains(value, ".") {
		return errors.New("author email is malformed")
	}
	return nil
}

func requireRegularFile(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is missing: %s", label, path)
	}
	return nil
}
