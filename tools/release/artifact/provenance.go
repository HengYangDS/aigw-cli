package artifact

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/pelletier/go-toml/v2"
)

const (
	provenanceStatementType = "https://in-toto.io/Statement/v1"
	provenancePredicateType = "https://slsa.dev/provenance/v1"
)

type digest struct {
	SHA256 string `json:"sha256"`
}

type subject struct {
	Name   string `json:"name"`
	Digest digest `json:"digest"`
}

type resourceDescriptor struct {
	URI    string            `json:"uri"`
	Digest map[string]string `json:"digest"`
}

type provenanceStatement struct {
	Type          string              `json:"_type"`
	Subject       []subject           `json:"subject"`
	PredicateType string              `json:"predicateType"`
	Predicate     provenancePredicate `json:"predicate"`
}

type provenancePredicate struct {
	BuildDefinition buildDefinition `json:"buildDefinition"`
	RunDetails      runDetails      `json:"runDetails"`
}

type buildDefinition struct {
	BuildType            string               `json:"buildType"`
	ExternalParameters   map[string]string    `json:"externalParameters"`
	InternalParameters   map[string]string    `json:"internalParameters"`
	ResolvedDependencies []resourceDescriptor `json:"resolvedDependencies"`
}

type runDetails struct {
	Builder struct {
		ID string `json:"id"`
	} `json:"builder"`
}

// SourceTrust selects the local product repository and its independently approved Git signers.
type SourceTrust struct {
	Repository     string
	AllowedSigners string
}

// WriteProvenance records the source, locked inputs, toolchain and unsigned artifact subjects.
func WriteProvenance(root, candidate, target, version, commit, tree string) error {
	data, err := provenanceBytes(func(name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, name))
	}, candidate, version, commit, tree)
	if err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o600)
}

// VerifyProvenance binds canonical artifact provenance to the selected, trusted local tag and commit.
func VerifyProvenance(ctx context.Context, directory, tag string, trust SourceTrust) error {
	if trust.Repository == "" || trust.AllowedSigners == "" {
		return errors.New("release source authorization requires repository and Git allowed-signers file")
	}
	version, err := semver.StrictNewVersion(strings.TrimPrefix(tag, "v"))
	if err != nil || !strings.HasPrefix(tag, "v") {
		return errors.New("release provenance requires v<semver> tag")
	}
	git := func(args ...string) ([]byte, error) {
		command := exec.CommandContext(ctx, "git", append([]string{
			"--no-replace-objects", "-C", trust.Repository, "-c", "gpg.format=ssh", "-c", "gpg.ssh.program=ssh-keygen",
			"-c", "gpg.ssh.allowedSignersFile=" + trust.AllowedSigners,
		}, args...)...)
		output, err := command.Output()
		if err != nil {
			return nil, fmt.Errorf("release provenance Git %s: %w", args[0], err)
		}
		return output, nil
	}
	tagObject, err := git("rev-parse", "--verify", "refs/tags/"+tag)
	if err != nil {
		return err
	}
	object := strings.TrimSpace(string(tagObject))
	if _, err := git("verify-tag", object); err != nil {
		return err
	}
	commitBytes, err := git("rev-parse", "--verify", object+"^{commit}")
	if err != nil {
		return err
	}
	commit := strings.TrimSpace(string(commitBytes))
	if _, err := git("verify-commit", commit); err != nil {
		return err
	}
	treeBytes, err := git("rev-parse", "--verify", commit+"^{tree}")
	if err != nil {
		return err
	}
	readSource := func(name string) ([]byte, error) { return git("show", commit+":"+name) }
	declared, err := readSource("VERSION")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(declared)) != version.String() {
		return errors.New("release provenance tag and source VERSION differ")
	}
	expected, err := provenanceBytes(readSource, directory, version.String(), commit, strings.TrimSpace(string(treeBytes)))
	if err != nil {
		return err
	}
	actual, err := os.ReadFile(filepath.Join(directory, "aigw_"+version.String()+".provenance.json"))
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return errors.New("release provenance does not match selected source, locked inputs, toolchain and artifact subjects")
	}
	return nil
}

func provenanceBytes(readSource func(string) ([]byte, error), candidate, version, commit, tree string) ([]byte, error) {
	tools, err := readToolVersions(readSource)
	if err != nil {
		return nil, err
	}
	dependencies := []resourceDescriptor{{URI: "urn:aigw:source", Digest: map[string]string{"gitCommit": commit, "gitTree": tree}}}
	for _, name := range []string{"go.mod", "go.sum", "package-lock.json", "mise.lock"} {
		data, digestErr := readSource(name)
		if digestErr != nil {
			return nil, fmt.Errorf("digest release input %s: %w", name, digestErr)
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(data))
		dependencies = append(dependencies, resourceDescriptor{URI: "file:" + name, Digest: map[string]string{"sha256": sum}})
	}
	names := provenanceSubjects(version)
	subjects := make([]subject, 0, len(names))
	for _, name := range names {
		sum, digestErr := fileDigest(filepath.Join(candidate, name))
		if digestErr != nil {
			return nil, fmt.Errorf("digest release subject %s: %w", name, digestErr)
		}
		subjects = append(subjects, subject{Name: name, Digest: digest{SHA256: sum}})
	}
	statement := provenanceStatement{
		Type: provenanceStatementType, Subject: subjects, PredicateType: provenancePredicateType,
		Predicate: provenancePredicate{BuildDefinition: buildDefinition{
			BuildType: "urn:aigw:release:v1", ExternalParameters: map[string]string{"version": version},
			InternalParameters: tools, ResolvedDependencies: dependencies,
		}},
	}
	statement.Predicate.RunDetails.Builder.ID = "urn:aigw:tools:release"
	data, err := json.MarshalIndent(statement, "", "  ")
	return append(data, '\n'), err
}

func readToolVersions(readSource func(string) ([]byte, error)) (map[string]string, error) {
	data, err := readSource("mise.toml")
	if err != nil {
		return nil, fmt.Errorf("read mise toolchain: %w", err)
	}
	var manifest struct {
		Tools map[string]string `toml:"tools"`
	}
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode mise toolchain: %w", err)
	}
	if len(manifest.Tools) == 0 {
		return nil, errors.New("mise toolchain contains no tools")
	}
	return manifest.Tools, nil
}

func fileDigest(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}
