package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A command that read the stored configuration itself would not see a model
// chosen in a client, and its save would be refused as a changed preimage.
// Every command of this tree reads through LoadConfiguration.
func TestEveryCommandReadsTheWorkingConfiguration(t *testing.T) {
	direct := regexp.MustCompile(`\bConfig\.Load\(\)`)
	loads := 0
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if direct.Match(data) {
			t.Errorf("%s reads the stored configuration with Config.Load(); a command reads runtime.LoadConfiguration()", path)
		}
		loads += strings.Count(string(data), "runtime.LoadConfiguration()")
		return nil
	})
	if err != nil || loads == 0 {
		t.Fatalf("walked the command tree: %v, %d loads of the working configuration", err, loads)
	}
}
