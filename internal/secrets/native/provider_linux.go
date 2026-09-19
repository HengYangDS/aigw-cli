//go:build linux

package native

import (
	"fmt"

	ss "github.com/zalando/go-keyring/secret_service"
)

func observeCredential(service, account string) (_ bool, resultErr error) {
	credentialService, err := ss.NewSecretService()
	if err != nil {
		return false, fmt.Errorf("connect to Secret Service: %w", err)
	}
	defer func() {
		closeErr := credentialService.Conn.Close()
		if resultErr == nil && closeErr != nil {
			resultErr = fmt.Errorf("close Secret Service connection: %w", closeErr)
		}
	}()
	items, err := credentialService.SearchItems(credentialService.GetLoginCollection(), map[string]string{
		"service":  service,
		"username": account,
	})
	if err != nil {
		return false, fmt.Errorf("search Secret Service item metadata: %w", err)
	}
	return len(items) > 0, nil
}

func nativeEnvironment(getenv func(string) string) []string {
	return retainedEnvironment(getenv, "HOME", "DBUS_SESSION_BUS_ADDRESS", "XDG_RUNTIME_DIR")
}
