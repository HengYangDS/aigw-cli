package program

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aigw-cli/internal/platform"
)

// ErrHomebrewManaged means a valid Homebrew receipt owns the executable, which
// Homebrew upgrades and removes; its next action is the Homebrew command.
var ErrHomebrewManaged error = homebrewManagedError{}

// homebrewManagedError is ErrHomebrewManaged.
type homebrewManagedError struct{}

func (homebrewManagedError) Error() string {
	return "Homebrew manages this executable; upgrade, roll back or remove it with Homebrew, such as `brew upgrade --cask aigw`, instead of portable installation commands, and run `aigw update --rollback` before Homebrew installs an earlier release"
}

// NextAction is the Homebrew command that keeps the program current.
func (homebrewManagedError) NextAction() string { return "brew upgrade --cask aigw" }

// InstalledProgram reports whether an installation rolls back through
// executable: Homebrew's receipt owns it, or AIGW retained the portable
// predecessor beside it. A versioned reader copy, an extracted release and a
// development build have neither, so they cannot tell which program their
// installation runs.
func InstalledProgram(executable string) bool {
	if strings.TrimSpace(executable) == "" {
		return false
	}
	if errors.Is(RequirePortableOwnership(executable), ErrHomebrewManaged) {
		return true
	}
	predecessor, err := os.Lstat(RollbackPath(executable))
	return err == nil && predecessor.Mode().IsRegular()
}

// RequirePortableOwnership stops product-managed writes to a Homebrew installation.
// Native receipts, rather than a host prefix or environment hint, establish ownership.
func RequirePortableOwnership(executable string) error {
	if strings.TrimSpace(executable) == "" {
		return nil
	}
	absolute, err := filepath.Abs(executable)
	if err != nil {
		return fmt.Errorf("resolve installation ownership: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("resolve installation ownership: %w", err)
		}
		resolved, err = resolveExistingParent(absolute)
		if err != nil {
			return err
		}
	}
	for directory := filepath.Dir(resolved); ; directory = filepath.Dir(directory) {
		parent := filepath.Dir(directory)
		var receipt string
		switch filepath.Base(parent) {
		case "Cellar":
			relative, err := filepath.Rel(directory, resolved)
			if err != nil {
				return err
			}
			version, _, found := strings.Cut(relative, string(filepath.Separator))
			if found {
				receipt = filepath.Join(directory, version, "INSTALL_RECEIPT.json")
			}
		case "Caskroom":
			receipt = filepath.Join(directory, ".metadata", "INSTALL_RECEIPT.json")
		}
		if receipt != "" {
			if err := checkHomebrewReceipt(receipt); err != nil {
				return err
			}
		}
		if parent == directory {
			return nil
		}
	}
}

func resolveExistingParent(path string) (string, error) {
	parent := filepath.Dir(path)
	if parent == path {
		return path, nil
	}
	resolved, err := filepath.EvalSymlinks(parent)
	if absent, refusal := platform.MissingAt(parent, err); refusal != nil {
		return "", fmt.Errorf("resolve installation parent: %w", refusal)
	} else if absent {
		resolved, err = resolveExistingParent(parent)
	}
	if err != nil {
		return "", fmt.Errorf("resolve installation parent: %w", err)
	}
	return filepath.Join(resolved, filepath.Base(path)), nil
}

func checkHomebrewReceipt(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read Homebrew installation receipt: %w", err)
	}
	var receipt struct {
		Version string `json:"homebrew_version"`
		Source  struct {
			Tap string `json:"tap"`
		} `json:"source"`
	}
	if err := json.Unmarshal(data, &receipt); err != nil {
		return fmt.Errorf("invalid Homebrew installation receipt; repair the installation through Homebrew: %w", err)
	}
	if receipt.Version == "" || receipt.Source.Tap == "" {
		return errors.New("incomplete Homebrew installation receipt; repair the installation through Homebrew")
	}
	return ErrHomebrewManaged
}
