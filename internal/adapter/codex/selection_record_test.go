package codex

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// ownedFiles reads every file in the folders of targets, by path: the Codex
// files, AIGW's records of them and its catalogs.
type ownedFiles map[string]ownedFile

// ownedFile is a file as the file system identifies it and the bytes that it
// holds, so that a file that was written again is told from one that was
// left, whatever was written and whenever.
type ownedFile struct {
	info os.FileInfo
	data []byte
}

func readOwnedFiles(t *testing.T, targets []string) ownedFiles {
	t.Helper()
	files := ownedFiles{}
	for _, target := range targets {
		entries, err := os.ReadDir(filepath.Dir(target))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			path := filepath.Join(filepath.Dir(target), entry.Name())
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			files[path] = ownedFile{info: info, data: data}
		}
	}
	return files
}

// rewritten returns, sorted, the paths that hold another file or other bytes
// in now than in files, or that only one of them has.
func (files ownedFiles) rewritten(now ownedFiles) []string {
	var paths []string
	for path, file := range files {
		if current, kept := now[path]; !kept || !os.SameFile(file.info, current.info) || !bytes.Equal(file.data, current.data) {
			paths = append(paths, path)
		}
	}
	for path := range now {
		if _, known := files[path]; !known {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	return paths
}

// followedCodexChoice projects Codex's selection into count files, chooses
// model in the named ones as Codex's picker does, and returns the
// configuration that follows the choice with the files.
func followedCodexChoice(t *testing.T, count int, model string, chosen ...int) (domain.Config, []string) {
	t.Helper()
	cfg, targets := codexAdoptionFixture(t, count)
	for _, index := range chosen {
		chooseCodexModel(t, targets[index], model)
	}
	if followed, err := (codexAdapter{}).AdoptSelection(adapter.Dependencies{}, &cfg); err != nil || followed != domain.NewModelRef("gateway", model) {
		t.Fatalf("the choice is followed as %s: %v", followed, err)
	}
	return cfg, targets
}

// recordedModel is the model that AIGW's record of the Codex file at target
// names.
func recordedModel(t *testing.T, target string) string {
	t.Helper()
	snapshot, err := platform.CaptureFileSnapshot(codexRecordPath(target))
	if err != nil {
		t.Fatal(err)
	}
	state, err := codexStateForTarget(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return state.ProjectedModel
}

// recordingCodex are the dependencies of a write of the records alone: no
// Token store and no credential reader.
func recordingCodex() adapter.Dependencies {
	return adapter.Dependencies{Discovery: fixedDiscoverer{}, RecordOnly: []string{ID}}
}

// Of two Codex files, one shows the choice and the other still shows the
// recorded model. The record of the first moves alone: no Codex file and no
// catalog is written, the second file's record is left for the projection
// that writes that file, nothing is left to follow, and the receipt puts the
// earlier record back.
func TestTheRecordOfACodexChoiceMovesAlone(t *testing.T) {
	followed, targets := followedCodexChoice(t, 2, "gpt-terra", 0)
	// The record as the listing of the file's folder names it.
	record := filepath.Join(filepath.Dir(targets[0]), filepath.Base(codexRecordPath(targets[0])))
	before := readOwnedFiles(t, targets)

	plans, err := codexRegistry().Plan(recordingCodex(), followed, followed, ID)
	if err != nil || len(plans) != 1 {
		t.Fatalf("plans = %+v, %v; want one, for the file that shows the choice", plans, err)
	}
	if plan := plans[0]; plan.Action != adapter.ActionRecord || !sameTargetFile(plan.Target, targets[0]) || len(plan.Effects) != 1 || !sameTargetFile(plan.Effects[0], record) || !plan.ChangesState {
		t.Fatalf("the record's plan is %+v; the record is %s", plan, record)
	}
	if rewritten := before.rewritten(readOwnedFiles(t, targets)); len(rewritten) != 0 {
		t.Fatalf("planning the record wrote %v", rewritten)
	}
	receipt, changed, err := codexRegistry().Apply(t.Context(), recordingCodex(), followed, followed, ID)
	if err != nil || !changed {
		t.Fatalf("apply = %t, %v", changed, err)
	}
	if rewritten := before.rewritten(readOwnedFiles(t, targets)); !slices.Equal(rewritten, []string{record}) {
		t.Fatalf("recording the choice wrote %v, want the record %s alone", rewritten, record)
	}
	if first, second := recordedModel(t, targets[0]), recordedModel(t, targets[1]); first != "gpt-terra" || second != "gpt-sol" {
		t.Fatalf("the records name %s and %s", first, second)
	}
	again := followed.Clone()
	if chosen, err := (codexAdapter{}).AdoptSelection(adapter.Dependencies{}, &again); err != nil || !chosen.IsZero() {
		t.Fatalf("the recorded choice is followed again as %s: %v", chosen, err)
	}
	if plans, err := codexRegistry().Plan(recordingCodex(), followed, followed, ID); err != nil || len(plans) != 0 {
		t.Fatalf("records that hold the choice are planned again: %+v, %v", plans, err)
	}
	if err := receipt.Rollback(); err != nil {
		t.Fatal(err)
	}
	if rewritten := before.rewritten(readOwnedFiles(t, targets)); len(rewritten) != 1 || recordedModel(t, targets[0]) != "gpt-sol" || !bytes.Equal(readOwnedFiles(t, targets)[record].data, before[record].data) {
		t.Fatalf("the receipt restored %v; the record names %s", rewritten, recordedModel(t, targets[0]))
	}
}

// No record is written while a file holds more than the followed choice: an
// outside edit of another managed setting, in the file that chose or in
// another, or a model that the configuration does not follow.
func TestACodexRecordIsRefusedForAnythingButTheFollowedChoice(t *testing.T) {
	edit := func(t *testing.T, target, from, to string) {
		t.Helper()
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		edited := strings.Replace(string(data), from, to, 1)
		if edited == string(data) {
			t.Fatalf("the file lacks %q:\n%s", from, data)
		}
		if err := os.WriteFile(target, []byte(edited), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const address, edited = `base_url = "https://gateway.test/v1"`, `base_url = "https://edited.test/v1"`
	for name, after := range map[string]func(t *testing.T, targets []string){
		"an outside edit beside the choice": func(t *testing.T, targets []string) { edit(t, targets[0], address, edited) },
		"an outside edit in the other file": func(t *testing.T, targets []string) { edit(t, targets[1], address, edited) },
		"another added model in the file that chose": func(t *testing.T, targets []string) {
			edit(t, targets[0], `model = "gpt-terra"`, `model = "gpt-luna"`)
		},
	} {
		t.Run(name, func(t *testing.T) {
			followed, targets := followedCodexChoice(t, 2, "gpt-terra", 0)
			followed.Models[domain.NewModelRef("gateway", "gpt-luna")] = qualifiedModel("Luna", "gateway", "gpt-luna", domain.ProtocolResponses)
			after(t, targets)
			before := readOwnedFiles(t, targets)
			_, planErr := codexRegistry().Plan(recordingCodex(), followed, followed, ID)
			_, _, applyErr := codexRegistry().Apply(t.Context(), recordingCodex(), followed, followed, ID)
			for _, err := range []error{planErr, applyErr} {
				if _, conflict := errors.AsType[*adapter.ConflictError](err); !conflict {
					t.Fatalf("the record was not refused as a conflict: %v", err)
				}
			}
			if rewritten := before.rewritten(readOwnedFiles(t, targets)); len(rewritten) != 0 {
				t.Fatalf("the refusal wrote %v", rewritten)
			}
		})
	}
}

// When both files show the choice and the second record cannot be written,
// the first is put back: either every record moves or none.
func TestCodexRecordsMoveTogetherOrNotAtAll(t *testing.T) {
	followed, targets := followedCodexChoice(t, 2, "gpt-terra", 0, 1)
	before := readOwnedFiles(t, targets)
	failure := errors.New("the record cannot be written")
	original, writes := writeFileAtomicIfUnchanged, 0
	t.Cleanup(func() { writeFileAtomicIfUnchanged = original })
	writeFileAtomicIfUnchanged = func(path string, expected platform.FileSnapshot, data []byte, mode os.FileMode) (platform.FileSnapshot, error) {
		if writes++; writes == 2 {
			return platform.FileSnapshot{}, failure
		}
		return original(path, expected, data, mode)
	}
	if _, _, err := codexRegistry().Apply(t.Context(), recordingCodex(), followed, followed, ID); !errors.Is(err, failure) {
		t.Fatalf("apply = %v, want the failed write", err)
	}
	if writes != 2 {
		t.Fatalf("%d records were written, want the second to fail", writes)
	}
	for _, target := range targets {
		if recorded := recordedModel(t, target); recorded != "gpt-sol" {
			t.Fatalf("after the failure the record of %s names %s", target, recorded)
		}
	}
	for path, file := range readOwnedFiles(t, targets) {
		if !bytes.Equal(file.data, before[path].data) {
			t.Fatalf("after the failure %s holds other bytes", path)
		}
	}
}
