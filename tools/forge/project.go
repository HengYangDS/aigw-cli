package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"
)

type repeatedFlag []string

func (values *repeatedFlag) String() string { return strings.Join(*values, ",") }
func (values *repeatedFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

type projectionOptions struct {
	repository     string
	source         string
	remote         string
	email          string
	allowedSigners string
	expectedTips   map[string]string
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

func project(options projectionOptions) error {
	if options.source != "main" && !strings.HasPrefix(options.source, "proposal/") {
		return errors.New("publication branch must be main or proposal/*")
	}
	if status, err := gitOutput(options.repository, "status", "--porcelain", "--untracked-files=normal"); err != nil {
		return err
	} else if status != "" {
		return errors.New("refusing publication with a dirty local checkout")
	}
	if _, err := gitOutput(options.repository, "remote", "get-url", options.remote); err != nil {
		return fmt.Errorf("publication remote is not configured: %s", options.remote)
	}
	sourceRef, sourceCommit, err := localPublicationSource(options.repository, options.source)
	if err != nil {
		return err
	}
	if err := runCommitVerification([]string{
		"--repository", options.repository,
		"--revision", sourceRef,
		"--email", options.email,
		"--allowed-signers", options.allowedSigners,
	}); err != nil {
		return err
	}
	targets := []string{options.source}
	if options.source == "main" {
		targets = []string{"main", "dev"}
	}
	arguments := []string{"push", "--atomic"}
	for _, branch := range targets {
		remoteTip, err := remoteReference(options.repository, options.remote, "refs/heads/"+branch)
		if err != nil {
			return err
		}
		switch {
		case remoteTip == "":
			arguments = append(arguments, "--force-with-lease=refs/heads/"+branch+":"+strings.Repeat("0", len(sourceCommit)))
		case remoteTip == sourceCommit:
		case isAncestor(options.repository, options.remote, branch, remoteTip, sourceCommit):
		default:
			if options.expectedTips[branch] != remoteTip {
				return fmt.Errorf("remote %s diverges; exact expected tip is required for cutover", branch)
			}
			arguments = append(arguments, "--force-with-lease=refs/heads/"+branch+":"+remoteTip)
		}
	}
	arguments = append(arguments, options.remote)
	for _, branch := range targets {
		arguments = append(arguments, sourceCommit+":refs/heads/"+branch)
	}
	if err := gitRun(options.repository, arguments...); err != nil {
		return err
	}
	for _, branch := range targets {
		remoteTip, err := remoteReference(options.repository, options.remote, "refs/heads/"+branch)
		if err != nil {
			return err
		}
		if remoteTip != sourceCommit {
			return fmt.Errorf("remote %s does not equal the local product commit", branch)
		}
	}
	fmt.Printf("product commit published unchanged: %s@%s\n", options.source, sourceCommit)
	return nil
}

func localPublicationSource(repository, branch string) (string, string, error) {
	ref, err := gitOutput(repository, "rev-parse", "--symbolic-full-name", "--verify", branch)
	if err != nil || ref != "refs/heads/"+branch {
		return "", "", fmt.Errorf("publication source is not a local branch: %s", branch)
	}
	commit, err := gitOutput(repository, "rev-parse", "--verify", ref+"^{commit}")
	return ref, commit, err
}

func remoteReference(repository, remote, ref string) (string, error) {
	output, err := gitOutput(repository, "ls-remote", remote, ref)
	if err != nil {
		return "", err
	}
	if output == "" {
		return "", nil
	}
	fields := strings.Fields(output)
	if len(fields) != 2 || fields[1] != ref {
		return "", fmt.Errorf("remote reference observation is malformed: %s", ref)
	}
	return fields[0], nil
}

func isAncestor(repository, remote, branch, ancestor, descendant string) bool {
	remoteRef := "refs/aigw/forge-observation/" + strings.ReplaceAll(branch, "/", "-")
	if err := gitRun(repository, "fetch", "--quiet", "--no-tags", remote, "+refs/heads/"+branch+":"+remoteRef); err != nil {
		return false
	}
	defer func() { _ = gitRun(repository, "update-ref", "-d", remoteRef) }()
	return gitRun(repository, "merge-base", "--is-ancestor", ancestor, descendant) == nil
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
	if err := verifyTag(*repository, *tag, *allowedSigners); err != nil {
		return err
	}
	ref := "refs/tags/" + *tag
	local, err := gitOutput(*repository, "rev-parse", "--verify", ref)
	if err != nil {
		return err
	}
	remoteObject, err := remoteReference(*repository, *remote, ref)
	if err != nil {
		return err
	}
	if remoteObject == local {
		fmt.Printf("product release tag already current: %s@%s\n", *tag, local)
		return nil
	}
	lease := strings.Repeat("0", len(local))
	if remoteObject != "" {
		if *expected != remoteObject {
			return errors.New("remote release tag diverges; exact expected object is required for cutover")
		}
		lease = remoteObject
	}
	if err := gitRun(*repository, "push", "--force-with-lease="+ref+":"+lease, *remote, local+":"+ref); err != nil {
		return err
	}
	observed, err := remoteReference(*repository, *remote, ref)
	if err != nil {
		return err
	}
	if observed != local {
		return errors.New("remote release tag does not equal the local product tag object")
	}
	fmt.Printf("product release tag published unchanged: %s@%s\n", *tag, local)
	return nil
}
