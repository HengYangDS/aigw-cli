//go:build windows

package secrets

import (
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func encodeCredential(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, errors.New("Token is empty")
	}
	input := windows.DataBlob{Size: uint32(len(plain)), Data: &plain[0]}
	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	return consumeLocalBlob(output)
}

func decodeCredential(protected []byte) ([]byte, error) {
	if len(protected) == 0 {
		return nil, errors.New("protected Token is empty")
	}
	input := windows.DataBlob{Size: uint32(len(protected)), Data: &protected[0]}
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	return consumeLocalBlob(output)
}

func consumeLocalBlob(blob windows.DataBlob) ([]byte, error) {
	if blob.Size == 0 || blob.Data == nil {
		return nil, errors.New("Windows DPAPI returned an empty value")
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(blob.Data))) }()
	result := make([]byte, int(blob.Size))
	copy(result, unsafe.Slice(blob.Data, int(blob.Size)))
	return result, nil
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
