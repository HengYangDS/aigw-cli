package construction

import (
	"aigw-cli/internal/program/artifact"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Release metadata selects a peer only when the checkout leaves exactly one
// unambiguous native choice, and reports every Git failure it depends on.
func TestReleaseGitEnvironmentSelectsOnlyAnUnambiguousPeer(t *testing.T) {
	failure := errors.New("git exited with status 128")
	for _, test := range []struct {
		name, branch, remote, remotes, count string
		failing                              string
		want                                 []string
		problem                              string
	}{
		{name: "unreadable branch", failing: "rev-parse --abbrev-ref HEAD", problem: "read release source branch"},
		{name: "detached checkout", branch: "HEAD", remotes: "origin\n"},
		{name: "unreadable branch peer", branch: "work/x", failing: "config get --default= branch.work/x.remote", problem: "read release source peer"},
		{name: "explicit branch peer", branch: "work/x", remote: "gitlab"},
		{name: "unreadable peers", branch: "work/x", failing: "remote", problem: "read release source peers"},
		{name: "origin present", branch: "work/x", remotes: "origin\ngithub\n"},
		{name: "single peer", branch: "work/x", remotes: "github\n"},
		{name: "GitLab beside another peer", branch: "work/x", remotes: "gitlab\nmirror\n",
			want: []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=branch.work/x.remote", "GIT_CONFIG_VALUE_0=gitlab"}},
		{name: "invalid inherited count", branch: "work/x", remotes: "github\ngitlab\n", count: "-1", problem: "invalid inherited Git configuration count"},
		{name: "no native peer", branch: "work/x", remotes: "mirror\nbackup\n", problem: "explicitly selected native branch peer"},
		{name: "no peer beside inherited configuration", branch: "work/x", count: "2",
			want: []string{"GIT_CONFIG_COUNT=3", "GIT_CONFIG_KEY_2=remote.origin.url", "GIT_CONFIG_VALUE_2=."}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_COUNT", test.count)
			environment, err := goReleaserGitEnvironment(t.TempDir(), nil, func(call toolCall) error {
				arguments := strings.Join(call.Args, " ")
				if call.Name != "git" {
					t.Fatalf("unselected tool %q", call.Name)
				}
				if arguments == test.failing {
					return failure
				}
				switch {
				case arguments == "rev-parse --abbrev-ref HEAD":
					_, _ = io.WriteString(call.Stdout, test.branch+"\n")
				case strings.HasPrefix(arguments, "config get --default= "):
					_, _ = io.WriteString(call.Stdout, test.remote+"\n")
				case arguments == "remote":
					_, _ = io.WriteString(call.Stdout, test.remotes)
				default:
					t.Fatalf("unexpected Git call %q", arguments)
				}
				return nil
			})
			if test.problem != "" {
				if err == nil || !strings.Contains(err.Error(), test.problem) || environment != nil {
					t.Fatalf("peer selection = %q, %v; want %q", environment, err, test.problem)
				}
				if test.failing != "" && !errors.Is(err, failure) {
					t.Fatalf("peer selection lost the Git failure: %v", err)
				}
				return
			}
			if err != nil || !slices.Equal(environment, test.want) {
				t.Fatalf("peer selection = %q, %v; want %q", environment, err, test.want)
			}
		})
	}
}

func TestNativeArchiveMetadataSelectsAnExistingPeerWithoutReplacingGitInputs(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "2")
	request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
	request.ReleasePublicKey, _ = releasePublicKey(signingKey(t))
	stop := errors.New("metadata observed before archive build")
	_, err := buildArchives(request, t.TempDir(), func(call toolCall) error {
		if call.Name == "git" {
			switch strings.Join(call.Args, " ") {
			case "rev-parse --abbrev-ref HEAD":
				_, _ = io.WriteString(call.Stdout, "work/product\n")
			case "remote":
				_, _ = io.WriteString(call.Stdout, "github\ngitlab\n")
			}
			return nil
		}
		if call.Name != "goreleaser" {
			t.Fatalf("unselected tool %q", call.Name)
		}
		for _, want := range []string{"GIT_CONFIG_COUNT=3", "GIT_CONFIG_KEY_2=branch.work/product.remote", "GIT_CONFIG_VALUE_2=github"} {
			if !slices.Contains(call.Env, want) {
				t.Errorf("native metadata omits %q", want)
			}
		}
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("metadata-only refusal = %v", err)
	}
}

// remoteFreeCheckout returns a committed checkout that has no Git remote, with
// the repository's own GoReleaser configuration and a minimal program.
func remoteFreeCheckout(t *testing.T) (root string, git func(args ...string) string) {
	t.Helper()
	root = releaseRoot(t)
	config, err := os.ReadFile(filepath.Join("..", "..", "..", ".config", "release", "goreleaser", ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{
		".config/release/goreleaser/.goreleaser.yaml": config,
		"go.mod":           []byte("module aigw-cli\n\ngo 1.25\n"),
		"cmd/aigw/main.go": []byte("package main\n\nfunc main() {}\n"),
		"README.md":        []byte("# Remote-free fixture\n"), "LICENSE": []byte("fixture\n"),
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git = func(args ...string) string {
		t.Helper()
		command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root, "-c", "commit.gpgsign=false", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test"}, args...)...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("remote-free fixture git %v: %v\n%s", args, err, output)
		}
		return string(output)
	}
	git("init", "--quiet", "-b", "work/product")
	git("add", ".")
	git("commit", "--quiet", "-m", "test fixture")
	if remotes := git("remote"); remotes != "" {
		t.Fatalf("fixture checkout has a remote: %q", remotes)
	}
	return root, git
}

// Where no Forge is configured, GoReleaser's question for the remote's URL is
// answered with the checkout itself, through the process environment alone.
func TestReleaseGitEnvironmentNamesTheCheckoutWhereNoForgeIsConfigured(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "")
	root, git := remoteFreeCheckout(t)
	run := executeTool(t.Context())
	remoteURL := func(environment []string) (string, error) {
		var answer, diagnostic bytes.Buffer
		err := run(toolCall{Name: "git", Directory: root, Args: []string{"ls-remote", "--get-url"}, Env: environment, Stdout: &answer, Stderr: &diagnostic})
		return strings.TrimSpace(answer.String()), err
	}
	configuration, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=remote.origin.url", "GIT_CONFIG_VALUE_0=."}
	for _, state := range []string{"on a branch", "detached"} {
		if state == "detached" {
			git("checkout", "--quiet", "--detach")
		}
		if answer, err := remoteURL(nil); err == nil {
			t.Fatalf("%s: Git names a remote where none is configured: %q", state, answer)
		}
		environment, err := goReleaserGitEnvironment(root, nil, run)
		if err != nil || !slices.Equal(environment, want) {
			t.Fatalf("%s: environment = %q, %v; want %q", state, environment, err, want)
		}
		if answer, err := remoteURL(environment); err != nil || answer != "." {
			t.Fatalf("%s: remote URL = %q, %v", state, answer, err)
		}
	}
	if written, err := os.ReadFile(filepath.Join(root, ".git", "config")); err != nil || !bytes.Equal(written, configuration) {
		t.Fatalf("construction wrote Git configuration: %v\n%s", err, written)
	}
	git("remote", "add", "mirror", filepath.Join(root, "mirror"))
	if environment, err := goReleaserGitEnvironment(root, nil, run); err != nil || environment != nil {
		t.Fatalf("a configured remote was replaced: %q, %v", environment, err)
	}
}

// TestPortableArtifactsBuildWhereNoForgeIsConfigured verifies the scenario "No
// Forge is configured" for release construction: the real build of a checkout
// without a Git remote is complete, and no diagnostic of GoReleaser prevents
// its qualification.
func TestPortableArtifactsBuildWhereNoForgeIsConfigured(t *testing.T) {
	root, _ := remoteFreeCheckout(t)
	request := buildRequest{Root: root, Version: "1.2.3", Epoch: "1784246400", TargetOS: runtime.GOOS}
	stage, err := buildArchives(request, t.TempDir(), executeTool(t.Context()))
	if err != nil {
		t.Fatalf("build without a Git remote: %v", err)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		target := artifact.Target{OS: runtime.GOOS, Arch: arch}
		if _, err := target.ReadProgram(filepath.Join(stage, target.ArchiveName(request.Version)), filepath.Join(stage, "checksums.txt"), request.Version); err != nil {
			t.Fatal(err)
		}
	}
}
