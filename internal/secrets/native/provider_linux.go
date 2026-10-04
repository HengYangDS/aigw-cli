//go:build linux

package native

import (
	"errors"
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

func writeCredential(service, account string, value []byte) (resultErr error) {
	credentialService, err := ss.NewSecretService()
	if err != nil {
		return fmt.Errorf("connect to Secret Service: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, credentialService.Conn.Close()) }()
	session, err := credentialService.OpenSession()
	if err != nil {
		return fmt.Errorf("open Secret Service session: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, credentialService.Close(session)) }()
	collection := credentialService.GetLoginCollection()
	if err := credentialService.Unlock(collection.Path()); err != nil {
		return fmt.Errorf("unlock Secret Service collection: %w", err)
	}
	attributes := map[string]string{"service": service, "username": account}
	items, err := credentialService.SearchItems(collection, attributes)
	if err != nil {
		return fmt.Errorf("search Secret Service item metadata: %w", err)
	}
	secret := ss.NewSecret(session.Path(), string(value))
	switch len(items) {
	case 0:
		return credentialService.CreateItem(collection, fmt.Sprintf("Password for '%s' on '%s'", account, service), attributes, secret)
	case 1:
		if err := credentialService.Unlock(items[0]); err != nil {
			return fmt.Errorf("unlock Secret Service item: %w", err)
		}
		return credentialService.Object("org.freedesktop.secrets", items[0]).Call("org.freedesktop.Secret.Item.SetSecret", 0, secret).Err
	default:
		return errors.New("Secret Service credential identity is ambiguous")
	}
}
