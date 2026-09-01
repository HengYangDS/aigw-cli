//go:build linux

package secrets

import (
	"fmt"

	ss "github.com/zalando/go-keyring/secret_service"
)

func observeKeyringItem(service, slot string) (bool, error) {
	credentialService, err := ss.NewSecretService()
	if err != nil {
		return false, fmt.Errorf("connect to Secret Service: %w", err)
	}
	defer credentialService.Conn.Close()
	items, err := credentialService.SearchItems(credentialService.GetLoginCollection(), map[string]string{
		"service":  service,
		"username": slot,
	})
	return classifySecretServiceObservation(len(items), err)
}

func classifySecretServiceObservation(matches int, err error) (bool, error) {
	if err != nil {
		return false, fmt.Errorf("search Secret Service item metadata: %w", err)
	}
	return matches > 0, nil
}
