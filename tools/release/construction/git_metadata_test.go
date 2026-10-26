package construction

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
)

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
