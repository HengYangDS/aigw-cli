package construction

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestNativeAcceptanceOwnsBuildConsumptionAndCleanup(t *testing.T) {
	for _, failure := range []string{"", "build", "inventory", "acceptance"} {
		t.Run(failure, func(t *testing.T) {
			request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
			var stage string
			var accepted bool
			want := errors.New("injected failure")
			err := acceptNative(request, false, func(call toolCall) error {
				if call.Name == "goreleaser" {
					stage = goReleaserStage(t, call.Args)
					if failure == "build" {
						return want
					}
					writeNativeInventory(t, stage, "valid")
					if failure == "inventory" {
						return os.Remove(filepath.Join(stage, "artifacts.json"))
					}
					return nil
				}
				accepted = true
				if call.Name != "go" || !slices.Equal(call.Args, []string{"test", "./tools/release", "-run", "^TestNativeProductJourney$/(portable_artifact_lifecycle|system_credential_store)$", "-count=1", "-v"}) {
					t.Fatalf("native acceptance escaped its existing test owner: %#v", call)
				}
				if !slices.Equal(call.Env, []string{"AIGW_ACCEPTANCE_RELEASE=" + stage}) {
					t.Fatalf("native acceptance environment = %#v", call.Env)
				}
				program := "aigw"
				if runtime.GOOS == "windows" {
					program += ".exe"
				}
				data, err := os.ReadFile(filepath.Join(stage, "aigw_1.2.3_"+runtime.GOOS+"_"+runtime.GOARCH, program))
				if err != nil || string(data) != "native candidate" {
					t.Fatalf("selected native bytes = %q, %v", data, err)
				}
				if failure == "acceptance" {
					return want
				}
				return nil
			})
			if (err != nil) != (failure != "") || accepted != (failure == "" || failure == "acceptance") {
				t.Fatalf("failure=%q accepted=%t error=%v", failure, accepted, err)
			}
			if _, err := os.Stat(filepath.Dir(stage)); !os.IsNotExist(err) {
				t.Fatalf("native build workspace survived: %s, %v", stage, err)
			}
		})
	}
}

func TestNativeInventorySelectsOneOwnedHostExecutable(t *testing.T) {
	for _, scenario := range []string{"valid", "relative", "relative-escaped", "missing", "malformed", "foreign", "duplicate", "escaped", "unreadable", "collision"} {
		t.Run(scenario, func(t *testing.T) {
			stage := t.TempDir()
			writeNativeInventory(t, stage, scenario)
			err := prepareNativeBinary(filepath.Dir(stage), stage, "1.2.3")
			if (err == nil) != (scenario == "valid" || scenario == "relative") {
				t.Fatalf("inventory %s: %v", scenario, err)
			}
		})
	}
}

func TestNativeClientAcceptanceSharesStageAndPropagatesFailure(t *testing.T) {
	for _, test := range []struct {
		name           string
		failure, calls int
	}{
		{"success", 0, 2},
		{"lifecycle failure", 1, 1},
		{"client failure", 2, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := buildRequest{Root: releaseRoot(t), Version: "1.2.3", Epoch: "1784246400"}
			var stage string
			var calls []toolCall
			want := errors.New("acceptance failed")
			err := acceptNative(request, true, func(call toolCall) error {
				if call.Name == "goreleaser" {
					stage = goReleaserStage(t, call.Args)
					writeNativeInventory(t, stage, "valid")
					return nil
				}
				calls = append(calls, call)
				if len(calls) == test.failure {
					return want
				}
				return nil
			})
			if (test.failure == 0 && err != nil) || (test.failure != 0 && !errors.Is(err, want)) {
				t.Fatalf("acceptance outcome = %v", err)
			}
			if len(calls) != test.calls {
				t.Fatalf("executed %d acceptance commands", len(calls))
			}
			for _, call := range calls {
				if call.Directory != request.Root || !slices.Equal(call.Env, []string{"AIGW_ACCEPTANCE_RELEASE=" + stage}) {
					t.Fatalf("acceptance lost stage ownership: %#v", call)
				}
			}
			if len(calls) == 2 && !slices.Equal(calls[1].Args, []string{"test", "-tags=client_acceptance", "./tools/release", "-run", "^TestNativeClientJourney$", "-count=1", "-v"}) {
				t.Fatalf("real-client command = %#v", calls[1])
			}
			if _, err := os.Stat(filepath.Dir(stage)); !os.IsNotExist(err) {
				t.Fatalf("acceptance stage survived: %v", err)
			}
		})
	}
}

func writeNativeInventory(t *testing.T, stage, scenario string) {
	t.Helper()
	if err := os.MkdirAll(stage, 0o700); err != nil {
		t.Fatal(err)
	}
	name := "aigw"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(stage, name)
	if err := os.WriteFile(binary, []byte("native candidate"), 0o700); err != nil {
		t.Fatal(err)
	}
	item := map[string]string{"name": name, "path": binary, "goos": runtime.GOOS, "goarch": runtime.GOARCH, "type": "Binary"}
	items := []map[string]string{item}
	switch scenario {
	case "relative":
		item["path"] = filepath.Join(filepath.Base(stage), name)
	case "relative-escaped":
		item["path"] = filepath.Join("..", name)
	case "missing":
		return
	case "foreign":
		item["goos"] = "other-platform"
	case "duplicate":
		items = append(items, item)
	case "escaped":
		item["path"] = filepath.Join(filepath.Dir(stage), name)
	case "unreadable":
		item["path"] = filepath.Join(stage, "missing")
	case "collision":
		if err := os.WriteFile(filepath.Join(stage, "aigw_1.2.3_"+runtime.GOOS+"_"+runtime.GOARCH), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	data, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	if scenario == "malformed" {
		data = []byte("{")
	}
	if err := os.WriteFile(filepath.Join(stage, "artifacts.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}
