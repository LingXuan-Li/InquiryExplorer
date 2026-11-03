//go:build windows

package storage

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

const cryptProtectUIForbidden = 0x1

type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32DLL  = syscall.NewLazyDLL("crypt32.dll")
	kernel32DLL = syscall.NewLazyDLL("kernel32.dll")

	procCryptProtectData   = crypt32DLL.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32DLL.NewProc("CryptUnprotectData")
	procLocalFree          = kernel32DLL.NewProc("LocalFree")
)

// protectData encrypts data using Windows DPAPI.
// The encrypted data is protected for the current Windows user.
func protectData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("cannot encrypt empty data")
	}

	input := dataBlob{
		cbData: uint32(len(data)),
		pbData: &data[0],
	}

	var output dataBlob

	result, _, callErr := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&input)),
		0,
		0,
		0,
		0,
		cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&output)),
	)

	runtime.KeepAlive(data)

	if result == 0 {
		return nil, fmt.Errorf(
			"CryptProtectData failed: %w",
			callErr,
		)
	}

	defer procLocalFree.Call(uintptr(unsafe.Pointer(output.pbData)))

	encrypted := unsafe.Slice(output.pbData, output.cbData)
	return append([]byte(nil), encrypted...), nil
}

// unprotectData decrypts data using Windows DPAPI.
func unprotectData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("cannot decrypt empty data")
	}

	input := dataBlob{
		cbData: uint32(len(data)),
		pbData: &data[0],
	}

	var output dataBlob

	result, _, callErr := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&input)),
		0,
		0,
		0,
		0,
		cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&output)),
	)

	runtime.KeepAlive(data)

	if result == 0 {
		return nil, fmt.Errorf(
			"CryptUnprotectData failed: %w",
			callErr,
		)
	}

	defer procLocalFree.Call(uintptr(unsafe.Pointer(output.pbData)))

	decrypted := unsafe.Slice(output.pbData, output.cbData)
	return append([]byte(nil), decrypted...), nil
}
