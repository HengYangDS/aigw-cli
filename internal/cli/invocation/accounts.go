package invocation

import (
	"aigw-cli/internal/domain"
	"errors"
	"fmt"
)

// ProviderReference returns the Account that an optional argument names: the
// argument itself, or the only configured Account when it is omitted. Without
// any Account, the fix is setup, which connects the first one.
func ProviderReference(cfg domain.Config, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	switch len(cfg.Providers) {
	case 0:
		return "", errors.New("no account is configured; run `aigw setup`")
	case 1:
		return cfg.CatalogProviderIDs()[0], nil
	default:
		return "", fmt.Errorf("account is required when configuration contains %d accounts", len(cfg.Providers))
	}
}
