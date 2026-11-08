package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
)

// A client file that the user cannot read or write is named, with the step
// that makes it so, instead of AIGW's folder, which holds nothing to change.
// Once the file is readable again, the command that failed passes.
func TestAnUnreadableClientFileNamesThatFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not represent a file that its owner cannot read in a file's mode")
	}
	if os.Geteuid() == 0 {
		t.Skip("the superuser reads a file of any mode")
	}
	for id, build := range projectedClients {
		t.Run(id, func(t *testing.T) {
			client := build(t)
			if err := os.Chmod(client.file, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(client.file, 0o600) })
			client.output.Reset()
			if err := cli.Execute(client.app, []string{"sync", "--json"}); err == nil {
				t.Fatal("sync passed with an unreadable client file")
			}
			var failure struct {
				Error      string `json:"error"`
				NextDetail string `json:"next_detail"`
			}
			if err := json.Unmarshal(client.output.Bytes(), &failure); err != nil {
				t.Fatalf("%v\n%s", err, client.output)
			}
			name := filepath.Base(client.file)
			if !strings.Contains(failure.Error, name) || !strings.Contains(failure.NextDetail, name) ||
				strings.Contains(failure.NextDetail, "AIGW's folder") || !strings.HasSuffix(failure.NextDetail, "then run the same command again.") {
				t.Fatalf("the failure = %+v", failure)
			}
			if err := os.Chmod(client.file, 0o600); err != nil {
				t.Fatal(err)
			}
			syncAndCheckPass(t, client.app, client.output, id)
		})
	}
}
