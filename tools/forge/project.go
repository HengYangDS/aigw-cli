package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type projectionOptions struct {
	repository     string
	source         string
	remote         string
	email          string
	allowedSigners string
	expectedTips   map[string]string
}

func project(options projectionOptions) error {
	if status, err := gitOutput(options.repository, "status", "--porcelain", "--untracked-files=normal"); err != nil {
		return err
	} else if status != "" {
		return errors.New("refusing publication with a dirty local checkout")
	}
	if _, err := gitOutput(options.repository, "remote", "get-url", options.remote); err != nil {
		return fmt.Errorf("publication remote is not configured: %s", options.remote)
	}
	sourceCommit, err := localPublicationSource(options.repository, options.source)
	if err != nil {
		return err
	}
	targets := []string{options.source}
	if options.source == "main" {
		targets = []string{"main", "dev"}
	}
	acceptedTip, err := remoteReference(options.repository, options.remote, "refs/heads/dev")
	if err != nil {
		return err
	}
	base := ""
	if acceptedTip != "" {
		ancestor, err := isAncestor(options.repository, options.remote, acceptedTip, sourceCommit)
		if err != nil {
			return err
		}
		if ancestor {
			base = acceptedTip
		}
	}
	if _, err := verifyCommits(options.repository, sourceCommit, base, options.email, options.allowedSigners); err != nil {
		return err
	}
	arguments := []string{"push", "--atomic"}
	for _, branch := range targets {
		remoteTip, err := remoteReference(options.repository, options.remote, "refs/heads/"+branch)
		if err != nil {
			return err
		}
		switch remoteTip {
		case "":
			arguments = append(arguments, "--force-with-lease=refs/heads/"+branch+":"+strings.Repeat("0", len(sourceCommit)))
		case sourceCommit:
		default:
			ancestor, err := isAncestor(options.repository, options.remote, remoteTip, sourceCommit)
			if err != nil {
				return err
			}
			if !ancestor {
				if options.expectedTips[branch] != remoteTip {
					return fmt.Errorf("remote %s diverges; exact expected tip is required for cutover", branch)
				}
				arguments = append(arguments, "--force-with-lease=refs/heads/"+branch+":"+remoteTip)
			}
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
	if _, err := fmt.Printf("product commit published unchanged: %s@%s\n", options.source, sourceCommit); err != nil {
		return fmt.Errorf("product commit published and verified; write result: %w", err)
	}
	return nil
}

func localPublicationSource(repository, branch string) (string, error) {
	if branch != "main" && !strings.HasPrefix(branch, "proposal/") {
		return "", errors.New("publication branch must be main or proposal/*")
	}
	ref, err := gitOutput(repository, "rev-parse", "--symbolic-full-name", "--verify", branch)
	if err != nil || ref != "refs/heads/"+branch {
		return "", fmt.Errorf("publication source is not a local branch: %s", branch)
	}
	return gitOutput(repository, "rev-parse", "--verify", ref+"^{commit}")
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

func isAncestor(repository, remote, ancestor, descendant string) (bool, error) {
	if err := gitRun(repository, "cat-file", "-e", ancestor+"^{commit}"); err != nil {
		if err := gitRun(repository, "fetch", "--quiet", "--no-tags", "--no-write-fetch-head", "--no-auto-maintenance", "--refmap=", remote, ancestor); err != nil {
			return false, fmt.Errorf("fetch observed peer commit %s: %w", ancestor, err)
		}
	}
	err := gitRun(repository, "merge-base", "--is-ancestor", ancestor, descendant)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return err == nil, err
}

func publishTag(repository, remote, tag, allowedSigners, expected string) error {
	if err := verifyTag(repository, tag, allowedSigners); err != nil {
		return err
	}
	ref := "refs/tags/" + tag
	local, err := gitOutput(repository, "rev-parse", "--verify", ref)
	if err != nil {
		return err
	}
	remoteObject, err := remoteReference(repository, remote, ref)
	if err != nil {
		return err
	}
	if remoteObject == local {
		_, err := fmt.Printf("product release tag already current: %s@%s\n", tag, local)
		return err
	}
	lease := strings.Repeat("0", len(local))
	if remoteObject != "" {
		if expected != remoteObject {
			return errors.New("remote release tag diverges; exact expected object is required for cutover")
		}
		lease = remoteObject
	}
	if err := gitRun(repository, "push", "--force-with-lease="+ref+":"+lease, remote, local+":"+ref); err != nil {
		return err
	}
	observed, err := remoteReference(repository, remote, ref)
	if err != nil {
		return err
	}
	if observed != local {
		return errors.New("remote release tag does not equal the local product tag object")
	}
	if _, err := fmt.Printf("product release tag published unchanged: %s@%s\n", tag, local); err != nil {
		return fmt.Errorf("product release tag published and verified; write result: %w", err)
	}
	return nil
}

func gitOutput(repository string, arguments ...string) (string, error) {
	output, err := gitBytes(repository, arguments...)
	return strings.TrimSpace(string(output)), err
}

func gitBytes(repository string, arguments ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", repository}, arguments...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), err, message)
	}
	return stdout.Bytes(), nil
}

func gitRun(repository string, arguments ...string) error {
	_, err := gitBytes(repository, arguments...)
	return err
}
