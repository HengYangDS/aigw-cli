package construction

import (
	"os"
	"strings"
	"testing"
)

func TestPublishedPredecessorRefusesReleasesThatCannotReadTheTeamFile(t *testing.T) {
	for _, tag := range []string{"v0.6.1", "v0.3.1", "v0.1.0", "v0.7.0-rc.1"} {
		t.Run(tag, func(t *testing.T) {
			stage := t.TempDir() + "/stage"
			_, err := preparePublishedBaseline(t.Context(), t.TempDir(), t.TempDir(), tag, stage)
			version := strings.TrimPrefix(tag, "v")
			if err == nil || !strings.Contains(err.Error(), version) || !strings.Contains(err.Error(), "0.7.0 is the oldest release a journey can start from") || !strings.Contains(err.Error(), "aigw.toml") {
				t.Fatalf("predecessor %s was not refused for its schema: %v", tag, err)
			}
			if _, statErr := os.Stat(stage); statErr == nil {
				t.Fatalf("refused predecessor %s created its stage", tag)
			}
		})
	}
	for _, tag := range []string{"v0.7.0", "v0.7.1", "v0.8.0", "v1.0.0"} {
		_, err := preparePublishedBaseline(t.Context(), t.TempDir(), t.TempDir(), tag, t.TempDir()+"/stage")
		if err == nil || strings.Contains(err.Error(), "oldest release") {
			t.Fatalf("predecessor %s was refused by the floor, or admitted without its matrix: %v", tag, err)
		}
	}
}

func TestEveryPredecessorSelectorIsRefusedBeforeAnyToolRuns(t *testing.T) {
	for name, input := range map[string]NativeAcceptance{
		"github tag":    {Peer: "github", Repository: "group/product", Tag: "v0.7.0", BaselineTag: "v0.6.1"},
		"gitlab tag":    {Peer: "gitlab", Repository: "group/product", Tag: "v0.7.0", BaselineTag: "v0.6.1"},
		"local matrix":  {Artifacts: "/candidate", BaselineArtifacts: "/baseline", BaselineTag: "v0.6.1"},
		"retained":      {Candidate: true, InputArchive: "/inputs.tar", InputSHA256: strings.Repeat("a", 64), CandidateSource: strings.Repeat("b", 40), BaselineTag: "v0.6.1"},
		"package":       {Candidate: true, Peer: "gitlab", Repository: "group/product", InputPackage: "native-inputs", InputSHA256: strings.Repeat("a", 64), CandidateSource: strings.Repeat("b", 40), BaselineTag: "v0.6.1"},
		"version 0.0.1": {Peer: "github", Repository: "group/product", Tag: "v0.7.0", BaselineTag: "v0.0.1"},
	} {
		t.Run(name, func(t *testing.T) {
			err := acceptNativeInput(t.Context(), input, buildRequest{Root: t.TempDir(), Version: "0.7.0", Epoch: "1784246400"}, func(call toolCall) error {
				t.Fatalf("a refused predecessor reached %s %v", call.Name, call.Args)
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), "0.7.0 is the oldest release a journey can start from") {
				t.Fatalf("selector was not refused: %v", err)
			}
		})
	}
}
