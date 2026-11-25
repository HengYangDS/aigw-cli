package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// fixtureHermesInstaller stands in for the official installer bytes that a

// supply downloads; fixtureRelease pins exactly these bytes.

// fixtureReleaseFor drops the Hermes pins on a host whose journeys omit Hermes.

// Every supported host supplies exactly the clients that the release owner's

// journey declares for its platform, under that host's official layout.

// The official installer runs only after its exact pinned bytes are verified,

// and its homes, caches and launcher stay inside the supply.

// A signal during supply stops before the next step, exports no clients and

// removes the partial supply, including a signal during its last step.

func TestClientSupplyRequiresEveryOfficialClient(t *testing.T) {
	root := t.TempDir()
	clientTools(t, root)
	layout := clientLayouts["linux/amd64"]
	var calls []command
	supply := recordingSupply(&calls, layout)
	err := supplyClients("linux/amd64", root, filepath.Join(root, "github-env"), fixtureRelease(), &bytes.Buffer{}, func(call command) error {
		if !slices.Contains(call.Args, "install") {
			return supply(call)
		}
		// The installation reports success but supplies only Claude.
		file := filepath.Join(call.Args[slices.Index(call.Args, "--prefix")+1], filepath.FromSlash(layout.claudeExecutable()))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			return err
		}
		return os.WriteFile(file, []byte("claude fixture"), 0o600)
	}, nil)
	if !errors.Is(err, fs.ErrNotExist) || !slices.ContainsFunc(calls, func(call command) bool { return call.Name == "bash" }) {
		t.Fatalf("supply without the official Codex client = %v after %#v", err, calls)
	}
	if left, _ := filepath.Glob(filepath.Join(root, clientSupplyPrefix+"*")); len(left) != 0 {
		t.Fatalf("incomplete client supply survived: %q", left)
	}
}

func TestClientSupplyRetiresEveryFailedSupply(t *testing.T) {
	for name, entry := range map[string]struct {
		root, environment func(string) string
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
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			clientTools(t, root)
			var calls []command
			err := supplyClients("linux/amd64", entry.root(root), entry.environment(root), fixtureRelease(), &bytes.Buffer{}, recordingSupply(&calls, clientLayouts["linux/amd64"]), nil)
			if err == nil || !entry.check(err) {
				t.Fatalf("failed client supply = %v", err)
			}
			if left, _ := filepath.Glob(filepath.Join(root, clientSupplyPrefix+"*")); len(left) != 0 {
				t.Fatalf("failed client supply survived: %q", left)
			}
		})
	}
}

// The supply keeps its exact installation and isolation.
func TestClientSupplyRecordsItsOwnedInstallation(t *testing.T) {
	root := t.TempDir()
	githubEnvironment := filepath.Join(root, "github-env")
	if err := os.WriteFile(githubEnvironment, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	clientTools(t, root)
	layout := clientLayouts["linux/amd64"]
	var calls []command
	release := fixtureRelease()
	if err := supplyClients("linux/amd64", root, githubEnvironment, release, &bytes.Buffer{}, recordingSupply(&calls, layout), nil); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(readFile(t, githubEnvironment))), "\n")
	clients := strings.TrimPrefix(lines[0], "AIGW_NATIVE_CLIENT_SUPPLY=")
	gitConfig := filepath.Join(clients, "gitconfig")
	isolated := []string{"GIT_CONFIG_GLOBAL=" + gitConfig}
	want := []command{
		{Name: "git", Args: []string{"config", "--file", gitConfig, "core.autocrlf", "false"}},
		{Name: "mise", Args: append([]string{"exec", "--locked", "--", "npm", "install", "--prefix", clients, "--ignore-scripts", "--save-exact", "--no-audit", "--no-fund"},
			release.packages(layout)...), Env: isolated},
		{Name: "mise", Args: []string{"exec", "--locked", "--", "npm", "audit", "signatures", "--prefix", clients}, Env: isolated},
	}
	if len(calls) < len(want) || !reflect.DeepEqual(calls[:len(want)], want) || string(readFile(t, filepath.Join(clients, "package.json"))) != "{\"private\":true}\n" {
		t.Fatalf("official client installation = %#v", calls)
	}
}

// A failed step retires the supply's directory.
func TestClientSupplyRetiresItsDirectoryOnFailure(t *testing.T) {
	root := t.TempDir()
	githubEnvironment := filepath.Join(root, "github-env")
	clientTools(t, root)
	failure := errors.New("npm audit failed")
	err := supplyClients("linux/amd64", root, githubEnvironment, fixtureRelease(), &bytes.Buffer{}, func(call command) error {
		if slices.Contains(call.Args, "audit") {
			return failure
		}
		return nil
	}, nil)
	lines := strings.Split(strings.TrimSpace(string(readFile(t, githubEnvironment))), "\n")
	clients := strings.TrimPrefix(lines[0], "AIGW_NATIVE_CLIENT_SUPPLY=")
	if !errors.Is(err, failure) || len(lines) != 1 {
		t.Fatalf("failed client supply = %v, recorded %q", err, lines)
	}
	if _, err := os.Lstat(clients); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed client supply survived: %v", err)
	}
}

func TestNativeClientRetirementRemovesOnlyItsOwnSupply(t *testing.T) {
	foreign := t.TempDir()
	for _, owned := range []string{foreign, clientSupplyPrefix + "relative"} {
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

// A process that outlived its journey keeps the supply and fails retirement,

// whichever process it is, and an unproved inventory fails it as well.

func TestNativeClientsCommandRequiresExactInputs(t *testing.T) {
	for _, arguments := range [][]string{
		{"native-clients"},
		{"native-clients", "install"},
		{"native-clients", "retire", "extra"},
		{"native-clients", "supply", "--root", "/r", "--codex", "1.0.0", "--claude", "1.0.0"},
		{"native-clients", "supply", "--github-env", "/e", "--codex", "1.0.0", "--claude", "1.0.0"},
		{"native-clients", "supply", "--root", "/r", "--github-env", "/e", "--package", "p"},
		{"native-clients", "supply", "--root", "/r", "--github-env", "/e", "--codex", "1.0.0", "--claude", "1.0.0", "extra"},
	} {
		if err := run(arguments, &bytes.Buffer{}, func(command) error {
			t.Fatalf("%q reached client installation", arguments)
			return nil
		}); err == nil || !strings.Contains(err.Error(), "usage: ci native-clients") {
			t.Fatalf("%q was admitted: %v", arguments, err)
		}
	}
}

// recordingSupply stands in for npm, curl, the official Hermes installer and

// Git: it records each call and writes the files the official tools produce.

// expectedPosixClientEnvironment is the acceptance environment for clients and

// the official Hermes installation under supply.

func checkClientIdentities(t *testing.T, environment []string) {
	t.Helper()
	for client, key := range map[string]string{"codex": "AIGW_ACCEPTANCE_CODEX=", "claude": "AIGW_ACCEPTANCE_CLAUDE=", "hermes": "AIGW_ACCEPTANCE_HERMES="} {
		index := slices.IndexFunc(environment, func(entry string) bool { return strings.HasPrefix(entry, key) })
		if index < 0 {
			if client == "hermes" {
				continue
			}
			t.Fatalf("client environment omits %s", key)
		}
		if data, err := os.ReadFile(strings.TrimPrefix(environment[index], key)); err != nil || string(data) != client+" fixture" {
			t.Fatalf("selected %s client identity is unavailable: %q, %v", client, data, err)
		}
	}
}

// clientTools puts inert Git and Node.js stand-ins first on PATH and returns
// Git's command directory and Node's directory.
func clientTools(t *testing.T, root string) (string, string) {
	t.Helper()
	gitBin, nodeDirectory := filepath.Join(root, "git", "cmd"), filepath.Join(root, "node")
	for _, path := range []string{filepath.Join(gitBin, "git"), filepath.Join(nodeDirectory, "node")} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("inert fixture"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", strings.Join([]string{gitBin, nodeDirectory, os.Getenv("PATH")}, string(os.PathListSeparator)))
	return gitBin, nodeDirectory
}
