//go:build !windows

package configuration

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/rogpeppe/go-internal/robustio"
)

// CreateVerificationHome borrows committed dependencies, not their mutable boot
// state. Native dependency leases remain Hermes-owned and expire on process exit.
// Windows has no implementation: installed Hermes republishes its launchers from
// any home there, so callers refuse verification before starting the client.
func CreateVerificationHome(target string) (string, func() error, error) {
	root := filepath.Dir(target)
	if filepath.Base(filepath.Dir(root)) == "profiles" {
		root = filepath.Dir(filepath.Dir(root))
	}
	home, err := os.MkdirTemp("", "aigw-hermes-verify-")
	if err != nil {
		return "", nil, errors.New("cannot create an isolated Hermes verification home")
	}
	cleanup := func() error { return robustio.RemoveAll(home) }
	fail := func(err error) (string, func() error, error) {
		return "", nil, errors.Join(err, cleanup())
	}
	physicalHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return fail(errors.New("cannot resolve the isolated Hermes verification home"))
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return fail(errors.New("cannot resolve the native Hermes home for verification"))
	}
	for _, nativeRoot := range []string{root, filepath.Join(userHome, ".hermes")} {
		physicalRoot, err := filepath.EvalSymlinks(nativeRoot)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fail(errors.New("cannot resolve the native Hermes data root"))
		}
		if relative, err := filepath.Rel(physicalRoot, physicalHome); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return fail(errors.New("Hermes verification requires temporary storage outside its native data root"))
		}
	}
	for _, name := range []string{"tools", "installs"} {
		source := filepath.Join(root, name)
		if _, err := os.Stat(source); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return fail(errors.New("cannot read the installed Hermes dependency state"))
		}
		if name == "tools" {
			if err := os.Symlink(source, filepath.Join(home, name)); err != nil {
				return fail(errors.New("cannot reuse the installed Hermes tools without provisioning"))
			}
			continue
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return fail(errors.New("cannot read the installed Hermes dependency selections"))
		}
		for _, entry := range entries {
			installed := filepath.Join(source, entry.Name())
			info, err := os.Stat(installed)
			if err != nil {
				return fail(errors.New("cannot resolve the installed Hermes dependency selection"))
			}
			if !info.IsDir() {
				continue
			}
			if err := borrowVerificationInstall(installed, filepath.Join(home, "installs", entry.Name())); err != nil {
				return fail(err)
			}
		}
	}
	return home, cleanup, nil
}

func borrowVerificationInstall(installed, private string) error {
	if _, err := os.Stat(filepath.Join(installed, "facts.json")); err == nil {
		if lock, err := os.Stat(filepath.Join(installed, ".install.lock")); err != nil || !lock.Mode().IsRegular() {
			return errors.New("Hermes dependency selection lock is unavailable; restore the native installation before verification")
		}
	}
	if err := os.MkdirAll(private, 0o700); err != nil {
		return errors.New("cannot isolate Hermes boot maintenance")
	}
	for _, dependency := range []string{"facts.json", "environments", "pm-runtime", ".install.lock"} {
		borrowed := filepath.Join(installed, dependency)
		if _, err := os.Stat(borrowed); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return errors.New("cannot read a committed Hermes dependency")
		}
		if err := os.Symlink(borrowed, filepath.Join(private, dependency)); err != nil {
			return errors.New("cannot reuse a committed Hermes dependency without provisioning")
		}
	}
	return nil
}
