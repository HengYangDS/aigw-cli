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
	} {
		address, err := purego.Dlsym(lib, binding.name)
		if err != nil {
			return nativeAPI{}, nil, errors.Join(ErrUnavailable, closeLibrary())
		}
		purego.RegisterFunc(binding.target, address)
	}
	return api, closeLibrary, nil
}

func queryNative(service, account string, observe bool) (value []byte, err error) {
	api, closeLibrary, err := loadNative()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, closeLibrary()) }()
	return api.read(0, service, account, observe)
}

func (api nativeAPI) read(keychain uintptr, service, account string, observe bool) ([]byte, error) {
	if len(service) > math.MaxUint32 || len(account) > math.MaxUint32 {
		return nil, ErrUnavailable
	}
	// These legacy APIs address the existing file-based Keychain used by
	// go-keyring. UI suppression is process-wide, so only the isolated worker
	// uses it; the parent and other credential operations are unaffected.
	if api.interaction(false) != 0 {
		return nil, ErrUnavailable
	}
	var length uint32
	var data unsafe.Pointer
	sizeTarget, dataTarget := &length, &data
	if observe {
		sizeTarget, dataTarget = nil, nil
	}
	// #nosec G115 -- Both byte lengths are checked against MaxUint32 before this native call.
	status := api.find(keychain, uint32(len(service)), service, uint32(len(account)), account, sizeTarget, dataTarget, nil)
	if status != 0 {
		switch status {
		case -25300:
			return nil, ErrNotFound
		case -25308, -25293:
			return nil, ErrDenied
		default:
			return nil, ErrUnavailable
		}
	}
	if observe {
		return nil, nil
	}
	var value []byte
	if data != nil && length <= 128*1024 {
		// #nosec G103 -- Security.framework owns this exact-length buffer until FreeContent below.
		value = append([]byte(nil), unsafe.Slice((*byte)(data), length)...)
	}
	if api.free(0, data) != 0 || length > 128*1024 || data == nil && length != 0 {
		clear(value)
		return nil, ErrUnavailable
	}
	return value, nil
}
