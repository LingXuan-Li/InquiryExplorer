//go:build windows

package storage

import (
	"bytes"
	"testing"
)

func TestDPAPIProtectUnprotect(t *testing.T) {
	original := []byte("InquiryExplorer DPAPI Test Data")

	// 暗号化
	encrypted, err := protectData(original)
	if err != nil {
		t.Fatalf("protectData failed: %v", err)
	}

	if len(encrypted) == 0 {
		t.Fatal("encrypted data is empty")
	}

	// 暗号文が平文と異なることを確認
	if bytes.Equal(original, encrypted) {
		t.Fatal("encrypted data is identical to plaintext")
	}

	// 復号
	decrypted, err := unprotectData(encrypted)
	if err != nil {
		t.Fatalf("unprotectData failed: %v", err)
	}

	// 元データと一致することを確認
	if !bytes.Equal(original, decrypted) {
		t.Errorf(
			"decrypted data does not match original\noriginal:  %q\ndecrypted: %q",
			original,
			decrypted,
		)
	}
}

func TestDPAPITamperedData(t *testing.T) {
	original := []byte("InquiryExplorer DPAPI Tamper Test")

	encrypted, err := protectData(original)
	if err != nil {
		t.Fatalf("protectData failed: %v", err)
	}

	// 暗号文を改変
	tampered := append([]byte(nil), encrypted...)
	tampered[len(tampered)-1] ^= 0xFF

	// 改変された暗号文の復号が失敗することを確認
	_, err = unprotectData(tampered)
	if err == nil {
		t.Fatal("expected unprotectData to fail for tampered data")
	}
}
