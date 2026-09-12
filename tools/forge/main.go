// Command forge verifies product Git objects and publishes them unchanged to
// independently selected optional Forge peers.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var branchName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

func main() { os.Exit(execute(os.Args[1:], os.Stderr)) }

func execute(arguments []string, stderr *os.File) int {
	if err := run(arguments); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func run(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: forge <commits|tag|tags|refs|project|publish-tag>")
	}
	switch arguments[0] {
	case "commits":
		return runCommitVerification(arguments[1:])
	case "tag":
		return runTagVerification(arguments[1:])
	case "tags":
		return runTagSetVerification(arguments[1:])
	case "refs":
		return runRefVerification(arguments[1:])
	case "project":
		return runProjection(arguments[1:])
	case "publish-tag":
		return runTagPublication(arguments[1:])
	default:
		return fmt.Errorf("unknown forge command: %s", arguments[0])
	}
}

type repeatedFlag []string

func (values *repeatedFlag) String() string { return strings.Join(*values, ",") }
func (values *repeatedFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func runCommitVerification(arguments []string) error {
	flags := flag.NewFlagSet("forge commits", flag.ContinueOnError)
	repository := flags.String("repository", ".", "Git repository")
	revision := flags.String("revision", "HEAD", "commit revision to verify")
	base := flags.String("base", "", "exclusive base commit of the integration range")
	email := flags.String("email", "", "required author and committer email")
	allowedSigners := flags.String("allowed-signers", "", "SSH allowed signers file")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || *email == "" || *allowedSigners == "" {
		return errors.New("usage: forge commits --email <email> --allowed-signers <path> [--repository <path>] [--revision <revision>] [--base <commit>]")
	}
	count, err := verifyCommits(*repository, *revision, *base, *email, *allowedSigners)
	if err != nil {
		return err
	}
	_, err = fmt.Printf("product commit provenance: %d verified commit(s)\n", count)
	return err
}

func runTagVerification(arguments []string) error {
	flags := flag.NewFlagSet("forge tag", flag.ContinueOnError)
	repository := flags.String("repository", ".", "Git repository")
	tag := flags.String("tag", "", "release tag")
	allowedSigners := flags.String("allowed-signers", "", "SSH allowed signers file")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || *tag == "" || *allowedSigners == "" {
		return errors.New("usage: forge tag --tag <tag> --allowed-signers <path> [--repository <path>]")
	}
	if err := verifyTag(*repository, *tag, *allowedSigners); err != nil {
		return err
	}
	_, err := fmt.Printf("product release tag signature: verified (%s)\n", *tag)
	return err
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
	for tag := range strings.FieldsSeq(tags) {
		if err := verifyTag(*repository, tag, *allowedSigners); err != nil {
			return err
		}
	}
	_, err = fmt.Println("product release tag set: verified")
	return err
}

func runRefVerification(arguments []string) error {
	var expected repeatedFlag
	flags := flag.NewFlagSet("forge refs", flag.ContinueOnError)
	repository := flags.String("repository", ".", "Git repository")
	remote := flags.String("remote", "", "target Git remote")
	flags.Var(&expected, "expect", "branch=OID expected remote object")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || *remote == "" || len(expected) == 0 {
		return errors.New("usage: forge refs --remote <name> --expect <branch=OID>... [--repository <path>]")
	}
	if _, err := gitOutput(*repository, "remote", "get-url", *remote); err != nil {
		return fmt.Errorf("publication remote is not configured: %s", *remote)
	}
	wanted, err := parseExpectedTips(expected)
	if err != nil {
		return err
	}
	for branch, oid := range wanted {
		if !branchName.MatchString(branch) || strings.Contains(branch, "..") || strings.Contains(branch, "//") {
			return fmt.Errorf("expected remote branch is malformed: %s", branch)
		}
		commit, resolveErr := gitOutput(*repository, "rev-parse", "--verify", oid+"^{commit}")
		if resolveErr != nil || commit != oid {
			return fmt.Errorf("expected object is not an exact local commit: %s", oid)
		}
		actual, observeErr := remoteReference(*repository, *remote, "refs/heads/"+branch)
		if observeErr != nil {
			return observeErr
		}
		if actual != oid {
			return fmt.Errorf("remote %s is %s, want %s", branch, actual, oid)
		}
	}
	_, err = fmt.Printf("remote branch objects verified: %s (%d ref(s))\n", *remote, len(wanted))
	return err
}

func runProjection(arguments []string) error {
	var expected repeatedFlag
	flags := flag.NewFlagSet("forge project", flag.ContinueOnError)
	repository := flags.String("repository", ".", "canonical local Git repository")
	source := flags.String("source", "main", "local publication branch")
	remote := flags.String("remote", "", "target Git remote")
	email := flags.String("email", "", "product author and committer email")
	allowedSigners := flags.String("allowed-signers", "", "product SSH trust input")
	flags.Var(&expected, "expect-remote-tip", "branch=OID divergent cutover lease")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || *remote == "" || *email == "" || *allowedSigners == "" {
		return errors.New("usage: forge project --remote <name> --email <email> --allowed-signers <path> [--source <main|proposal/*>] [--expect-remote-tip <branch=OID>]...")
	}
	tips, err := parseExpectedTips(expected)
	if err != nil {
		return err
	}
	return project(projectionOptions{
		repository:     *repository,
		source:         *source,
		remote:         *remote,
		email:          *email,
		allowedSigners: *allowedSigners,
		expectedTips:   tips,
	})
}

func runTagPublication(arguments []string) error {
	flags := flag.NewFlagSet("forge publish-tag", flag.ContinueOnError)
	repository := flags.String("repository", ".", "canonical local Git repository")
	remote := flags.String("remote", "", "target Git remote")
	tag := flags.String("tag", "", "product release tag")
	allowedSigners := flags.String("allowed-signers", "", "product SSH trust input")
	expected := flags.String("expect-remote-tag", "", "exact divergent remote tag object")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || *remote == "" || *tag == "" || *allowedSigners == "" {
		return errors.New("usage: forge publish-tag --remote <name> --tag <tag> --allowed-signers <path> [--expect-remote-tag <OID>]")
	}
	return publishTag(*repository, *remote, *tag, *allowedSigners, *expected)
}

func parseExpectedTips(values []string) (map[string]string, error) {
	result := make(map[string]string, len(values))
	for _, value := range values {
		branch, oid, ok := strings.Cut(value, "=")
		if !ok || branch == "" || oid == "" || result[branch] != "" {
			return nil, errors.New("expected remote tip must be a unique branch=OID value")
		}
		result[branch] = oid
	}
	return result, nil
}
