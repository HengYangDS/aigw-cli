//go:build darwin

package keychain

import (
	"errors"
	"math"
	"unsafe"

	"github.com/ebitengine/purego"
)

const securityFramework = "/System/Library/Frameworks/Security.framework/Security"

type nativeAPI struct {
	interaction func(bool) int32
	find        func(uintptr, uint32, string, uint32, string, *uint32, *unsafe.Pointer, *uintptr) int32
	free        func(uintptr, unsafe.Pointer) int32
	add         func(uintptr, uint32, string, uint32, string, uint32, unsafe.Pointer, *uintptr) int32
	modify      func(uintptr, uintptr, uint32, unsafe.Pointer) int32
	remove      func(uintptr) int32
	release     func(uintptr)
}

func (api nativeAPI) mutate(keychain uintptr, operation, service, account string, value []byte) error {
	if operation != writeCommand && operation != deleteCommand || len(value) > maxStoredValue {
		return ErrUnavailable
	}
	if err := api.prepare(service, account); err != nil {
		return err
	}
	var item uintptr
	// #nosec G115 -- prepare validates both byte lengths before this native call.
	status := api.find(keychain, uint32(len(service)), service, uint32(len(account)), account, nil, nil, &item)
	if item != 0 {
		defer api.release(item)
	}
	if operation == deleteCommand {
		if status == -25300 {
			return nil
		}
		if err := nativeStatus(status); err != nil {
			return err
		}
		return nativeStatus(api.remove(item))
	}
	// #nosec G103 -- The synchronous native call borrows this bounded Go-owned buffer.
	data := unsafe.Pointer(unsafe.SliceData(value))
	if status == -25300 {
		// #nosec G115 -- prepare and maxStoredValue bound every length to UInt32.
		return nativeStatus(api.add(keychain, uint32(len(service)), service, uint32(len(account)), account, uint32(len(value)), data, nil))
	}
	if err := nativeStatus(status); err != nil {
		return err
	}
	// Update the existing item, preserving its ownership and access policy.
	// #nosec G115 -- maxStoredValue bounds the native byte count.
	return nativeStatus(api.modify(item, 0, uint32(len(value)), data))
}

func loadNative() (nativeAPI, func() error, error) {
	lib, err := purego.Dlopen(securityFramework, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nativeAPI{}, nil, ErrUnavailable
	}
	closeLibrary := func() error { return purego.Dlclose(lib) }
	var api nativeAPI
	for _, binding := range []struct {
		name   string
		target any
	}{
		{"SecKeychainSetUserInteractionAllowed", &api.interaction},
		{"SecKeychainFindGenericPassword", &api.find},
		{"SecKeychainItemFreeContent", &api.free},
		{"SecKeychainAddGenericPassword", &api.add},
		{"SecKeychainItemModifyAttributesAndData", &api.modify},
		{"SecKeychainItemDelete", &api.remove},
		{"CFRelease", &api.release},
	} {
		address, err := purego.Dlsym(lib, binding.name)
		if err != nil {
			return nativeAPI{}, nil, errors.Join(ErrUnavailable, closeLibrary())
		}
		purego.RegisterFunc(binding.target, address)
	}
	return api, closeLibrary, nil
}

func queryNative(operation, service, account string, input []byte) (value []byte, err error) {
	api, closeLibrary, err := loadNative()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, closeLibrary()) }()
	switch operation {
	case workerCommand, observeCommand:
		return api.read(0, service, account, operation == observeCommand)
	case writeCommand, deleteCommand:
		return nil, api.mutate(0, operation, service, account, input)
	default:
		return nil, ErrUnavailable
	}
}

func (api nativeAPI) prepare(service, account string) error {
	if len(service) > math.MaxUint32 || len(account) > math.MaxUint32 {
		return ErrUnavailable
	}
	// These legacy APIs address the existing file-based Keychain used by
	// go-keyring. UI suppression is process-wide, so only the isolated worker
	// uses it; the parent and other credential operations are unaffected.
	if api.interaction(false) != 0 {
		return ErrUnavailable
	}
	return nil
}

func (api nativeAPI) read(keychain uintptr, service, account string, observe bool) ([]byte, error) {
	if err := api.prepare(service, account); err != nil {
		return nil, err
	}
	var length uint32
	var data unsafe.Pointer
	sizeTarget, dataTarget := &length, &data
	if observe {
		sizeTarget, dataTarget = nil, nil
	}
	// #nosec G115 -- Both byte lengths are checked against MaxUint32 before this native call.
	status := api.find(keychain, uint32(len(service)), service, uint32(len(account)), account, sizeTarget, dataTarget, nil)
	if err := nativeStatus(status); err != nil {
		return nil, err
	}
	if observe {
		return nil, nil
	}
	var value []byte
	if data != nil && length <= maxStoredValue {
		// #nosec G103 -- Security.framework owns this exact-length buffer until FreeContent below.
		value = append([]byte(nil), unsafe.Slice((*byte)(data), length)...)
	}
	if api.free(0, data) != 0 || length > maxStoredValue || data == nil && length != 0 {
		clear(value)
		return nil, ErrUnavailable
	}
	return value, nil
}

func nativeStatus(status int32) error {
	switch status {
	case 0:
		return nil
	case -25300:
		return ErrNotFound
	case -25308, -25293:
		return ErrDenied
	default:
		return ErrUnavailable
	}
}
