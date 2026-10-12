//go:build !windows

package configuration

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rogpeppe/go-internal/robustio"
	"go.yaml.in/yaml/v3"
)

// CreateVerificationHome borrows committed dependencies and their physical
// selection lock while keeping boot records and runtime state private.
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
	if err := borrowVerificationMembers(root, home); err != nil {
		return fail(err)
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

func verificationDependencyHomes(root string) ([]string, error) {
	homes := []string{root}
	profiles := filepath.Join(root, "profiles")
	entries, err := os.ReadDir(profiles)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.New("cannot read Hermes dependency profiles")
	}
	profileID := regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	for _, entry := range entries {
		// Native profile names and tombstones define the dependency union.
		if entry.Name() == "default" || !profileID.MatchString(entry.Name()) {
			continue
		}
		source := filepath.Join(profiles, entry.Name())
		if info, err := os.Stat(source); err == nil && info.IsDir() {
			if _, err := os.Stat(filepath.Join(profiles, ".deleted", entry.Name())); os.IsNotExist(err) {
				homes = append(homes, source)
			} else if err != nil {
				return nil, errors.New("cannot read Hermes dependency profile retirement")
			}
		} else if err != nil && !os.IsNotExist(err) {
			return nil, errors.New("cannot resolve Hermes dependency profile")
		}
	}
	return homes, nil
}

func borrowVerificationMembers(root, home string) error {
	homes, err := verificationDependencyHomes(root)
	if err != nil {
		return err
	}
	for _, source := range homes {
		data, err := os.ReadFile(filepath.Join(source, "config.yaml"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return errors.New("cannot read Hermes dependency selection")
		}
		data, err = verificationDependencySelection(data)
		if err != nil {
			return err
		}
		if len(data) == 0 {
			continue
		}
		parent := filepath.Join(home, "profiles")
		if err := os.MkdirAll(parent, 0o700); err != nil {
			return errors.New("cannot isolate Hermes dependency selections")
		}
		private := filepath.Join(parent, filepath.Base(source))
		if source == root {
			private, err = os.MkdirTemp(parent, "aigw-dependencies-")
		} else {
			err = os.Mkdir(private, 0o700)
		}
		if err != nil {
			return errors.New("cannot isolate a Hermes dependency profile")
		}
		if err := os.WriteFile(filepath.Join(private, "config.yaml"), data, 0o600); err != nil {
			return errors.New("cannot write an isolated Hermes dependency selection")
		}
		plugins := filepath.Join(source, "plugins")
		if _, err := os.Stat(plugins); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return errors.New("cannot read Hermes dependency members")
		}
		if err := os.Symlink(plugins, filepath.Join(private, "plugins")); err != nil {
			return errors.New("cannot borrow Hermes dependency member identities")
		}
	}
	return nil
}

func verificationDependencySelection(data []byte) ([]byte, error) {
	root, err := parse(data)
	if err != nil {
		return nil, errors.New("cannot decode Hermes dependency selection")
	}
	selection := mapping()
	selected := false
	for _, key := range []string{"plugins", "memory"} {
		section := field(root, key)
		if section == nil || section.Tag == "!!null" {
			continue
		}
		if section.Kind != yaml.MappingNode || section.Anchor != "" {
			return nil, errors.New("Hermes dependency selection must be an independent mapping")
		}
		fields := []string{"enabled", "disabled"}
		if key == "memory" {
			fields = []string{"provider"}
		}
		projected := mapping()
		for _, name := range fields {
			value := field(section, name)
			if value == nil || value.Tag == "!!null" {
				continue
			}
			items := value.Content
			if key == "memory" {
				items = []*yaml.Node{value}
			} else if value.Kind != yaml.SequenceNode {
				return nil, errors.New("Hermes dependency selection must contain native lists of names")
			}
			for _, item := range items {
				if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
					return nil, errors.New("Hermes dependency selection must contain native string names")
				}
			}
			if name != "disabled" && len(items) > 0 {
				selected = true
			}
			setField(projected, name, value)
		}
		setField(selection, key, projected)
	}
	if !selected {
		return nil, nil
	}
	return encode(selection)
}
