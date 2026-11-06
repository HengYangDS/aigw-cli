package invocation

import (
	"aigw-cli/internal/domain"
	"fmt"
)

// ProviderReference returns the Account that an optional argument names: the
// argument itself, or the only configured Account when it is omitted.
func ProviderReference(cfg domain.Config, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	if len(cfg.Providers) != 1 {
		return "", fmt.Errorf("account is required when configuration contains %d accounts", len(cfg.Providers))
	}
	for accountID := range cfg.Providers {
		return accountID, nil
	}
	return "", fmt.Errorf("no account is configured")
}
