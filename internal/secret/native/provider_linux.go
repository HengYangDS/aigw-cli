//go:build linux

package native

import (
	"errors"
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
	ss "github.com/zalando/go-keyring/secret_service"
)

const (
	secretServiceName         = "org.freedesktop.secrets"
	secretCollectionInterface = "org.freedesktop.Secret.Collection"
	secretItemInterface       = "org.freedesktop.Secret.Item"
	secretPromptInterface     = "org.freedesktop.Secret.Prompt"
)

// queryCredential serves every program role alike: Secret Service authorizes
// the user's session, not the reading executable, so no item changes owner.
func queryCredential(operation, service, account string, input []byte, _ Role) (_ []byte, resultErr error) {
	switch operation {
	case readCommand, writeCommand, deleteCommand, existsCommand, listCommand:
	default:
		return nil, ErrUnavailable
	}
	credentialService, err := ss.NewSecretService()
	if err != nil {
		return nil, fmt.Errorf("connect to Secret Service: %w", err)
	}
	// The library borrows the process-wide D-Bus connection; this call only owns
	// the Secret Service session created below.
	collection := credentialService.GetLoginCollection()
	if operation == listCommand {
		return listSecretServiceAccounts(credentialService, collection, service)
	}
	attributes := map[string]string{"service": service, "username": account}
	items, err := credentialService.SearchItems(collection, attributes)
	if err != nil {
		return nil, fmt.Errorf("search Secret Service item metadata: %w", err)
	}
	if len(items) > 1 {
		return nil, errors.New("Secret Service credential identity is ambiguous")
	}
	if operation == existsCommand {
		if len(items) == 0 {
			return []byte("0"), nil
		}
		return []byte("1"), nil
	}
	if len(items) == 0 && operation != writeCommand {
		if operation == readCommand {
			return nil, ErrNotFound
		}
		return nil, nil
	}
	if err := requireUnlockedSecretServiceObject(collection, secretCollectionInterface); err != nil {
		return nil, err
	}
	if len(items) == 1 {
		item := credentialService.Object(secretServiceName, items[0])
		if err := requireUnlockedSecretServiceObject(item, secretItemInterface); err != nil {
			return nil, err
		}
	}
	if operation == deleteCommand {
		var prompt dbus.ObjectPath
		if err := credentialService.Object(secretServiceName, items[0]).Call(secretItemInterface+".Delete", 0).Store(&prompt); err != nil {
			return nil, err
		}
		return nil, refuseSecretServicePrompt(credentialService.Object(secretServiceName, prompt))
	}
	session, err := credentialService.OpenSession()
	if err != nil {
		return nil, fmt.Errorf("open Secret Service session: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, credentialService.Close(session)) }()
	if operation == readCommand {
		secret, err := credentialService.GetSecret(items[0], session.Path())
		if err != nil {
			return nil, err
		}
		return secret.Value, nil
	}
	secret := ss.NewSecret(session.Path(), string(input))
	if len(items) == 1 {
		return nil, credentialService.Object(secretServiceName, items[0]).Call(secretItemInterface+".SetSecret", 0, secret).Err
	}
	properties := map[string]dbus.Variant{
		secretItemInterface + ".Label":      dbus.MakeVariant(fmt.Sprintf("Password for '%s' on '%s'", account, service)),
		secretItemInterface + ".Attributes": dbus.MakeVariant(attributes),
	}
	var item, prompt dbus.ObjectPath
	if err := collection.Call(secretCollectionInterface+".CreateItem", 0, properties, secret, false).Store(&item, &prompt); err != nil {
		return nil, err
	}
	return nil, refuseSecretServicePrompt(credentialService.Object(secretServiceName, prompt))
}

// secretServiceListing is what listing reads from a Secret Service: the
// collection's lock state, its items for a service, and an item's attributes.
type secretServiceListing struct {
	requireUnlocked func() error
	search          func(service string) ([]dbus.ObjectPath, error)
	attributes      func(item dbus.ObjectPath) (dbus.Variant, error)
}

// listSecretServiceAccounts names a service's accounts from each item's
// attributes. It opens no session, so no secret can be transferred.
func listSecretServiceAccounts(credentialService *ss.SecretService, collection dbus.BusObject, service string) ([]byte, error) {
	return listAccounts(secretServiceListing{
		requireUnlocked: func() error { return requireUnlockedSecretServiceObject(collection, secretCollectionInterface) },
		search: func(service string) ([]dbus.ObjectPath, error) {
			return credentialService.SearchItems(collection, map[string]string{"service": service})
		},
		attributes: func(item dbus.ObjectPath) (dbus.Variant, error) {
			return credentialService.Object(secretServiceName, item).GetProperty(secretItemInterface + ".Attributes")
		},
	}, service)
}

// listAccounts refuses a locked collection before it searches, rather than
// ask to unlock it: a locked store can return no items, which would read as
// nothing stored when the truth is that it cannot tell.
func listAccounts(store secretServiceListing, service string) ([]byte, error) {
	if err := store.requireUnlocked(); err != nil {
		return nil, err
	}
	items, err := store.search(service)
	if err != nil {
		return nil, fmt.Errorf("search Secret Service item metadata: %w", err)
	}
	if len(items) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		property, err := store.attributes(item)
		if err != nil {
			return nil, fmt.Errorf("read Secret Service item attributes: %w", err)
		}
		attributes, ok := property.Value().(map[string]string)
		if !ok {
			return nil, ErrUnavailable
		}
		names = append(names, attributes["username"])
	}
	return []byte(strings.Join(names, "\n")), nil
}

func requireUnlockedSecretServiceObject(object dbus.BusObject, interfaceName string) error {
	locked, err := object.GetProperty(interfaceName + ".Locked")
	if err != nil {
		return err
	}
	if locked.Value() != false {
		return ErrUnavailable
	}
	return nil
}

func refuseSecretServicePrompt(prompt dbus.BusObject) error {
	if prompt.Path() == "/" {
		return nil
	}
	return errors.Join(ErrUnavailable, prompt.Call(secretPromptInterface+".Dismiss", 0).Err)
}

func nativeEnvironment(getenv func(string) string) []string {
	return retainedEnvironment(getenv, "HOME", "DBUS_SESSION_BUS_ADDRESS", "XDG_RUNTIME_DIR")
}
