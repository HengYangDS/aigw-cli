package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

const fixtureClientSupplier = "suppliers/windows-clients-fixture-native-packages.tar.gz"

// windowsPublicInput prepares a GitLab Windows job beside its checkout: the
// job-owned directory, trust files, a public input archive that carries the
// given members, and the job inputs that select it.
func windowsPublicInput(t *testing.T, members map[string][]byte) (string, []byte, map[string]string) {
	t.Helper()
	operation := t.TempDir()
	project, job := filepath.Join(operation, "project"), filepath.Join(operation, "aigw-ci-mise-123")
	for _, directory := range []string{project, job} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	trust := filepath.Join(operation, "trust")
	if err := os.WriteFile(trust, []byte("fixture-only public trust"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := packFixture(t, members, false)
	return job, archive, map[string]string{
		"AIGW_NATIVE_PLATFORM": "windows", "AIGW_NATIVE_INPUT_PACKAGE": "fixture", "AIGW_CANDIDATE_SOURCE": strings.Repeat("a", 40),
		"AIGW_NATIVE_INPUT_SHA256": fmt.Sprintf("%x", sha256.Sum256(archive)), "AIGW_NATIVE_CLIENT_SUPPLIER": fixtureClientSupplier,
		"AIGW_RELEASE_ALLOWED_SIGNERS_FILE": trust, "AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE": trust,
		"AIGW_RELEASE_ARTIFACT_SIGNER": "fixture@example.invalid",
		"CI_PROJECT_DIR":               project, "CI_PROJECT_URL": "https://gitlab.test/team/aigw", "CI_JOB_ID": "123",
	}
}

func officialClientMembers(t *testing.T) map[string][]byte {
	t.Helper()
	return map[string][]byte{
		"candidate/checksums.txt": []byte("test"),
		fixtureClientSupplier: packFixture(t, map[string][]byte{
			"clients/" + windowsCodexRoot + "/bin/codex.exe":     []byte("codex fixture"),
			"clients/" + windowsCodexRoot + "/codex-path/rg.exe": []byte("resource fixture"),
			"clients/" + windowsClaudeExecutable:                 []byte("claude fixture"),
		}, true),
	}
}

// servePublicInput answers the job's package download with archive and runs
// the system tar for extraction.
func servePublicInput(archive []byte) commandRunner {
	return func(call command) error {
		if call.Name == "tar" {
			return systemRunner(call)
		}
		return os.WriteFile(call.Args[slices.Index(call.Args, "--path")+1], archive, 0o600)
	}
}

func TestWindowsInputSupplyAdmitsOnlyTheExactPublicInput(t *testing.T) {
	job, archive, base := windowsPublicInput(t, officialClientMembers(t))
	operation := filepath.Dir(job)
	gitBin, bash, nodeDirectory := windowsClientTools(t, operation)
	source := base["AIGW_CANDIDATE_SOURCE"]
	for name, entry := range map[string]struct {
		env     map[string]string
		served  []byte
		problem string
	}{
		"admitted":           {served: archive},
		"tampered":           {served: []byte("tampered public bytes"), problem: "native public input checksum mismatch"},
		"wrong platform":     {env: map[string]string{"AIGW_NATIVE_PLATFORM": "all"}, problem: "requires the Windows platform"},
		"inexact checksum":   {env: map[string]string{"AIGW_NATIVE_INPUT_SHA256": strings.ToUpper(base["AIGW_NATIVE_INPUT_SHA256"])}, problem: "exact source and checksum"},
		"inexact source":     {env: map[string]string{"AIGW_CANDIDATE_SOURCE": "main"}, problem: "exact source and checksum"},
		"missing trust":      {env: map[string]string{"AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE": ""}, problem: "configured source and artifact trust"},
		"missing trust file": {env: map[string]string{"AIGW_RELEASE_ALLOWED_SIGNERS_FILE": filepath.Join(operation, "missing")}, problem: "configured source and artifact trust"},
		"missing signer":     {env: map[string]string{"AIGW_RELEASE_ARTIFACT_SIGNER": " "}, problem: "configured source and artifact trust"},
		"missing supplier":   {env: map[string]string{"AIGW_NATIVE_CLIENT_SUPPLIER": ""}, problem: "client supplier"},
	} {
		t.Run(name, func(t *testing.T) {
			for _, path := range []string{filepath.Join(job, "native-input"), filepath.Join(job, "native-input.tar")} {
				if err := os.RemoveAll(path); err != nil {
					t.Fatal(err)
				}
			}
			downloads := 0
			environment, err := supplyWindowsInput(environmentLookup(base, entry.env), func(call command) error {
				if call.Name == "tar" {
					return systemRunner(call)
				}
				download := []string{"exec", "--locked", "--", "glab", "packages", "download", "--repo", "https://gitlab.test/team/aigw", "--name", "fixture", "--version", source, "--filename", "public-inputs.tar", "--path"}
				if call.Name != "mise" || len(call.Args) != len(download)+1 || !slices.Equal(call.Args[:len(download)], download) ||
					!slices.Equal(call.Env, []string{"GLAB_ENABLE_CI_AUTOLOGIN=true", "GLAB_CONFIG_DIR=" + filepath.Join(job, "glab")}) {
					t.Fatalf("public input download escaped its job-owned scope: %#v", call)
				}
				downloads++
				return os.WriteFile(call.Args[len(download)], entry.served, 0o600)
			})
			if entry.problem != "" {
				if err == nil || !strings.Contains(err.Error(), entry.problem) || entry.served == nil && downloads != 0 {
					t.Fatalf("public input admission = %v, downloads=%d; want %q", err, downloads, entry.problem)
				}
				return
			}
			want := expectedWindowsClientEnvironment(filepath.Join(job, "native-input", "clients"), gitBin, bash, nodeDirectory)
			if err != nil || downloads != 1 || !slices.Equal(environment, want) {
				t.Fatalf("Windows client environment = %q, %v; want %q", environment, err, want)
			}
			checkWindowsClientIdentities(t, environment)
		})
	}
}

// Each failed step reports itself and returns no client environment.
func TestWindowsInputSupplyReportsEachFailedStep(t *testing.T) {
	failingTar := func(archive []byte) commandRunner {
		serve := servePublicInput(archive)
		return func(call command) error {
			if call.Name == "tar" {
				return errors.New("tar exited with status 1")
			}
			return serve(call)
		}
	}
	for name, entry := range map[string]struct {
		runner   func([]byte) commandRunner
		existing bool
		check    func(error) bool
	}{
		"download": {
			runner: func([]byte) commandRunner {
				return func(command) error { return errors.New("glab exited with status 1") }
			},
			check: func(err error) bool { return strings.Contains(err.Error(), "download native public input") },
		},
		"missing download": {
			runner: func([]byte) commandRunner { return func(command) error { return nil } },
			check: func(err error) bool {
				return strings.Contains(err.Error(), "native public input checksum mismatch") && errors.Is(err, fs.ErrNotExist)
			},
		},
		"extraction": {
			runner: failingTar,
			check:  func(err error) bool { return strings.Contains(err.Error(), "extract official Windows clients") },
		},
		"existing extraction": {runner: servePublicInput, existing: true, check: func(err error) bool { return errors.Is(err, fs.ErrExist) }},
	} {
		t.Run(name, func(t *testing.T) {
			job, archive, base := windowsPublicInput(t, officialClientMembers(t))
			windowsClientTools(t, filepath.Dir(job))
			if entry.existing {
				if err := os.Mkdir(filepath.Join(job, "native-input"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			environment, err := supplyWindowsInput(environmentLookup(base, nil), entry.runner(archive))
			if err == nil || environment != nil || !entry.check(err) {
				t.Fatalf("failed public input step = %v, environment %q", err, environment)
			}
		})
	}
}

func TestWindowsClientEnvironmentNamesTheMissingTool(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", root)
	if _, err := windowsClientEnvironment(root, "missing Git Bash"); err == nil || !strings.Contains(err.Error(), "locate Git") {
		t.Fatalf("Windows clients without Git = %v", err)
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	if err := os.WriteFile(filepath.Join(root, "git"+suffix), []byte("inert fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := windowsClientEnvironment(root, "missing Git Bash"); err == nil || !strings.Contains(err.Error(), "locate Node.js") {
		t.Fatalf("Windows clients without Node.js = %v", err)
	}
}

func TestGitHubWindowsClientSupplyRequiresEveryOfficialClient(t *testing.T) {
	root := t.TempDir()
	windowsClientTools(t, root)
	err := supplyWindowsClients(root, filepath.Join(root, "github-env"), []string{"@openai/codex@0.162.1"}, &bytes.Buffer{}, func(call command) error {
		if !slices.Contains(call.Args, "install") {
			return nil
		}
		// The installation reports success but supplies only Claude.
		file := filepath.Join(call.Args[slices.Index(call.Args, "--prefix")+1], filepath.FromSlash(windowsClaudeExecutable))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			return err
		}
		return os.WriteFile(file, []byte("claude fixture"), 0o600)
	})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("supply without the official Codex client = %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(root, windowsClientSupplyPrefix+"*")); len(left) != 0 {
		t.Fatalf("incomplete client supply survived: %q", left)
	}
}

func TestGitHubWindowsClientSupplyRetiresEveryFailedSupply(t *testing.T) {
	for name, entry := range map[string]struct {
		root, environment func(string) string
		removeBash        bool
		check             func(error) bool
	}{
		"missing root": {
			root:        func(root string) string { return filepath.Join(root, "missing") },
			environment: func(root string) string { return filepath.Join(root, "github-env") },
			check:       func(err error) bool { return errors.Is(err, fs.ErrNotExist) },
		},
		"unwritable environment": {
			root:        func(root string) string { return root },
			environment: func(root string) string { return filepath.Join(root, "missing", "github-env") },
			check:       func(err error) bool { return strings.Contains(err.Error(), "open GitHub environment") },
		},
		"missing Git Bash": {
			root:        func(root string) string { return root },
			environment: func(root string) string { return filepath.Join(root, "github-env") },
			removeBash:  true,
			check:       func(err error) bool { return err.Error() == "Claude requires native Git Bash" },
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			_, bash, _ := windowsClientTools(t, root)
			if entry.removeBash {
				if err := os.Remove(bash); err != nil {
					t.Fatal(err)
				}
			}
			err := supplyWindowsClients(entry.root(root), entry.environment(root), []string{"@openai/codex@0.162.1"}, &bytes.Buffer{}, installOfficialClients)
			if err == nil || !entry.check(err) {
				t.Fatalf("failed client supply = %v", err)
			}
			if left, _ := filepath.Glob(filepath.Join(root, windowsClientSupplyPrefix+"*")); len(left) != 0 {
				t.Fatalf("failed client supply survived: %q", left)
			}
		})
	}
}

// installOfficialClients stands in for npm and writes the two official client
// executables under the requested prefix.
func installOfficialClients(call command) error {
	if !slices.Contains(call.Args, "install") {
		return nil
	}
	prefix := call.Args[slices.Index(call.Args, "--prefix")+1]
	for path, data := range map[string]string{windowsCodexRoot + "/bin/codex.exe": "codex fixture", windowsClaudeExecutable: "claude fixture"} {
		file := filepath.Join(prefix, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(file, []byte(data), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func TestWindowsClientsRequireGitBash(t *testing.T) {
	root := t.TempDir()
	_, bash, _ := windowsClientTools(t, root)
	if err := os.Remove(bash); err != nil {
		t.Fatal(err)
	}
	if _, err := windowsClientEnvironment(filepath.Join(root, "clients"), "native Claude requires Git Bash"); err == nil || err.Error() != "native Claude requires Git Bash" {
		t.Fatalf("Windows clients without Git Bash = %v", err)
	}
}

func TestGitHubWindowsClientSupplyRecordsItsOwnedEnvironment(t *testing.T) {
	root := t.TempDir()
	githubEnvironment := filepath.Join(root, "github-env")
	if err := os.WriteFile(githubEnvironment, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	gitBin, bash, nodeDirectory := windowsClientTools(t, root)
	packages := []string{"@openai/codex@0.162.1", "@anthropic-ai/claude-code-win32-x64@2.1.296"}
	var calls []command
	var stdout bytes.Buffer
	err := run([]string{"native-clients", "supply", "--root", root, "--github-env", githubEnvironment, "--package", packages[0], "--package", packages[1]}, &stdout, func(call command) error {
		calls = append(calls, call)
		return installOfficialClients(call)
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(readFile(t, githubEnvironment))), "\n")
	clients, found := strings.CutPrefix(lines[0], "AIGW_NATIVE_CLIENT_SUPPLY=")
	if !found || filepath.Dir(clients) != root || !strings.HasPrefix(filepath.Base(clients), windowsClientSupplyPrefix) {
		t.Fatalf("Windows client supply is not recorded before installation: %q", lines)
	}
	gitConfig := filepath.Join(clients, "gitconfig")
	isolated := []string{"GIT_CONFIG_GLOBAL=" + gitConfig}
	want := []command{
		{Name: "git", Args: []string{"config", "--file", gitConfig, "core.autocrlf", "false"}},
		{Name: "mise", Args: append([]string{"exec", "--locked", "--", "npm", "install", "--prefix", clients, "--ignore-scripts", "--save-exact", "--no-audit", "--no-fund"}, packages...), Env: isolated},
		{Name: "mise", Args: []string{"exec", "--locked", "--", "npm", "audit", "signatures", "--prefix", clients}, Env: isolated},
	}
	if !reflect.DeepEqual(calls, want) || string(readFile(t, filepath.Join(clients, "package.json"))) != "{\"private\":true}\n" {
		t.Fatalf("official Windows client installation = %#v", calls)
	}
	environment := expectedWindowsClientEnvironment(clients, gitBin, bash, nodeDirectory)
	if !slices.Equal(lines[1:], environment) {
		t.Fatalf("GitHub client environment = %q, want %q", lines[1:], environment)
	}
	checkWindowsClientIdentities(t, environment)
	if digest := fmt.Sprintf("%x", sha256.Sum256([]byte("codex fixture"))); !strings.Contains(stdout.String(), digest) {
		t.Fatalf("official client digests were not reported: %s", stdout.String())
	}
	t.Setenv("AIGW_NATIVE_CLIENT_SUPPLY", clients)
	if err := run([]string{"native-clients", "retire"}, &bytes.Buffer{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(clients); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Windows client supply survived retirement: %v", err)
	}
}

func TestGitHubWindowsClientSupplyRetiresItsDirectoryOnFailure(t *testing.T) {
	root := t.TempDir()
	githubEnvironment := filepath.Join(root, "github-env")
	windowsClientTools(t, root)
	failure := errors.New("npm audit failed")
	err := run([]string{"native-clients", "supply", "--root", root, "--github-env", githubEnvironment, "--package", "@openai/codex@0.162.1"}, &bytes.Buffer{}, func(call command) error {
		if slices.Contains(call.Args, "audit") {
			return failure
		}
		return nil
	})
	lines := strings.Split(strings.TrimSpace(string(readFile(t, githubEnvironment))), "\n")
	clients := strings.TrimPrefix(lines[0], "AIGW_NATIVE_CLIENT_SUPPLY=")
	if !errors.Is(err, failure) || len(lines) != 1 {
		t.Fatalf("failed Windows client supply = %v, recorded %q", err, lines)
	}
	if _, err := os.Lstat(clients); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed Windows client supply survived: %v", err)
	}
}

func TestGitHubWindowsClientRetirementRemovesOnlyItsOwnSupply(t *testing.T) {
	foreign := t.TempDir()
	for _, owned := range []string{foreign, windowsClientSupplyPrefix + "relative"} {
		t.Setenv("AIGW_NATIVE_CLIENT_SUPPLY", owned)
		if err := run([]string{"native-clients", "retire"}, &bytes.Buffer{}, nil); err == nil {
			t.Fatalf("retirement admitted an unowned path %q", owned)
		}
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("retirement changed an unowned directory: %v", err)
	}
	t.Setenv("AIGW_NATIVE_CLIENT_SUPPLY", "")
	if err := run([]string{"native-clients", "retire"}, &bytes.Buffer{}, nil); err != nil {
		t.Fatalf("retirement without a supply = %v", err)
	}
}

func TestNativeClientsCommandRequiresExactInputs(t *testing.T) {
	for _, arguments := range [][]string{
		{"native-clients"},
		{"native-clients", "install"},
		{"native-clients", "retire", "extra"},
		{"native-clients", "supply", "--root", "/r", "--github-env", "/e"},
		{"native-clients", "supply", "--root", "/r", "--package", "p"},
		{"native-clients", "supply", "--github-env", "/e", "--package", "p"},
		{"native-clients", "supply", "--root", "/r", "--github-env", "/e", "--package", "p", "extra"},
	} {
		if err := run(arguments, &bytes.Buffer{}, func(command) error {
			t.Fatalf("%q reached client installation", arguments)
			return nil
		}); err == nil || !strings.Contains(err.Error(), "usage: ci native-clients") {
			t.Fatalf("%q was admitted: %v", arguments, err)
		}
	}
}

// expectedWindowsClientEnvironment is the acceptance environment for clients
// installed under root with the given Git, Git Bash and Node.js locations.
func expectedWindowsClientEnvironment(root, gitBin, bash, nodeDirectory string) []string {
	codex := filepath.Join(root, filepath.FromSlash(windowsCodexRoot))
	return []string{
		"AIGW_ACCEPTANCE_CODEX=" + filepath.Join(codex, "bin", "codex.exe"),
		"AIGW_ACCEPTANCE_CLAUDE=" + filepath.Join(root, filepath.FromSlash(windowsClaudeExecutable)),
		"AIGW_ACCEPTANCE_CLIENT_PATH=" + strings.Join([]string{filepath.Join(codex, "codex-path"), nodeDirectory, gitBin, filepath.Dir(bash), filepath.Join(os.Getenv("SystemRoot"), "System32")}, ";"),
		"CLAUDE_CODE_GIT_BASH_PATH=" + bash,
	}
}

func checkWindowsClientIdentities(t *testing.T, environment []string) {
	t.Helper()
	for client, key := range map[string]string{"codex": "AIGW_ACCEPTANCE_CODEX=", "claude": "AIGW_ACCEPTANCE_CLAUDE="} {
		index := slices.IndexFunc(environment, func(entry string) bool { return strings.HasPrefix(entry, key) })
		if index < 0 {
			t.Fatalf("Windows client environment omits %s", key)
		}
		if data, err := os.ReadFile(strings.TrimPrefix(environment[index], key)); err != nil || string(data) != client+" fixture" {
			t.Fatalf("selected %s client identity is unavailable: %q, %v", client, data, err)
		}
	}
}

// windowsClientTools puts inert Git and Node.js stand-ins first on PATH beside
// a Git Bash file and returns Git's command directory, Git Bash and Node's
// directory.
func windowsClientTools(t *testing.T, root string) (string, string, string) {
	t.Helper()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	gitBin, nodeDirectory := filepath.Join(root, "git", "cmd"), filepath.Join(root, "node")
	bash := filepath.Join(root, "git", "bin", "bash.exe")
	for _, path := range []string{filepath.Join(gitBin, "git"+suffix), filepath.Join(nodeDirectory, "node"+suffix), bash} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("inert fixture"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", strings.Join([]string{gitBin, nodeDirectory, os.Getenv("PATH")}, string(os.PathListSeparator)))
	return gitBin, bash, nodeDirectory
}

func packFixture(t *testing.T, files map[string][]byte, compressed bool) []byte {
	t.Helper()
	var archive bytes.Buffer
	var output io.Writer = &archive
	var zipped *gzip.Writer
	if compressed {
		zipped = gzip.NewWriter(&archive)
		output = zipped
	}
	writer := tar.NewWriter(output)
	for _, name := range slices.Sorted(maps.Keys(files)) {
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(files[name]))}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(files[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if zipped != nil {
		if err := zipped.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return archive.Bytes()
}
