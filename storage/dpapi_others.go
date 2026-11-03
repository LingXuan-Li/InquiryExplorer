//go:build !windows

package storage

import "errors"

var ErrDPAPIUnsupported = errors.New(
	"Windows DPAPI is not supported on this operating system",
)

func protectData(data []byte) ([]byte, error) {
	return nil, ErrDPAPIUnsupported
}

func unprotectData(data []byte) ([]byte, error) {
	return nil, ErrDPAPIUnsupported
}
