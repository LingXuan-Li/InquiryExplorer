package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

func TestSaveAndLoadToken(t *testing.T) {
	accountID := uuid.New()

	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		TokenType:    "Bearer",
		RefreshToken: "test-refresh-token",
	}

	if err := SaveToken(accountID, token); err != nil {
		t.Fatalf("SaveToken() error = %v", err)
	}
	defer DeleteToken(accountID)

	loaded, err := LoadToken(accountID)
	if err != nil {
		t.Fatalf("LoadToken() error = %v", err)
	}

	if loaded.AccessToken != token.AccessToken {
		t.Errorf("AccessToken = %q, want %q",
			loaded.AccessToken, token.AccessToken)
	}

	if loaded.RefreshToken != token.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q",
			loaded.RefreshToken, token.RefreshToken)
	}

	if loaded.TokenType != token.TokenType {
		t.Errorf("TokenType = %q, want %q",
			loaded.TokenType, token.TokenType)
	}

	path, err := tokenPath(accountID)
	if err != nil {
		t.Fatal(err)
	}

	if filepath.Ext(path) != ".token" {
		t.Errorf("file extension = %q, want .token",
			filepath.Ext(path))
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("token file does not exist: %v", err)
	}
}

func TestLoadTokenNotFound(t *testing.T) {
	_, err := LoadToken(uuid.New())

	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestDeleteTokenNotFound(t *testing.T) {
	err := DeleteToken(uuid.New())

	if err != nil {
		t.Errorf("DeleteToken() should ignore missing files: %v", err)
	}
}

func TestInvalidAccountID(t *testing.T) {
	_, err := tokenPath(uuid.Nil)

	if err == nil {
		t.Fatal("tokenPath() expected an error for uuid.Nil")
	}
}

func TestTokenEncryption(t *testing.T) {
	accountID := uuid.New()

	token := &oauth2.Token{
		AccessToken:  "encryption-test-access",
		RefreshToken: "encryption-test-refresh",
		TokenType:    "Bearer",
	}

	if err := SaveToken(accountID, token); err != nil {
		t.Fatal(err)
	}
	defer DeleteToken(accountID)

	path, err := tokenPath(accountID)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.HasPrefix(data, tokenMagic) {
		t.Fatal("token encryption magic header not found")
	}

	if bytes.Contains(data, []byte(token.AccessToken)) {
		t.Error("access token is stored in plaintext")
	}

	if bytes.Contains(data, []byte(token.RefreshToken)) {
		t.Error("refresh token is stored in plaintext")
	}
}

// TestLegacyJSONMigration verifies migration from old .json files.
func TestLegacyJSONMigration(t *testing.T) {
	accountID := uuid.New()

	token, err := tokenPath(accountID)
	if err != nil {
		t.Fatal(err)
	}

	legacy, err := legacyTokenPath(accountID)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(legacy), 0700); err != nil {
		t.Fatal(err)
	}

	legacyToken := &oauth2.Token{
		AccessToken:  "legacy-access-token",
		RefreshToken: "legacy-refresh-token",
		TokenType:    "Bearer",
	}

	legacyData, err := json.Marshal(legacyToken)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(legacy, legacyData, 0600); err != nil {
		t.Fatal(err)
	}

	// LoadToken should migrate .json to encrypted .token.
	loaded, err := LoadToken(accountID)
	if err != nil {
		t.Fatalf("LoadToken() migration error = %v", err)
	}

	if loaded.AccessToken != legacyToken.AccessToken {
		t.Errorf("AccessToken = %q, want %q",
			loaded.AccessToken, legacyToken.AccessToken)
	}

	if loaded.RefreshToken != legacyToken.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q",
			loaded.RefreshToken, legacyToken.RefreshToken)
	}

	// Verify new .token file exists.
	migratedData, err := os.ReadFile(token)
	if err != nil {
		t.Fatalf("migrated .token file does not exist: %v", err)
	}

	if !bytes.HasPrefix(migratedData, tokenMagic) {
		t.Fatal("migrated token is not encrypted")
	}

	// Verify old .json file was removed.
	if _, err := os.Stat(legacy); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("legacy .json file still exists")
	}

	// Verify the migrated token can be loaded again.
	reloaded, err := LoadToken(accountID)
	if err != nil {
		t.Fatalf("LoadToken() after migration error = %v", err)
	}

	if reloaded.RefreshToken != legacyToken.RefreshToken {
		t.Errorf("reloaded RefreshToken = %q, want %q",
			reloaded.RefreshToken, legacyToken.RefreshToken)
	}

	defer DeleteToken(accountID)
}

// TestLegacyPlaintextMigration verifies migration of plaintext JSON
// stored in the new .token path.
func TestLegacyPlaintextMigration(t *testing.T) {
	accountID := uuid.New()
	defer DeleteToken(accountID)

	path, err := tokenPath(accountID)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}

	token := &oauth2.Token{
		AccessToken:  "plaintext-access-token",
		RefreshToken: "plaintext-refresh-token",
		TokenType:    "Bearer",
	}

	data, err := json.Marshal(token)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadToken(accountID)
	if err != nil {
		t.Fatalf("LoadToken() error = %v", err)
	}

	if loaded.RefreshToken != token.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q",
			loaded.RefreshToken, token.RefreshToken)
	}

	migratedData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.HasPrefix(migratedData, tokenMagic) {
		t.Fatal("plaintext token was not migrated to encrypted format")
	}
}

func TestDeleteToken(t *testing.T) {
	accountID := uuid.New()

	token := &oauth2.Token{
		AccessToken: "test-access-token",
	}

	if err := SaveToken(accountID, token); err != nil {
		t.Fatal(err)
	}

	path, err := tokenPath(accountID)
	if err != nil {
		t.Fatal(err)
	}

	if err := DeleteToken(accountID); err != nil {
		t.Fatalf("DeleteToken() error = %v", err)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("token file still exists after deletion")
	}
}

func TestTokenFileEncrypted(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("DPAPI encryption test requires Windows")
	}

	accountID := uuid.New()

	token := &oauth2.Token{
		AccessToken:  "plain-access-token",
		RefreshToken: "plain-refresh-token",
		TokenType:    "Bearer",
	}

	if err := SaveToken(accountID, token); err != nil {
		t.Fatal(err)
	}
	defer DeleteToken(accountID)

	path, err := tokenPath(accountID)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Contains(data, []byte(token.AccessToken)) {
		t.Error("access token is stored in plaintext")
	}

	if bytes.Contains(data, []byte(token.RefreshToken)) {
		t.Error("refresh token is stored in plaintext")
	}

	if !bytes.HasPrefix(data, tokenMagic) {
		t.Error("encryption header not found")
	}
}
