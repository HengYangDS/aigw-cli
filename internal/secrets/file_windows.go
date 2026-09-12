//go:build windows

package secrets

import (
	"errors"
	"math"
	"os"
	"slices"
	"unsafe"

	"golang.org/x/sys/windows"
)

func encodeCredential(plain []byte) ([]byte, error) {
	size := len(plain)
	if size == 0 {
		return nil, errors.New("Token is empty")
	}
	if size > math.MaxUint32 {
		return nil, errors.New("Token exceeds the Windows DPAPI input limit")
	}
	input := windows.DataBlob{Size: uint32(size), Data: &plain[0]}
	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	return consumeLocalBlob(output)
}

func decodeCredential(protected []byte) ([]byte, error) {
	size := len(protected)
	if size == 0 {
		return nil, errors.New("protected Token is empty")
	}
	if size > math.MaxUint32 {
		return nil, errors.New("protected Token exceeds the Windows DPAPI input limit")
	}
	input := windows.DataBlob{Size: uint32(size), Data: &protected[0]}
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	return consumeLocalBlob(output)
}

func consumeLocalBlob(blob windows.DataBlob) (result []byte, err error) {
	if blob.Data == nil {
		return nil, errors.New("Windows DPAPI returned an empty value")
	}
	defer func() {
		// #nosec G103 -- DPAPI allocates this pointer with LocalAlloc; LocalFree owns its release.
		_, freeErr := windows.LocalFree(windows.Handle(unsafe.Pointer(blob.Data)))
		err = errors.Join(err, freeErr)
	}()
	if blob.Size == 0 {
		return nil, errors.New("Windows DPAPI returned an empty value")
	}
	// #nosec G103 -- The OS supplies the buffer length; clone before releasing its allocation.
	return slices.Clone(unsafe.Slice(blob.Data, blob.Size)), nil
}

func validateOwnedFile(info os.FileInfo) error {
	if !info.Mode().IsRegular() {
		return errors.New("Token path must be a regular file")
	}
	return nil
}

func validateSecureRoot(info os.FileInfo) error {
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("Token directory must be a real directory")
	}
	return nil
}

func syncCredentialDirectory(syncer) error { return nil }
