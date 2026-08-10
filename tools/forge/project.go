package main

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var scpLikeRemote = regexp.MustCompile(`^[^@/:]+@[^/:]+:.+$`)

type projectionOptions struct {
	repository, branch, remote, sourceProvider, targetProvider string
	sourceEmail, actorName, actorEmail                         string
	signingKey, signingProgram, sourceSigners, targetSigners   string
}

type projectedBranch struct {
	name, ref, target string
}

func runProjection(args []string) error {
	var option projectionOptions
	flags := flag.NewFlagSet("forge project", flag.ContinueOnError)
	flags.StringVar(&option.repository, "repository", ".", "canonical Git repository")
	flags.StringVar(&option.branch, "branch", "main", "branch to project")
	flags.StringVar(&option.remote, "remote", "", "target Git remote")
	flags.StringVar(&option.sourceProvider, "source-provider", "", "source Forge identity")
	flags.StringVar(&option.targetProvider, "target-provider", "", "target Forge identity")
	flags.StringVar(&option.sourceEmail, "source-email", "", "source commit email")
	flags.StringVar(&option.actorName, "actor-name", "", "target actor name")
	flags.StringVar(&option.actorEmail, "actor-email", "", "target actor email")
	flags.StringVar(&option.signingKey, "signing-key", "", "target SSH signing key")
	flags.StringVar(&option.signingProgram, "signing-program", "ssh-keygen", "SSH signing program")
	flags.StringVar(&option.sourceSigners, "source-allowed-signers", "", "source SSH trust input")
	flags.StringVar(&option.targetSigners, "target-allowed-signers", "", "target SSH trust input")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || option.remote == "" || option.sourceProvider == "" || option.targetProvider == "" || option.sourceEmail == "" || option.actorName == "" || option.actorEmail == "" || option.signingKey == "" || option.sourceSigners == "" || option.targetSigners == "" {
		return errors.New("usage: forge project --remote <name> --source-provider <gitlab|github> --target-provider <gitlab|github> --source-email <email> --actor-name <name> --actor-email <email> --signing-key <path> --source-allowed-signers <path> --target-allowed-signers <path> [options]")
	}
	if option.sourceProvider == option.targetProvider {
		return errors.New("source and target providers must differ")
	}
	return project(option)
}

func project(option projectionOptions) error {
	if option.signingProgram == "" {
		option.signingProgram = "ssh-keygen"
	}
	if option.branch != "main" && !strings.HasPrefix(option.branch, "proposal/") {
		return errors.New("projected branch must be main or proposal/*; main projects main and dev together")
	}
	repository, err := filepath.Abs(option.repository)
	if err != nil {
		return err
	}
	if _, err := gitOutput(repository, "rev-parse", "--is-inside-work-tree"); err != nil {
		return fmt.Errorf("run inside a Git worktree: %w", err)
	}
	status, err := gitOutput(repository, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return err
	}
	if status != "" {
		return errors.New("refusing projection with a dirty canonical worktree")
	}
	remoteURL, err := gitOutput(repository, "config", "--local", "--get", "remote."+option.remote+".url")
	if err != nil {
		return fmt.Errorf("target remote is not configured: %s", option.remote)
	}
	if err := validateProjectionRemote(remoteURL); err != nil {
		return err
	}
	branches := []string{option.branch}
	if option.branch == "main" {
		branches = []string{"main", "dev"}
	}
	workspace, err := os.MkdirTemp("", "aigw-forge-projection-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(workspace) }()
	projected := make([]projectedBranch, 0, len(branches))
	for _, branch := range branches {
		result, err := prepareProjection(repository, workspace, remoteURL, branch, option)
		if err != nil {
			return err
		}
		projected = append(projected, result)
	}
	arguments := []string{"push", "--quiet"}
	if len(projected) > 1 {
		arguments = append(arguments, "--atomic")
	}
	arguments = append(arguments, "target")
	for _, branch := range projected {
		arguments = append(arguments, branch.target+":"+branch.ref)
	}
	if err := git(filepath.Join(workspace, "transaction.git"), nil, arguments...); err != nil {
		return err
	}
	for _, branch := range projected {
		fmt.Printf("target provider projection synchronized: %s@%s\n", branch.name, branch.target)
	}
	return nil
}

func prepareProjection(repository, workspace, remoteURL, branch string, option projectionOptions) (projectedBranch, error) {
	branchRef, err := localBranch(repository, "source", branch)
	if err != nil {
		return projectedBranch{}, err
	}
	canonical, err := gitOutput(repository, "rev-parse", branchRef)
	if err != nil {
		return projectedBranch{}, err
	}
	if err := runCommitProvenance([]string{"--repository", repository, "--revision", branchRef, "--provider", option.sourceProvider, "--email", option.sourceEmail, "--allowed-signers", option.sourceSigners}); err != nil {
		return projectedBranch{}, err
	}
	projection := filepath.Join(workspace, "transaction.git")
	if _, err := os.Stat(projection); os.IsNotExist(err) {
		replayOption := options{
			source: repository, revision: canonical, output: projection, ref: branchRef,
			actorName: option.actorName, actorEmail: option.actorEmail,
			signingKey: option.signingKey, signingProgram: option.signingProgram,
			allowedSigners: option.targetSigners,
		}
		if err := replay(replayOption); err != nil {
			return projectedBranch{}, err
		}
		if err := git(projection, nil, "remote", "add", "target", remoteURL); err != nil {
			return projectedBranch{}, err
		}
	} else if err != nil {
		return projectedBranch{}, err
	} else {
		temporary := filepath.Join(workspace, strings.ReplaceAll(branch, "/", "-")+".git")
		replayOption := options{
			source: repository, revision: canonical, output: temporary, ref: branchRef,
			actorName: option.actorName, actorEmail: option.actorEmail,
			signingKey: option.signingKey, signingProgram: option.signingProgram,
			allowedSigners: option.targetSigners,
		}
		if err := replay(replayOption); err != nil {
			return projectedBranch{}, err
		}
		if err := git(projection, nil, "fetch", "--quiet", temporary, "+"+branchRef+":"+branchRef); err != nil {
			return projectedBranch{}, err
		}
	}
	target, err := gitOutput(projection, "rev-parse", branchRef)
	if err != nil {
		return projectedBranch{}, err
	}
	remoteTip, err := remoteHead(projection, branch)
	if err != nil {
		return projectedBranch{}, err
	}
	if remoteTip != "" {
		remoteRef := "refs/remotes/target/" + branch
		if err := git(projection, nil, "fetch", "--quiet", "--no-tags", "target", branchRef+":"+remoteRef); err != nil {
			return projectedBranch{}, err
		}
		if err := git(projection, nil, "merge-base", "--is-ancestor", remoteTip, target); err != nil {
			return projectedBranch{}, errors.New("target branch diverges from the complete canonical identity projection")
		}
		if err := runCommitProvenance([]string{"--repository", projection, "--revision", branchRef, "--provider", option.targetProvider, "--email", option.actorEmail, "--allowed-signers", option.targetSigners}); err != nil {
			return projectedBranch{}, err
		}
	}
	if err := verifyProjectionTags(repository, projection, canonical, option); err != nil {
		return projectedBranch{}, err
	}
	return projectedBranch{name: branch, ref: branchRef, target: target}, nil
}

func validateProjectionRemote(raw string) error {
	if strings.HasPrefix(raw, "file://") || filepath.IsAbs(raw) {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err == nil && (parsed.Scheme == "https" || parsed.Scheme == "ssh") && parsed.Hostname() != "" && parsed.RawQuery == "" && parsed.Fragment == "" {
		return nil
	}
	if scpLikeRemote.MatchString(raw) {
		return nil
	}
	return errors.New("target remote must use an explicit local, HTTPS, SSH, or SCP-like Git URL")
}

func remoteHead(repository, branch string) (string, error) {
	output, err := gitOutput(repository, "ls-remote", "--heads", "target", "refs/heads/"+branch)
	if err != nil || output == "" {
		return "", err
	}
	return strings.Fields(output)[0], nil
}

func verifyProjectionTags(canonicalRepository, projection, canonical string, option projectionOptions) error {
	output, err := gitOutput(projection, "ls-remote", "--tags", "target", "v[0-9]*")
	if err != nil {
		return err
	}
	canonicalTrees, err := orderedTrees(canonicalRepository, canonical)
	if err != nil {
		return err
	}
	treeSet := make(map[string]struct{}, len(canonicalTrees))
	for _, tree := range canonicalTrees {
		treeSet[tree] = struct{}{}
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.HasSuffix(fields[1], "^{}") {
			continue
		}
		tag := strings.TrimPrefix(fields[1], "refs/tags/")
		qualified := "github/" + tag
		if err := git(projection, nil, "fetch", "--quiet", "--no-tags", "target", "refs/tags/"+tag+":refs/tags/"+qualified); err != nil {
			return err
		}
		kind, err := gitOutput(projection, "cat-file", "-t", qualified)
		if err != nil || kind != "tag" {
			return fmt.Errorf("target release tag must be annotated: %s", tag)
		}
		tree, err := gitOutput(projection, "rev-parse", qualified+"^{}^{tree}")
		if err != nil {
			return err
		}
		if _, relevant := treeSet[tree]; !relevant {
			continue
		}
		if err := verifySSH(projection, option.targetSigners, "verify-tag", qualified); err != nil {
			return fmt.Errorf("target provenance tag does not verify: %s", tag)
		}
	}
	return nil
}
