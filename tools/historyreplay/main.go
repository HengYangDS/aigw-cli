// Command historyreplay constructs a signed Forge-specific commit graph while
// preserving source trees, raw messages, timestamps, and ordered topology.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var identityTimestamp = regexp.MustCompile(`^(?:author|committer) .* <[^>]*> (-?\d+) ([+-]\d{4})$`)

type sourceCommit struct {
	oid           string
	tree          string
	parents       []string
	authorDate    string
	committerDate string
	message       []byte
}

type options struct {
	source, revision, output, ref, actorName, actorEmail, signingKey, signingProgram, allowedSigners string
}

type replaySource struct {
	commit sourceCommit
	raw    []byte
}

func main() { os.Exit(execute(os.Args[1:], os.Stderr)) }

func execute(args []string, stderr *os.File) int {
	if err := run(args); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func run(args []string) error {
	var option options
	flags := flag.NewFlagSet("historyreplay", flag.ContinueOnError)
	flags.StringVar(&option.source, "source", "", "source repository")
	flags.StringVar(&option.revision, "revision", "", "source revision")
	flags.StringVar(&option.output, "output", "", "fresh bare output repository")
	flags.StringVar(&option.ref, "ref", "refs/heads/main", "target ref")
	flags.StringVar(&option.actorName, "actor-name", "", "target actor name")
	flags.StringVar(&option.actorEmail, "actor-email", "", "target actor email")
	flags.StringVar(&option.signingKey, "signing-key", "", "SSH signing key")
	flags.StringVar(&option.signingProgram, "signing-program", "ssh-keygen", "SSH signing program")
	flags.StringVar(&option.allowedSigners, "allowed-signers", "", "allowed signers file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || option.source == "" || option.revision == "" || option.output == "" || option.actorName == "" || option.actorEmail == "" || option.signingKey == "" || option.allowedSigners == "" {
		return errors.New("required: --source --revision --output --actor-name --actor-email --signing-key --allowed-signers")
	}
	return replay(option)
}

func replay(option options) (replayErr error) {
	source := filepath.Clean(option.source)
	output := filepath.Clean(option.output)
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		return fmt.Errorf("output already exists: %s", output)
	}
	if info, err := os.Stat(option.allowedSigners); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("allowed signers file is missing: %s", option.allowedSigners)
	}
	revision, err := gitOutput(source, "rev-parse", "--verify", option.revision+"^{commit}")
	if err != nil {
		return err
	}
	oidsRaw, err := gitOutput(source, "rev-list", "--reverse", "--topo-order", revision)
	if err != nil {
		return err
	}
	oids := strings.Fields(oidsRaw)
	// rev-parse above proves revision names one commit, so rev-list must return
	// at least that commit or fail. An empty successful result has no distinct
	// product meaning and must not become a parallel defensive contract.
	if err := command(nil, "git", "clone", "--quiet", "--bare", "--no-local", source, output); err != nil {
		return err
	}
	defer func() {
		if replayErr != nil {
			_ = os.RemoveAll(output)
		}
	}()
	if _, err := os.Stat(filepath.Join(output, "objects", "info", "alternates")); err == nil {
		return errors.New("replay object database uses alternates")
	}
	refs, err := gitOutput(output, "for-each-ref", "--format=%(refname)")
	if err != nil {
		return err
	}
	for _, ref := range strings.Fields(refs) {
		if err := git(output, nil, "update-ref", "-d", ref); err != nil {
			return err
		}
	}
	sources := make([]replaySource, 0, len(oids))
	for _, oid := range oids {
		raw, err := gitBytes(source, nil, "cat-file", "commit", oid)
		if err != nil {
			return err
		}
		commit, err := parseSourceCommit(oid, raw)
		if err != nil {
			return err
		}
		sources = append(sources, replaySource{commit: commit, raw: raw})
	}
	mapping, roots, merges, unterminated, err := replayCommits(output, sources, option)
	if err != nil {
		return err
	}
	tip := mapping[revision]
	if err := git(output, nil, "update-ref", option.ref, tip, strings.Repeat("0", 40)); err != nil {
		return err
	}
	var replayMap strings.Builder
	for _, oid := range oids {
		fmt.Fprintf(&replayMap, "%s\t%s\n", oid, mapping[oid])
	}
	if err := os.WriteFile(filepath.Join(output, "replay-map.tsv"), []byte(replayMap.String()), 0o600); err != nil {
		return err
	}
	receipt := map[string]any{"schema_version": 1, "source_tip": revision, "target_tip": tip, "target_ref": option.ref, "commit_count": len(oids), "root_count": roots, "merge_count": merges, "unterminated_message_count": unterminated, "semantic_fields": []string{"tree", "message_bytes", "author_timestamp", "committer_timestamp", "ordered_parents", "merge_topology"}}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(filepath.Join(output, "replay-receipt.json"), encoded, 0o600); err != nil {
		return err
	}
	compact, _ := json.Marshal(receipt)
	_, _ = fmt.Fprintln(os.Stdout, string(compact))
	return nil
}

func replayCommits(output string, sources []replaySource, option options) (map[string]string, int, int, int, error) {
	mapping := map[string]string{}
	roots, merges, unterminated := 0, 0, 0
	for _, source := range sources {
		commit := source.commit
		if len(commit.parents) == 0 {
			roots++
		}
		if len(commit.parents) > 1 {
			merges++
		}
		if !bytes.HasSuffix(commit.message, []byte{'\n'}) {
			unterminated++
		}
		commitArgs := []string{"-c", "gpg.format=ssh", "-c", "gpg.ssh.program=" + option.signingProgram, "-c", "user.signingkey=" + option.signingKey, "commit-tree", "-S", commit.tree}
		for _, parent := range commit.parents {
			mapped, ok := mapping[parent]
			if !ok {
				return nil, 0, 0, 0, fmt.Errorf("source parent is not mapped: %s", parent)
			}
			commitArgs = append(commitArgs, "-p", mapped)
		}
		environment := append(os.Environ(),
			"GIT_AUTHOR_NAME="+option.actorName, "GIT_AUTHOR_EMAIL="+option.actorEmail, "GIT_AUTHOR_DATE="+commit.authorDate,
			"GIT_COMMITTER_NAME="+option.actorName, "GIT_COMMITTER_EMAIL="+option.actorEmail, "GIT_COMMITTER_DATE="+commit.committerDate,
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		target, err := gitInput(output, environment, commit.message, commitArgs...)
		if err != nil {
			return nil, 0, 0, 0, err
		}
		targetOID := strings.TrimSpace(string(target))
		mapping[commit.oid] = targetOID
		if err := verifyReplay(output, commit, targetOID, mapping, option); err != nil {
			return nil, 0, 0, 0, err
		}
	}
	return mapping, roots, merges, unterminated, nil
}

func parseSourceCommit(oid string, raw []byte) (sourceCommit, error) {
	headers, message, ok := bytes.Cut(raw, []byte("\n\n"))
	if !ok {
		return sourceCommit{}, fmt.Errorf("commit object has no message separator: %s", oid)
	}
	result := sourceCommit{oid: oid, message: append([]byte(nil), message...)}
	active := ""
	for _, rawLine := range bytes.Split(headers, []byte{'\n'}) {
		line := string(rawLine)
		if strings.HasPrefix(line, " ") {
			if active != "gpgsig" {
				return sourceCommit{}, fmt.Errorf("unsupported continuation for commit header %q: %s", active, oid)
			}
			continue
		}
		active, _, _ = strings.Cut(line, " ")
		switch active {
		case "tree":
			result.tree = strings.TrimPrefix(line, "tree ")
		case "parent":
			result.parents = append(result.parents, strings.TrimPrefix(line, "parent "))
		case "author", "committer":
			match := identityTimestamp.FindStringSubmatch(line)
			if match == nil {
				return sourceCommit{}, fmt.Errorf("cannot parse commit identity timestamp: %s", oid)
			}
			date := "@" + match[1] + " " + match[2]
			if active == "author" {
				result.authorDate = date
			} else {
				result.committerDate = date
			}
		case "gpgsig":
		default:
			return sourceCommit{}, fmt.Errorf("unsupported commit header '%s': %s", active, oid)
		}
	}
	if result.tree == "" || result.authorDate == "" || result.committerDate == "" {
		return sourceCommit{}, fmt.Errorf("commit object lacks required semantic fields: %s", oid)
	}
	return result, nil
}

func verifyReplay(repository string, source sourceCommit, targetOID string, mapping map[string]string, option options) error {
	raw, err := gitBytes(repository, nil, "cat-file", "commit", targetOID)
	if err != nil {
		return err
	}
	target, err := parseSourceCommit(targetOID, raw)
	if err != nil {
		return err
	}
	expectedParents := make([]string, len(source.parents))
	for index, parent := range source.parents {
		expectedParents[index] = mapping[parent]
	}
	if target.tree != source.tree || !equalStrings(target.parents, expectedParents) || target.authorDate != source.authorDate || target.committerDate != source.committerDate || !bytes.Equal(target.message, source.message) {
		return fmt.Errorf("semantic replay mismatch: %s -> %s tree=%t parents=%t author_date=%t committer_date=%t message=%t", source.oid, targetOID, target.tree == source.tree, equalStrings(target.parents, expectedParents), target.authorDate == source.authorDate, target.committerDate == source.committerDate, bytes.Equal(target.message, source.message))
	}
	identity, err := gitOutput(repository, "show", "-s", "--format=%an%x00%ae%x00%cn%x00%ce", targetOID)
	if err != nil {
		return err
	}
	if identity != strings.Join([]string{option.actorName, option.actorEmail, option.actorName, option.actorEmail}, "\x00") {
		return fmt.Errorf("target identity mismatch: %s", targetOID)
	}
	return git(repository, nil, "-c", "gpg.format=ssh", "-c", "gpg.ssh.program=ssh-keygen", "-c", "gpg.ssh.allowedSignersFile="+option.allowedSigners, "verify-commit", targetOID)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func gitOutput(repository string, args ...string) (string, error) {
	output, err := gitBytes(repository, nil, args...)
	return strings.TrimSpace(string(output)), err
}

func git(repository string, environment []string, args ...string) error {
	return command(environment, "git", append([]string{"-C", repository}, args...)...)
}

func gitBytes(repository string, environment []string, args ...string) ([]byte, error) {
	return gitInput(repository, environment, nil, args...)
}

func gitInput(repository string, environment []string, input []byte, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", repository}, args...)...)
	if environment != nil {
		command.Env = environment
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if input != nil {
		command.Stdin = bytes.NewReader(input)
	}
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func command(environment []string, name string, args ...string) error {
	command := exec.Command(name, args...)
	if environment != nil {
		command.Env = environment
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return nil
}
