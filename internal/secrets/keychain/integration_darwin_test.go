//go:build darwin && keychain_integration

package keychain

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ebitengine/purego"
)

func TestNativeAbsentReadUsesTheRealBridgeAndBoundedWorker(t *testing.T) {
	if os.Getenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE") != "ephemeral-host" {
		t.Fatal("Keychain integration requires an explicitly admitted ephemeral-host")
	}
	// Only a unique absent item is queried; this process disables its own UI.
	account := "aigw-absent-" + filepath.Base(t.TempDir())
	if value, err := queryNative(workerCommand, "AIGW_TOKEN", account, nil); value != nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("native missing item classification: %v", err)
	}
	if value, err := Read("AIGW_TOKEN", account); value != "" || !errors.Is(err, ErrNotFound) {
		t.Fatalf("bounded native missing item classification: %v", err)
	}
	if exists, err := Exists("AIGW_TOKEN", account); exists || err != nil {
		t.Fatalf("bounded absent metadata: %v", err)
	}
}

func TestNativePrivateKeychainReadAndLockedFailure(t *testing.T) {
	if os.Getenv("AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE") != "ephemeral-host" {
		t.Fatal("Keychain integration requires an explicitly admitted ephemeral-host")
	}
	if root := os.Getenv("AIGW_TEST_PRIVATE_KEYCHAIN"); root != "" {
		if expectation := os.Getenv("AIGW_TEST_PRIVATE_READER"); expectation != "" {
			testPrivateKeychainReader(t, root, expectation)
			return
		}
		testPrivateKeychain(t, root)
		return
	}
	before, err := exec.Command("/usr/bin/security", "list-keychains", "-d", "user").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, identity := range []string{"ad-hoc", "certificate"} {
		t.Run(identity, func(t *testing.T) {
			// Security.framework selects partitioned Keychains by this path shape.
			// The entire path remains private; it is not the operator's HOME.
			root, program := filepath.Join(t.TempDir(), "Library", "Keychains"), os.Args[0]
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			environment := append(os.Environ(), "AIGW_TEST_PRIVATE_KEYCHAIN="+root)
			if identity == "certificate" {
				program = testCertificateReader(t, root)
				environment = append(environment, "AIGW_TEST_CERTIFICATE_ROOT="+root)
			}
			// Each isolated journey includes instrumented child startup and exit;
			// this budget is independent of the production worker's deadline.
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, program, "-test.run=^TestNativePrivateKeychainReadAndLockedFailure$")
			command.Env = environment
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("isolated native contract: %v\n%s", err, output)
			}
		})
	}
	after, err := exec.Command("/usr/bin/security", "list-keychains", "-d", "user").Output()
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("Keychain search list changed: %v", err)
	}
}

func testCertificateReader(t *testing.T, root string) string {
	t.Helper()
	testSigningCommand(t, "rcodesign", "-C", "/dev/null", "generate-self-signed-certificate",
		"--person-name", "aigw.private-keychain-fixture", "--validity-days", "1", "--pem-filename", filepath.Join(root, "identity"))
	fingerprint := string(testSigningCommand(t, "/usr/bin/openssl", "x509", "-in", filepath.Join(root, "identity.crt"), "-noout", "-fingerprint", "-sha1"))
	_, fingerprint, found := strings.Cut(fingerprint, "=")
	if !found {
		t.Fatal("synthetic certificate fingerprint absent")
	}
	fingerprint = strings.ReplaceAll(strings.TrimSpace(fingerprint), ":", "")
	requirement := `certificate leaf = H"` + fingerprint + `" and identifier "aigw.private-keychain-fixture"`
	testSigningCommand(t, "/usr/bin/csreq", "-r", "="+requirement, "-b", filepath.Join(root, "requirement.bin"))
	program, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "predecessor")
	if err := os.WriteFile(path, program, 0o700); err != nil {
		t.Fatal(err)
	}
	testSignPrivateReader(t, root, path, nil)
	return path
}

func testSignPrivateReader(t *testing.T, root, path string, options []string) {
	t.Helper()
	args := []string{"-C", "/dev/null", "sign", "--pem-file", filepath.Join(root, "identity.crt"),
		"--pem-file", filepath.Join(root, "identity.key"), "--binary-identifier", "aigw.private-keychain-fixture",
		"--code-requirements-file", filepath.Join(root, "requirement.bin"), "--timestamp-url", "none"}
	args = append(args, options...)
	testSigningCommand(t, "rcodesign", append(args, path)...)
	testSigningCommand(t, "/usr/bin/codesign", "--verify", "--strict", "--test-requirement", filepath.Join(root, "requirement.bin"), path)
}

func testSigningCommand(t *testing.T, executable string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("private signer %s: %v; context=%v\n%s", executable, err, ctx.Err(), output)
	}
	return output
}

func testPrivateKeychain(t *testing.T, root string) {
	t.Helper()
	api, closeLibrary, err := loadNative()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := closeLibrary(); err != nil {
			t.Error(err)
		}
	})
	if code := api.interaction(false); code != 0 {
		t.Fatalf("disable UI: %d", code)
	}
	lib, err := purego.Dlopen(securityFramework, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := purego.Dlclose(lib); err != nil {
			t.Error(err)
		}
	})
	var create func(string, uint32, string, bool, uintptr, *uintptr) int32
	var remove func(uintptr) int32
	var lock func(uintptr) int32
	var add func(uintptr, uint32, string, uint32, string, uint32, string, *uintptr) int32
	purego.RegisterLibFunc(&create, lib, "SecKeychainCreate")
	purego.RegisterLibFunc(&remove, lib, "SecKeychainDelete")
	purego.RegisterLibFunc(&lock, lib, "SecKeychainLock")
	purego.RegisterLibFunc(&add, lib, "SecKeychainAddGenericPassword")
	var chain uintptr
	const password = "synthetic-keychain-password"
	if code := create(filepath.Join(root, "fixture.keychain"), uint32(len(password)), password, false, 0, &chain); code != 0 {
		t.Fatalf("create private Keychain: %d", code)
	}
	t.Cleanup(func() {
		defer api.release(chain)
		if code := remove(chain); code != 0 {
			t.Errorf("delete private Keychain: %d", code)
		}
	})
	const value = "go-keyring-base64:c3ludGhldGlj"
	if code := add(chain, 7, "service", 7, "account", uint32(len(value)), value, nil); code != 0 {
		t.Fatalf("create synthetic item: %d", code)
	}
	testRequirePartitionedKeychain(t, lib, chain, root)
	if got, err := api.read(chain, "service", "account", false); err != nil || string(got) != value {
		t.Fatalf("synthetic exact item read: %v", err)
	}
	testPrivateReaderExecutableIdentity(t, root)
	if got, err := api.read(chain, "service", "absent", false); got != nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing item: %v", err)
	}
	testNativeCredentialMutation(t, api, chain)
	command := exec.Command("/usr/bin/security", "-i")
	command.Stdin = strings.NewReader("add-generic-password -s service -a security-writer -w synthetic-fixture '" + filepath.Join(root, "fixture.keychain") + "'\n")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create security-owned private fixture: %v; output bytes=%d", err, len(output))
	}
	if got, err := api.read(chain, "service", "security-writer", true); err != nil || got != nil {
		t.Fatalf("security-owned metadata: %v", err)
	}
	if got, err := api.read(chain, "service", "security-writer", false); got != nil || !errors.Is(err, ErrDenied) {
		t.Fatalf("foreign writer authorization boundary: %v", err)
	}
	if code := lock(chain); code != 0 {
		t.Fatalf("lock private Keychain: %d", code)
	}
	if got, err := api.read(chain, "service", "account", false); got != nil || !errors.Is(err, ErrDenied) {
		t.Fatalf("locked item: %v", err)
	}
	if err := api.mutate(chain, writeCommand, "service", "account", []byte("replacement")); !errors.Is(err, ErrDenied) {
		t.Fatalf("locked write: %v", err)
	}
	// File-based Keychain deletion has its own authorization: this owned item
	// can be removed while locked without unlocking or reading its password.
	if err := api.mutate(chain, deleteCommand, "service", "account", nil); err != nil {
		t.Fatalf("owned locked-item deletion: %v", err)
	}
	if _, err := api.read(chain, "service", "account", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted locked item remains: %v", err)
	}
}

func testRequirePartitionedKeychain(t *testing.T, library, chain uintptr, root string) {
	t.Helper()
	var getVersion func(uintptr, *uint32) int32
	purego.RegisterLibFunc(&getVersion, library, "SecKeychainGetKeychainVersion")
	var version uint32
	if code := getVersion(chain, &version); code != 0 {
		t.Fatalf("inspect private Keychain version: %d", code)
	}
	if version != 0x200 {
		t.Fatalf("native credential fixture must enforce partition ACLs: %#x", version)
	}
	acl := testSigningCommand(t, "/usr/bin/security", "dump-keychain", "-a", filepath.Join(root, "fixture.keychain"))
	if !bytes.Contains(acl, []byte("partition_id")) || !bytes.Contains(acl, []byte("cdhash:")) {
		t.Fatal("private item lacks its code-hash partition boundary")
	}
}

func testPrivateKeychainReader(t *testing.T, root, expectation string) {
	t.Helper()
	api, closeLibrary, err := loadNative()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := closeLibrary(); err != nil {
			t.Error(err)
		}
	})
	if code := api.interaction(false); code != 0 {
		t.Fatalf("disable reader interaction: %d", code)
	}
	lib, err := purego.Dlopen(securityFramework, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := purego.Dlclose(lib); err != nil {
			t.Error(err)
		}
	})
	var open func(string, *uintptr) int32
	purego.RegisterLibFunc(&open, lib, "SecKeychainOpen")
	var chain uintptr
	if code := open(filepath.Join(root, "fixture.keychain"), &chain); code != 0 {
		t.Fatalf("open private Keychain: %d", code)
	}
	defer api.release(chain)
	value, err := api.read(chain, "service", "account", false)
	defer clear(value)
	switch expectation {
	case "authorized":
		if err != nil || string(value) != "go-keyring-base64:c3ludGhldGlj" {
			t.Fatalf("same-image subprocess read: %v", err)
		}
	case "denied":
		if value != nil || !errors.Is(err, ErrDenied) {
			t.Fatalf("different-image authorization: %v", err)
		}
	default:
		t.Fatal("unknown reader expectation")
	}
}

func testPrivateReaderExecutableIdentity(t *testing.T, root string) {
	t.Helper()
	copyPath := filepath.Join(root, "successor-reader")
	program, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, program, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, program := range []string{os.Args[0], copyPath} {
		testPrivateReaderProcess(t, program, root, "authorized")
	}
	if certificate := os.Getenv("AIGW_TEST_CERTIFICATE_ROOT"); certificate != "" {
		before := testSigningCommand(t, "/usr/bin/codesign", "--display", "--verbose=4", copyPath)
		testSignPrivateReader(t, certificate, copyPath, []string{"--code-signature-flags", "kill"})
		after := testSigningCommand(t, "/usr/bin/codesign", "--display", "--verbose=4", copyPath)
		_, original, found := strings.Cut(string(before), "CDHash=")
		original, _, _ = strings.Cut(original, "\n")
		if !found || original == "" || strings.Contains(string(after), "CDHash="+original+"\n") {
			t.Fatal("certificate successor must have a different code hash")
		}
		// A self-signed designated requirement can match while the independent
		// code-hash partition still denies a changed image without interaction.
		testPrivateReaderProcess(t, copyPath, root, "denied")
	}
	metadata, err := exec.Command("/usr/bin/codesign", "--display", "--verbose=2", copyPath).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect disposable code identity: %v", err)
	}
	_, identifier, found := strings.Cut(string(metadata), "Identifier=")
	identifier, _, _ = strings.Cut(identifier, "\n")
	if !found || identifier == "" {
		t.Fatal("disposable code has no signing identifier")
	}
	// Only this disposable copy is re-signed. No certificate, user keychain,
	// production executable or item access policy is changed. Both the path
	// and identifier stay the same; code-hash identity alone changes.
	command := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", "--identifier", identifier, copyPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("different disposable code identity: %v\n%s", err, output)
	}
	testPrivateReaderProcess(t, copyPath, root, "denied")
}

func testPrivateReaderProcess(t *testing.T, program, root, expectation string) {
	t.Helper()
	// Allow race/coverage startup and exit bookkeeping around the native call.
	// The product worker still has its independent five-second deadline.
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, program, "-test.run=^TestNativePrivateKeychainReadAndLockedFailure$")
	command.Env = append(os.Environ(), "AIGW_TEST_PRIVATE_KEYCHAIN="+root, "AIGW_TEST_PRIVATE_READER="+expectation)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("private reader process (%s, %s): %v; context=%v\n%s", filepath.Base(program), expectation, err, ctx.Err(), output)
	}
}

func testNativeCredentialMutation(t *testing.T, api nativeAPI, chain uintptr) {
	t.Helper()
	for _, token := range []string{"created-token", "rotated-token"} {
		if err := api.mutate(chain, writeCommand, "service", "native-writer", []byte(token)); err != nil {
			t.Fatalf("native writer: %v", err)
		}
		if got, err := api.read(chain, "service", "native-writer", false); err != nil || string(got) != token {
			t.Fatalf("same-identity read after write: %v", err)
		}
	}
	for range 2 {
		if err := api.mutate(chain, deleteCommand, "service", "native-writer", nil); err != nil {
			t.Fatalf("native delete: %v", err)
		}
	}
	if _, err := api.read(chain, "service", "native-writer", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted native item remains: %v", err)
	}
}
