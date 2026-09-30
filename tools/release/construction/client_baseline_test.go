package construction

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestNativeClientAcceptancePreservesExplicitPublishedPredecessor(t *testing.T) {
	baseline := filepath.Join(t.TempDir(), "published-aigw")
	if err := os.WriteFile(baseline, []byte("published predecessor"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIGW_ACCEPTANCE_BASELINE", baseline)
	request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
	artifacts := t.TempDir()
	writeNativeArchive(t, artifacts)
	observed := false
	err := acceptNative(request, artifacts, true, "", func(call toolCall) error {
		if slices.Contains(call.Args, "-tags=client_acceptance") {
			observed = true
			if !slices.Contains(call.Env, "AIGW_ACCEPTANCE_BASELINE="+baseline) {
				t.Error("real-client acceptance discarded the explicit published predecessor")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !observed {
		t.Fatal("real-client acceptance did not execute")
	}
}
