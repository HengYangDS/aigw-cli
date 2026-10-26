package construction

import (
	"errors"
	"io"
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
		{name: "detached checkout", branch: "HEAD"},
		{name: "unreadable branch peer", branch: "work/x", failing: "config get --default= branch.work/x.remote", problem: "read release source peer"},
		{name: "explicit branch peer", branch: "work/x", remote: "gitlab"},
		{name: "unreadable peers", branch: "work/x", failing: "remote", problem: "read release source peers"},
		{name: "origin present", branch: "work/x", remotes: "origin\ngithub\n"},
		{name: "single peer", branch: "work/x", remotes: "github\n"},
		{name: "GitLab beside another peer", branch: "work/x", remotes: "gitlab\nmirror\n",
			want: []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=branch.work/x.remote", "GIT_CONFIG_VALUE_0=gitlab"}},
		{name: "invalid inherited count", branch: "work/x", remotes: "github\ngitlab\n", count: "-1", problem: "invalid inherited Git configuration count"},
		{name: "no native peer", branch: "work/x", remotes: "mirror\nbackup\n", problem: "explicitly selected native branch peer"},
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
