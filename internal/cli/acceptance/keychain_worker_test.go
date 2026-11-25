package cli_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/platform/processtest"
	"aigw-cli/internal/secret"
)

// keychainWorkerSource is a credential worker that answers each operation as
// macOS answers it for the items of one test. The items lie in the folder
// keychain of the nearest folder above the worker's own file, so a copy of the
// worker below that folder, such as the credential reader, sees the same
// items: own holds the value of each item that AIGW stored, and other names
// each item that another program stored. AIGW observes and deletes an item of
// another program, and its read, its emptiness read and its replacement are
// refused while the keychain is unlocked. The worker records each operation,
// and a copy below a folder named credential, as the credential reader is,
// records it as the reader's.
const keychainWorkerSource = `package main

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func answer(yes bool) {
	if yes {
		_, _ = os.Stdout.WriteString("1")
		return
	}
	_, _ = os.Stdout.WriteString("0")
}

func main() {
	if len(os.Args) < 3 {
		os.Exit(3)
	}
	folder := filepath.Dir(os.Args[0])
	for {
		if info, err := os.Stat(filepath.Join(folder, "keychain")); err == nil && info.IsDir() {
			break
		}
		if folder == filepath.Dir(folder) {
			os.Exit(3)
		}
		folder = filepath.Dir(folder)
	}
	keychain := filepath.Join(folder, "keychain")
	operation := strings.TrimPrefix(os.Args[1], "__aigw-native-credential-")
	recorded := operation
	if strings.Contains(filepath.ToSlash(os.Args[0]), "/credential/") {
		recorded = "reader " + operation
	}
	log, err := os.OpenFile(filepath.Join(keychain, "operations"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		os.Exit(3)
	}
	if _, err := log.WriteString(recorded + " " + strings.Join(os.Args[3:], " ") + "\n"); err != nil {
		os.Exit(3)
	}
	if err := log.Close(); err != nil {
		os.Exit(3)
	}
	if operation == "list" {
		var names []string
		for _, owner := range []string{"own", "other"} {
			entries, err := os.ReadDir(filepath.Join(keychain, owner))
			if err != nil {
				os.Exit(3)
			}
			for _, entry := range entries {
				// A balance credential's name stays in the worker.
				if !strings.Contains(entry.Name(), "@") {
					names = append(names, entry.Name())
				}
			}
		}
		sort.Strings(names)
		_, _ = os.Stdout.WriteString(strings.Join(names, "\n"))
		return
	}
	if len(os.Args) != 4 {
		os.Exit(3)
	}
	own, other := filepath.Join(keychain, "own", os.Args[3]), filepath.Join(keychain, "other", os.Args[3])
	_, foreign := os.Stat(other)
	value, stored := os.ReadFile(own)
	switch {
	case operation == "delete":
		_, _ = os.Remove(own), os.Remove(other)
	case operation == "exists":
		answer(foreign == nil || stored == nil)
	case foreign == nil:
		_, _ = os.Stderr.WriteString("{\"keychain_status\":-25293,\"keychain_unlocked\":true}\n")
		os.Exit(3)
	case operation == "write":
		replacement, err := io.ReadAll(os.Stdin)
		if err != nil || len(replacement) == 0 || os.WriteFile(own, replacement, 0o600) != nil {
			os.Exit(3)
		}
	case stored != nil:
		os.Exit(2)
	case operation == "read":
		_, _ = os.Stdout.Write(value)
	case operation == "empty":
		answer(len(value) == 0)
	default:
		os.Exit(3)
	}
}
`

// keychainWorker builds the worker once for the tests of one process and
// returns its bytes, which each test copies below a folder of its own.
var keychainWorker = sync.OnceValues(func() ([]byte, error) {
	folder, err := os.MkdirTemp("", "aigw-keychain-worker-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(folder) }()
	source := filepath.Join(folder, "worker.go")
	if err := os.WriteFile(source, []byte(keychainWorkerSource), 0o600); err != nil {
		return nil, err
	}
	program := filepath.Join(folder, "worker")
	build := exec.Command("go", "build", "-o", program, source)
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off")
	if output, err := build.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("build the keychain worker: %w\n%s", err, output)
	}
	return os.ReadFile(program)
})

// keychain is the system keyring of one test: the product's own keyring store
// over the worker of keychainWorkerSource. Only the worker is a double, so a
// command meets an item of another program where the product meets it: present
// when it is observed, refused when it is read, replaced or asked whether it
// is empty, and gone once it is deleted.
type keychain struct {
	secret.Store
	// root holds the worker, the items and, once a reader is bound, the
	// credential reader.
	root string
}

// withItemsOfAnotherProgram gives app a system keyring that holds an item of
// another program at each of slots, named by the account under which the
// system tool names it: a provider's ID for its Token, and
// diagnostic@<provider> for its balance credentials.
func withItemsOfAnotherProgram(t *testing.T, app *cli.App, slots ...string) *keychain {
	t.Helper()
	worker, err := keychainWorker()
	if err != nil {
		t.Fatal(err)
	}
	items := &keychain{root: t.TempDir()}
	processtest.Write(t, items.worker(), string(worker))
	for _, folder := range []string{"own", "other"} {
		if err := os.MkdirAll(filepath.Join(items.root, "keychain", folder), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, slot := range slots {
		writeFile(t, filepath.Join(items.root, "keychain", "other", slot), nil, 0o600)
	}
	if items.Store, err = secret.Select(secret.Selection{Backend: "keyring", Executable: items.worker()}); err != nil {
		t.Fatal(err)
	}
	app.Secrets = items.Store
	if app.Providers, err = secret.NewDiagnosticCredentialStore(items.Store); err != nil {
		t.Fatal(err)
	}
	return items
}

// worker is the program that the store runs for each operation.
func (items *keychain) worker() string {
	return filepath.Join(items.root, "bin", "aigw")
}

// hold puts an item that AIGW stored at slot, with value, which is empty for
// an item that holds no Token, without an operation of the worker.
func (items *keychain) hold(t *testing.T, slot, value string) {
	t.Helper()
	writeFile(t, filepath.Join(items.root, "keychain", "own", slot), []byte(value), 0o600)
}

// bindReader makes the worker app's own program, as the product's worker is,
// so that the credential reader that a command installs is a copy of it and
// is checked against the same items.
func (items *keychain) bindReader(app *cli.App) {
	app.Executable, app.DataDir = items.worker(), filepath.Join(items.root, "data")
	app.UseExecutableIdentity(nil, "aigw")
}

// delete does what the printed deletion step does: it removes the item that
// the command names. A command that is not the system tool's deletion of an
// item that another program stored fails the test.
func (items *keychain) delete(t *testing.T, command string) {
	t.Helper()
	words := strings.Fields(command)
	if len(words) != 6 || !slices.Equal(words[:5], []string{"security", "delete-generic-password", "-s", secret.Service, "-a"}) {
		t.Fatalf("the printed step %q is not the system tool's deletion of one item", command)
	}
	if err := os.Remove(filepath.Join(items.root, "keychain", "other", words[5])); err != nil {
		t.Fatalf("the printed step %q deletes no item that another program stored: %v", command, err)
	}
}

// operations are the operations that the worker ran so far, each as its name
// and its item, and with the word reader before it when the credential reader
// ran it.
func (items *keychain) operations(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(items.root, "keychain", "operations"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return strings.FieldsFunc(string(data), func(r rune) bool { return r == '\n' })
}

// changed are the operations among the worker's that write or delete an item.
func (items *keychain) changed(t *testing.T) []string {
	t.Helper()
	return slices.DeleteFunc(items.operations(t), func(operation string) bool {
		return !strings.HasPrefix(operation, "write ") && !strings.HasPrefix(operation, "delete ")
	})
}
