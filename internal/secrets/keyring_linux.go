//go:build linux

package secrets

import (
	"fmt"

	ss "github.com/zalando/go-keyring/secret_service"
)

func observeKeyringItem(service, slot string) (_ bool, resultErr error) {
	credentialService, err := ss.NewSecretService()
	if err != nil {
		return false, fmt.Errorf("connect to Secret Service: %w", err)
	}
	defer func() {
		resultErr = secretServiceCloseError(resultErr, credentialService.Conn.Close())
	}()
	items, err := credentialService.SearchItems(credentialService.GetLoginCollection(), map[string]string{
		"service":  service,
		"username": slot,
	})
	return classifySecretServiceObservation(len(items), err)
}

func classifySecretServiceObservation(matches int, searchErr error) (bool, error) {
	if searchErr != nil {
		return false, fmt.Errorf("search Secret Service item metadata: %w", searchErr)
	}
	return matches > 0, nil
}

func secretServiceCloseError(resultErr, closeErr error) error {
	if resultErr != nil || closeErr == nil {
		return resultErr
	}
	return fmt.Errorf("close Secret Service connection: %w", closeErr)
}
