package secrets

import (
	"errors"
	"fmt"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

const Service = "AIGW_TOKEN"

var (
	ErrNotFound = errors.New("secret not found")
	ErrReadOnly = errors.New("secret store is read-only")
)

type Store interface {
	Get(profile string) (string, error)
	Set(profile, value string) error
	Delete(profile string) error
	Has(profile string) bool
}

func validate(profile, value string, requireValue bool) error {
	if !domain.ValidProfileName(profile) {
		return fmt.Errorf("invalid profile name %q", profile)
	}
	if requireValue && value == "" {
		return errors.New("empty secret refused")
	}
	return nil
}
