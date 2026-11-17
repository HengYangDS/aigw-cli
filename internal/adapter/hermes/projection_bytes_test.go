package hermes

// The tests in this file pin, byte for byte, what AIGW writes into a Hermes
// configuration and into its ownership record. Each case writes its input as
// config.yaml in a temporary directory, prepares the projection of
// testDesired through Prepare, and compares the bytes that the plan would
// write with expected files under testdata/projection-bytes/<case>/:
//
//   - config.yaml.golden: the configuration after the projection;
//   - record.json.golden: the ownership record after the projection;
//   - withdrawn.yaml.golden: the configuration that Prepare(path, nil) would
//     write after the projection was applied;
//   - refusal.txt.golden: the refusal's text and one final newline, for a case
//     that Prepare refuses.
//
// The expected files record today's output, which is not always what an
// operator would want. A test that finds no expected file fails and writes
// nothing. To write the expected files from the current output, run this
// package's tests on a workstation with AIGW_TEST_UPDATE_YAML=1, read every
// file that the run wrote, and run the tests again without the variable. With
// the variable set, each test writes its files and fails, so the variable is
// never the reason that a test passes; when CI is set, it fails and writes
// nothing.
//
// The .golden suffix keeps the repository's formatter from rewriting the
// expected bytes.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const updatePinnedBytes = "AIGW_TEST_UPDATE_YAML"

// pinnedProjection is one Hermes configuration whose projection, ownership
// record and withdrawal are pinned. A refused case pins the refusal's text.
type pinnedProjection struct {
	name    string
	input   string
	refused bool
}

// lines joins its arguments into a YAML text with a final newline.
func lines(text ...string) string { return strings.Join(text, "\n") + "\n" }

// crlf turns every line ending of text into CR LF.
func crlf(text string) string { return strings.ReplaceAll(text, "\n", "\r\n") }

var pinnedProjections = []pinnedProjection{
	{name: "long-values", input: lines(
		"display:",
		"  banner: This welcome line that the operator wrote is longer than eighty characters on purpose",
		"toolsets: [web, terminal, file, browser, vision, image_gen, tts, skills, memory, session_search, todo]",
	)},
	{name: "comments", input: lines(
		"# Hermes configuration for this workstation",
		"",
		"# Display settings",
		"display:",
		"  theme: dark # chosen by the operator",
		"  # Compact layout saves space.",
		"  compact: true",
		"  # The theme follows the terminal.",
		"",
		"# Terminal settings",
		"terminal:",
		"  backend: local",
		"model: # selected in Hermes",
		"  provider: original",
		"  default: previous-model",
	)},
	{name: "blank-lines", input: lines(
		"display:",
		"  theme: dark",
		"",
		"  compact: true",
		"",
		"",
		"terminal:",
		"  backend: local",
		"",
		"agent:",
		"  max_turns: 60",
	)},
	{name: "quoted-strings", input: lines(
		"display:",
		"  greeting: 'Hello, operator'",
		`  farewell: "Goodbye, operator"`,
		"  empty: ''",
		"  looks_boolean: 'true'",
		`  looks_number: "123"`,
		"  looks_date: '2026-11-17'",
	)},
	{name: "block-scalars", input: lines(
		"agent:",
		"  system_prompt: |",
		"    You are a careful assistant.",
		"    Answer in plain words.",
		"  signature: |-",
		"    Sent from Hermes",
		"    on this workstation",
		"  summary: >",
		"    This folded text",
		"    reads as one line.",
	)},
	{name: "four-space-indent", input: lines(
		"display:",
		"    theme: dark",
		"    compact: true",
		"model:",
		"    provider: original",
		"    default: previous-model",
		"providers:",
		"    personal:",
		"        base_url: https://personal.test",
	)},
	{name: "compact-sequence", input: lines(
		"toolsets:",
		"- web",
		"- terminal",
		"platform_toolsets:",
		"  cli:",
		"  - web",
		"  - file",
	)},
	{name: "document-marker", input: lines(
		"---",
		"display:",
		"  theme: dark",
	)},
	{name: "no-final-newline", input: strings.TrimSuffix(lines(
		"display:",
		"  theme: dark",
	), "\n")},
	{name: "crlf", input: crlf(lines(
		"# Operator settings",
		"display:",
		"  theme: dark # chosen by the operator",
		"terminal:",
		"  backend: local",
	))},
	{name: "byte-order-mark", input: "\ufeff" + lines(
		"display:",
		"  theme: dark",
	)},
	{name: "empty-file", input: ""},
	{name: "comment-only-file", input: lines(
		"# Hermes settings are added below.",
		"# Nothing is set yet.",
	)},
	{name: "anchors", input: lines(
		"defaults: &defaults",
		"  timeout: 30",
		"  retries: 2",
		"tools:",
		"  web:",
		"    <<: *defaults",
		"    timeout: 60",
		"  browser: *defaults",
	)},
	{name: "two-documents", refused: true, input: lines(
		"display:",
		"  theme: dark",
		"---",
		"display:",
		"  theme: light",
	)},
	{name: "duplicate-key", refused: true, input: lines(
		"display:",
		"  theme: dark",
		"  theme: light",
	)},
	{name: "mapping-model", input: lines(
		"model:",
		"  provider: original",
		"  default: previous-model",
		"  base_url: https://previous.test",
		"  api_mode: chat_completions",
		"  temperature: 0.3",
		"providers:",
		"  personal:",
		"    base_url: https://personal.test",
	)},
	{name: "scalar-model", input: lines(
		"model: previous-model",
	)},
}

func TestProjectionWritesPinnedBytes(t *testing.T) {
	for _, pinned := range pinnedProjections {
		t.Run(pinned.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(pinned.input), 0o600); err != nil {
				t.Fatal(err)
			}
			desired := testDesired()
			plan, err := Prepare(path, &desired)
			if pinned.refused {
				if err == nil {
					t.Fatalf("Prepare accepted the configuration: action %s", plan.Action)
				}
				requirePinnedBytes(t, pinned.name, "refusal.txt.golden", []byte(err.Error()+"\n"))
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if plan.Action != "write" {
				t.Fatalf("projection action = %s, want write", plan.Action)
			}
			requirePinnedBytes(t, pinned.name, "config.yaml.golden", plan.configAfter)
			requirePinnedBytes(t, pinned.name, "record.json.golden", plan.stateAfter)
			if _, err := plan.Apply(); err != nil {
				t.Fatal(err)
			}
			withdrawal, err := Prepare(path, nil)
			if err != nil {
				t.Fatal(err)
			}
			if withdrawal.Action != "remove" || withdrawal.removeConfig {
				t.Fatalf("withdrawal action = %s, removes file = %t; want a rewrite", withdrawal.Action, withdrawal.removeConfig)
			}
			requirePinnedBytes(t, pinned.name, "withdrawn.yaml.golden", withdrawal.configAfter)
		})
	}
}

// requirePinnedBytes compares got with the expected file name of a case. With
// AIGW_TEST_UPDATE_YAML set outside CI it writes got as that file instead, and
// fails.
func requirePinnedBytes(t *testing.T, pinnedCase, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "projection-bytes", pinnedCase, name)
	if os.Getenv(updatePinnedBytes) != "" {
		if os.Getenv("CI") != "" {
			t.Fatalf("%s is set in CI; expected files are written only on a workstation", updatePinnedBytes)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Errorf("wrote %s from today's output; read it, then run the tests without %s", path, updatePinnedBytes)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("read the expected file: %v; to write it from today's output, see this file's header", err)
		return
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from today's output\n got: %q\nwant: %q", path, got, want)
	}
}
