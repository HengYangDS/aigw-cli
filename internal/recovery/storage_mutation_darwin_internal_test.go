//go:build darwin

package recovery

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func TestImmutableRecoveryDestinationRejectsAtomicMutation(t *testing.T) {
	for _, operation := range []string{"write", "remove", "restore"} {
		t.Run(operation, func(t *testing.T) {
			root := privateRecoveryRootForTest(t)
			path := filepath.Join(root, "air", "ledger.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			before := []byte("immutable ledger\n")
			if err := os.WriteFile(path, before, 0o600); err != nil {
				t.Fatal(err)
			}
			current, err := captureRecoveryFileNoFollow(root, path)
			if err != nil {
				t.Fatal(err)
			}
			if err := unix.Chflags(path, unix.UF_IMMUTABLE); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = unix.Chflags(path, 0) })

			switch operation {
			case "write":
				_, err = writeRecoveryFileAtomicIfUnchanged(root, path, current, []byte("replacement\n"), 0o600)
			case "remove":
				_, err = removeRecoveryFileIfUnchanged(root, path, current)
			case "restore":
				err = restoreRecoveryFileAtomicIfPostimage(root, path, transaction.FileSnapshot{}, current)
			}
			if err == nil {
				t.Fatalf("%s unexpectedly mutated an immutable destination", operation)
			}
			got, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(got, before) {
				t.Fatalf("immutable destination = %q, %v", got, readErr)
			}
		})
	}
}

func TestImmutableRecoveryParentRejectsDirectoryAndTemporaryCreation(t *testing.T) {
	t.Run("missing parent directory", func(t *testing.T) {
		root := privateRecoveryRootForTest(t)
		if err := unix.Chflags(root, unix.UF_IMMUTABLE); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = unix.Chflags(root, 0) })
		path := filepath.Join(root, "air", "ledger.json")
		if _, err := writeRecoveryFileAtomicIfUnchanged(root, path, transaction.FileSnapshot{}, []byte("ledger"), 0o600); err == nil {
			t.Fatal("created a recovery parent beneath an immutable root")
		}
	})

	t.Run("temporary file", func(t *testing.T) {
		root := privateRecoveryRootForTest(t)
		parent := filepath.Join(root, "air")
		if err := os.Mkdir(parent, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := unix.Chflags(parent, unix.UF_IMMUTABLE); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = unix.Chflags(parent, 0) })
		path := filepath.Join(parent, "ledger.json")
		if _, err := writeRecoveryFileAtomicIfUnchanged(root, path, transaction.FileSnapshot{}, []byte("ledger"), 0o600); err == nil {
			t.Fatal("created a temporary file in an immutable parent")
		}
	})
}

func TestImmutableDirectoryRejectsRecoveryRootCreation(t *testing.T) {
	parent := privateRecoveryRootForTest(t)
	if err := unix.Chflags(parent, unix.UF_IMMUTABLE); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Chflags(parent, 0) })
	root, err := openRecoveryRootNoFollow(filepath.Join(parent, "child"), true, true)
	if root != nil {
		_ = root.Close()
	}
	if err == nil {
		t.Fatal("created a recovery root beneath an immutable directory")
	}
}

func TestCanonicalRecoveryRootPathResolvesPrivateAliases(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{name: "var alias", path: "/var", want: "/private/var"},
		{name: "var subtree", path: "/var/folders/example", want: "/private/var/folders/example"},
		{name: "tmp alias", path: "/tmp", want: "/private/tmp"},
		{name: "tmp subtree", path: "/tmp/aigw-recovery", want: "/private/tmp/aigw-recovery"},
		{name: "etc alias", path: "/etc", want: "/private/etc"},
		{name: "etc subtree", path: "/etc/aigw", want: "/private/etc/aigw"},
		{name: "non alias", path: "/Users/example/recovery", want: "/Users/example/recovery"},
		{name: "alias-prefixed non alias", path: "/varnish/recovery", want: "/varnish/recovery"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := canonicalRecoveryRootPath(testCase.path); got != testCase.want {
				t.Fatalf("canonicalRecoveryRootPath(%q) = %q, want %q", testCase.path, got, testCase.want)
			}
		})
	}
}

func TestOpenRecoveryRootReportsFileDescriptorExhaustion(t *testing.T) {
	var original unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &original); err != nil {
		t.Fatal(err)
	}
	limited := original
	limited.Cur = 0
	if err := unix.Setrlimit(unix.RLIMIT_NOFILE, &limited); err != nil {
		t.Fatal(err)
	}
	_, openErr := openRecoveryRootNoFollow("/", false, false)
	if err := unix.Setrlimit(unix.RLIMIT_NOFILE, &original); err != nil {
		t.Fatalf("restore file descriptor limit: %v", err)
	}
	if openErr == nil {
		t.Fatal("opened the filesystem root with no available file descriptors")
	}
}
