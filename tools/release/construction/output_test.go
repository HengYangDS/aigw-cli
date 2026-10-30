package construction

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDefaultReleaseOutputNamesTheVersionAndCommit(t *testing.T) {
	root := t.TempDir()
	commit := strings.Repeat("a", 12) + strings.Repeat("b", 28)
	output, err := defaultReleaseOutput(root, "1.2.3", func(call toolCall) error {
		if call.Name != "git" || call.Directory != root || !slices.Equal(call.Args, []string{"rev-parse", "--verify", "HEAD^{commit}"}) {
			return errors.New("unexpected tool call")
		}
		_, err := call.Stdout.Write([]byte(commit + "\n"))
		return err
	})
	if err != nil || output != filepath.Join(root, "build", "artifacts", "1.2.3-aaaaaaaa") {
		t.Fatalf("output = %q, error = %v", output, err)
	}
	if _, err := defaultReleaseOutput(root, "1.2.3", func(toolCall) error { return errors.New("no repository") }); err == nil {
		t.Fatal("an unresolved commit named a release output")
	}
}
