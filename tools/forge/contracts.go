package main

import (
	"errors"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var semanticTag = regexp.MustCompile(`^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

type repeatedFlag []string

func (values *repeatedFlag) String() string { return strings.Join(*values, ",") }
func (values *repeatedFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

type peerSpec struct{ name, ref, mode string }

func runCommitProvenance(args []string) error {
	flags := flag.NewFlagSet("forge commits", flag.ContinueOnError)
	repository := flags.String("repository", ".", "Git repository")
	revision := flags.String("revision", "HEAD", "commit revision to verify")
	provider := flags.String("provider", "", "gitlab or github")
	email := flags.String("email", "", "required author and committer email")
	allowedSigners := flags.String("allowed-signers", "", "SSH allowed signers file")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *provider == "" || *email == "" || *allowedSigners == "" {
		return errors.New("usage: forge commits --provider <gitlab|github> --email <email> --allowed-signers <path> [--repository <path>]")
	}
	if *provider != "gitlab" && *provider != "github" {
		return errors.New("provider must be gitlab or github")
	}
	address, err := mail.ParseAddress(*email)
	if err != nil || address.Address != *email || !strings.Contains(*email, ".") {
		return errors.New("author email is malformed")
	}
	if err := requireRegularFile(*allowedSigners, "allowed signers"); err != nil {
		return err
	}
	if _, err := gitOutput(*repository, "rev-parse", "--is-inside-work-tree"); err != nil {
		return fmt.Errorf("not a Git repository: %s", *repository)
	}
	if _, err := os.Stat(filepath.Join(*repository, ".mailmap")); err == nil {
		return errors.New(".mailmap is forbidden because provider identities must be stored in commit objects")
	} else if !os.IsNotExist(err) {
		return err
	}
	head, err := gitOutput(*repository, "rev-parse", "--verify", *revision+"^{commit}")
	if err != nil {
		return err
	}
	commits, err := gitOutput(*repository, "rev-list", "--reverse", "--topo-order", head)
	if err != nil {
		return err
	}
	oids := strings.Fields(commits)
	if len(oids) == 0 {
		return fmt.Errorf("%s commit provenance: HEAD has no reachable commits", *provider)
	}
	for _, oid := range oids {
		identity, err := gitOutput(*repository, "show", "-s", "--format=%ae%x00%ce", oid)
		if err != nil {
			return err
		}
		if identity != *email+"\x00"+*email {
			return fmt.Errorf("%s commit %s must use %s for author and committer", *provider, oid, *email)
		}
		if err := verifySSH(*repository, *allowedSigners, "verify-commit", oid); err != nil {
			return fmt.Errorf("%s commit %s does not have a trusted signature: %w", *provider, oid, err)
		}
	}
	fmt.Printf("%s commit provenance: %d verified commit(s)\n", *provider, len(oids))
	return nil
}

func runTagSignature(args []string) error {
	flags := flag.NewFlagSet("forge tag", flag.ContinueOnError)
	repository := flags.String("repository", ".", "Git repository")
	provider := flags.String("provider", "", "gitlab or github")
	tag := flags.String("tag", "", "release tag")
	allowedSigners := flags.String("allowed-signers", "", "SSH allowed signers file")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *tag == "" {
		return errors.New("usage: forge tag --provider <gitlab|github> --tag <tag> --allowed-signers <path> [--repository <path>]")
	}
	if *provider != "gitlab" && *provider != "github" {
		return errors.New("release tag provider must be gitlab or github")
	}
	if err := validateTagForProvider(*tag, *provider); err != nil {
		return err
	}
	if err := requireRegularFile(*allowedSigners, "release tag trust input"); err != nil {
		return err
	}
	if _, err := gitOutput(*repository, "rev-parse", "--verify", "refs/tags/"+*tag); err != nil {
		return fmt.Errorf("release tag does not exist: %s", *tag)
	}
	kind, err := gitOutput(*repository, "cat-file", "-t", *tag)
	if err != nil || kind != "tag" {
		return fmt.Errorf("release tag must be annotated: %s", *tag)
	}
	object, err := gitOutput(*repository, "cat-file", "-p", *tag)
	if err != nil {
		return err
	}
	if !strings.Contains(object, "-----BEGIN SSH SIGNATURE-----") || !strings.Contains(object, "-----END SSH SIGNATURE-----") {
		return fmt.Errorf("release tag is not SSH signed: %s", *tag)
	}
	if err := verifySSH(*repository, *allowedSigners, "verify-tag", *tag); err != nil {
		return err
	}
	fmt.Printf("release tag SSH signature: OK (%s %s)\n", *provider, *tag)
	return nil
}

func validateTagForProvider(tag, provider string) error {
	qualified := strings.HasPrefix(tag, "github/")
	plain := strings.TrimPrefix(tag, "github/")
	if !semanticTag.MatchString(plain) {
		return fmt.Errorf("release tag is malformed: %s", tag)
	}
	if qualified && provider != "github" {
		return fmt.Errorf("qualified GitHub tag requires github provider: %s", tag)
	}
	return nil
}

func runTagNamespace(args []string) error {
	flags := flag.NewFlagSet("forge tags", flag.ContinueOnError)
	repository := flags.String("repository", ".", "Git repository")
	mode := flags.String("mode", "", "local, gitlab, or github")
	gitlabSigners := flags.String("gitlab-allowed-signers", "", "GitLab SSH allowed signers")
	githubSigners := flags.String("github-allowed-signers", "", "GitHub SSH allowed signers")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("usage: forge tags --mode <local|gitlab|github> [trust inputs] [--repository <path>]")
	}
	if *mode != "local" && *mode != "gitlab" && *mode != "github" {
		return errors.New("tag namespace mode must be local, gitlab, or github")
	}
	if (*mode == "local" || *mode == "gitlab") && *gitlabSigners == "" {
		return errors.New("GitLab trust input is required")
	}
	if (*mode == "local" || *mode == "github") && *githubSigners == "" {
		return errors.New("GitHub trust input is required")
	}
	tags, err := gitOutput(*repository, "for-each-ref", "--format=%(refname:short)", "refs/tags")
	if err != nil {
		return err
	}
	for _, tag := range strings.Fields(tags) {
		provider, signers := "gitlab", *gitlabSigners
		switch {
		case strings.HasPrefix(tag, "github/"):
			if *mode != "local" {
				return fmt.Errorf("qualified GitHub provenance is only valid in a local canonical checkout: %s", tag)
			}
			provider, signers = "github", *githubSigners
		case semanticTag.MatchString(tag):
			if *mode == "github" {
				provider, signers = "github", *githubSigners
			}
		default:
			return fmt.Errorf("unexpected release tag namespace: %s", tag)
		}
		if err := verifySSH(*repository, signers, "verify-tag", tag); err != nil {
			return fmt.Errorf("%s tag does not verify: %s", provider, tag)
		}
	}
	fmt.Printf("release tag namespace: OK (%s)\n", *mode)
	return nil
}

func runSync(args []string, closeout bool) error {
	name := "forge sync"
	if closeout {
		name = "forge closeout"
	}
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	repository := flags.String("repository", ".", "Git repository")
	canonical := flags.String("canonical", "main", "canonical local branch")
	source := flags.String("source", "", "source local branch")
	var peers repeatedFlag
	flags.Var(&peers, "peer", "name:ref:commit|tree")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || len(peers) == 0 || closeout && *source == "" {
		return fmt.Errorf("usage: %s [--repository <path>] [--canonical <branch>] %s--peer <name:ref:mode>...", name, map[bool]string{true: "--source <branch> ", false: ""}[closeout])
	}
	canonicalRef, err := localBranch(*repository, "canonical", *canonical)
	if err != nil {
		return err
	}
	canonicalCommit, err := gitOutput(*repository, "rev-parse", canonicalRef+"^{commit}")
	if err != nil {
		return err
	}
	var sourceRef string
	if closeout {
		sourceRef, err = localBranch(*repository, "source", *source)
		if err != nil {
			return err
		}
		if sourceRef == canonicalRef {
			return fmt.Errorf("source branch must differ from canonical branch: %s", sourceRef)
		}
		worktree, err := gitOutput(*repository, "for-each-ref", "--format=%(worktreepath)", sourceRef)
		if err != nil {
			return err
		}
		if worktree != "" {
			status, statusErr := gitOutput(worktree, "status", "--porcelain", "--untracked-files=normal")
			if statusErr != nil {
				return fmt.Errorf("source branch worktree cannot be inspected: %s (%s)", sourceRef, worktree)
			}
			if status != "" {
				return fmt.Errorf("source branch worktree is not clean: %s (%s)", sourceRef, worktree)
			}
		}
		if err := git(*repository, nil, "merge-base", "--is-ancestor", sourceRef, canonicalRef); err != nil {
			return fmt.Errorf("canonical ref does not contain source tip: %s <- %s", canonicalRef, sourceRef)
		}
	}
	canonicalTrees, err := orderedTrees(*repository, canonicalRef)
	if err != nil {
		return err
	}
	for _, raw := range peers {
		peer, err := parsePeer(raw)
		if err != nil {
			return err
		}
		peerCommit, err := gitOutput(*repository, "rev-parse", "--verify", peer.ref+"^{commit}")
		if err != nil {
			return fmt.Errorf("peer %s is unavailable: %s", peer.name, peer.ref)
		}
		if peer.mode == "tree" {
			trees, treeErr := orderedTrees(*repository, peer.ref)
			if treeErr != nil {
				return treeErr
			}
			if !slices.Equal(canonicalTrees, trees) {
				return fmt.Errorf("peer %s does not preserve canonical ordered source-tree history", peer.name)
			}
		} else if closeout {
			if err := git(*repository, nil, "merge-base", "--is-ancestor", sourceRef, peer.ref); err != nil {
				return fmt.Errorf("peer %s does not contain source tip: %s <- %s", peer.name, peer.ref, sourceRef)
			}
		} else if peerCommit != canonicalCommit {
			return fmt.Errorf("peer %s does not exactly match canonical %s: %s@%s, expected %s", peer.name, canonicalRef, peer.ref, peerCommit, canonicalCommit)
		}
		fmt.Printf("%s peer: %s (%s) OK\n", strings.TrimPrefix(name, "forge "), peer.name, peer.mode)
	}
	return nil
}

func localBranch(repository, label, raw string) (string, error) {
	ref, err := gitOutput(repository, "rev-parse", "--symbolic-full-name", "--verify", raw)
	if err != nil || !strings.HasPrefix(ref, "refs/heads/") {
		return "", fmt.Errorf("%s ref is unavailable or is not a local branch: %s", label, raw)
	}
	return ref, nil
}

func parsePeer(raw string) (peerSpec, error) {
	parts := strings.Split(raw, ":")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] != "commit" && parts[2] != "tree" {
		return peerSpec{}, fmt.Errorf("peer specification must be name:ref:commit|tree: %s", raw)
	}
	return peerSpec{name: parts[0], ref: parts[1], mode: parts[2]}, nil
}

func orderedTrees(repository, revision string) ([]string, error) {
	output, err := gitOutput(repository, "log", "--reverse", "--topo-order", "--format=%T", revision, "--")
	return strings.Fields(output), err
}

func verifySSH(repository, allowedSigners, operation, object string) error {
	return git(repository, nil, "-c", "gpg.format=ssh", "-c", "gpg.ssh.program=ssh-keygen", "-c", "gpg.ssh.allowedSignersFile="+allowedSigners, operation, object)
}

func requireRegularFile(path, label string) error {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("%s is missing: %s", label, path)
	}
	return nil
}
