package main

import (
	"errors"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var semanticTag = regexp.MustCompile(`^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

func runCommitVerification(arguments []string) error {
	flags := flag.NewFlagSet("forge commits", flag.ContinueOnError)
	repository := flags.String("repository", ".", "Git repository")
	revision := flags.String("revision", "HEAD", "commit revision to verify")
	email := flags.String("email", "", "required author and committer email")
	allowedSigners := flags.String("allowed-signers", "", "SSH allowed signers file")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || *email == "" || *allowedSigners == "" {
		return errors.New("usage: forge commits --email <email> --allowed-signers <path> [--repository <path>] [--revision <revision>]")
	}
	if err := validateEmail(*email); err != nil {
		return err
	}
	if err := requireRegularFile(*allowedSigners, "allowed signers"); err != nil {
		return err
	}
	if _, err := gitOutput(*repository, "rev-parse", "--is-inside-work-tree"); err != nil {
		return fmt.Errorf("not a Git repository: %s", *repository)
	}
	if _, err := os.Stat(filepath.Join(*repository, ".mailmap")); err == nil {
		return errors.New(".mailmap is forbidden because product identities belong in commit objects")
	} else if !os.IsNotExist(err) {
		return err
	}
	head, err := gitOutput(*repository, "rev-parse", "--verify", *revision+"^{commit}")
	if err != nil {
		return err
	}
	list, err := gitOutput(*repository, "rev-list", "--reverse", "--topo-order", head)
	if err != nil {
		return err
	}
	commits := strings.Fields(list)
	for _, commit := range commits {
		identity, err := gitOutput(*repository, "show", "-s", "--format=%ae%x00%ce", commit)
		if err != nil {
			return err
		}
		if identity != *email+"\x00"+*email {
			return fmt.Errorf("product commit %s must use %s for author and committer", commit, *email)
		}
		if err := verifySSH(*repository, *allowedSigners, "verify-commit", commit); err != nil {
			return fmt.Errorf("product commit %s does not have a trusted signature: %w", commit, err)
		}
	}
	fmt.Printf("product commit provenance: %d verified commit(s)\n", len(commits))
	return nil
}

func runTagVerification(arguments []string) error {
	flags := flag.NewFlagSet("forge tag", flag.ContinueOnError)
	repository := flags.String("repository", ".", "Git repository")
	tag := flags.String("tag", "", "release tag")
	allowedSigners := flags.String("allowed-signers", "", "SSH allowed signers file")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || *tag == "" || *allowedSigners == "" {
		return errors.New("usage: forge tag --tag <tag> --allowed-signers <path> [--repository <path>]")
	}
	return verifyTag(*repository, *tag, *allowedSigners)
}

func runTagSetVerification(arguments []string) error {
	flags := flag.NewFlagSet("forge tags", flag.ContinueOnError)
	repository := flags.String("repository", ".", "Git repository")
	allowedSigners := flags.String("allowed-signers", "", "SSH allowed signers file")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || *allowedSigners == "" {
		return errors.New("usage: forge tags --allowed-signers <path> [--repository <path>]")
	}
	tags, err := gitOutput(*repository, "for-each-ref", "--format=%(refname:short)", "refs/tags")
	if err != nil {
		return err
	}
	for _, tag := range strings.Fields(tags) {
		if !semanticTag.MatchString(tag) {
			return fmt.Errorf("unexpected release tag: %s", tag)
		}
		if err := verifyTag(*repository, tag, *allowedSigners); err != nil {
			return err
		}
	}
	fmt.Println("product release tag set: verified")
	return nil
}

func verifyTag(repository, tag, allowedSigners string) error {
	if !semanticTag.MatchString(tag) {
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
	fmt.Printf("product release tag signature: verified (%s)\n", tag)
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
