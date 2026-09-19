package upgrade

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
	if errors.Is(err, os.ErrNotExist) {
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
	return errors.New("Homebrew manages this executable; use Homebrew for upgrade, rollback, or removal instead of portable installation commands")
}
