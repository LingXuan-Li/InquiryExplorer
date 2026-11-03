package storage

import (
	"bytes"
	"encoding/json"
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

	// 保存
	if err := SaveToken(accountID, token); err != nil {
		t.Fatalf("SaveToken() error = %v", err)
	}

	// 読み込み
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

	// ファイルが存在することを確認
	path, err := tokenPath(accountID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("token file does not exist: %v", err)
	}

	// 削除
	if err := DeleteToken(accountID); err != nil {
		t.Fatalf("DeleteToken() error = %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("token file still exists after deletion")
	}
}

func TestLoadTokenNotFound(t *testing.T) {
	_, err := LoadToken(uuid.New())

	if err == nil {
		t.Fatal("LoadToken() expected an error for missing token")
	}
}

func TestDeleteTokenNotFound(t *testing.T) {
	err := DeleteToken(uuid.New())

	if err != nil {
		t.Errorf("DeleteToken() should ignore missing files, got %v", err)
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

	// 暗号化済みデータであることを確認
	if len(data) <= len(tokenMagic) {
		t.Fatal("encrypted token data is too short")
	}

	if string(data[:len(tokenMagic)]) != string(tokenMagic) {
		t.Fatal("token encryption magic header not found")
	}

	// 平文のトークン情報が含まれていないことを確認
	if string(data) == "" {
		t.Fatal("token file is empty")
	}

	if contains := func() bool {
		return len(data) >= len(token.RefreshToken) &&
			string(data) != token.RefreshToken
	}(); !contains {
		t.Fatal("token data appears to be unencrypted")
	}
}

func TestLegacyTokenMigration(t *testing.T) {
	accountID := uuid.New()
	defer DeleteToken(accountID)

	path, err := tokenPath(accountID)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}

	legacyToken := &oauth2.Token{
		AccessToken:  "legacy-access-token",
		RefreshToken: "legacy-refresh-token",
		TokenType:    "Bearer",
	}

	// 旧形式の平文JSONを作成
	legacyData, err := json.Marshal(legacyToken)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, legacyData, 0600); err != nil {
		t.Fatal(err)
	}

	// 読み込み時に新形式へ移行されることを確認
	loaded, err := LoadToken(accountID)
	if err != nil {
		t.Fatalf("LoadToken() error = %v", err)
	}

	if loaded.RefreshToken != legacyToken.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q",
			loaded.RefreshToken, legacyToken.RefreshToken)
	}

	migratedData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(migratedData) < len(tokenMagic) ||
		string(migratedData[:len(tokenMagic)]) != string(tokenMagic) {
		t.Fatal("legacy token was not migrated to encrypted format")
	}
}

func TestSaveLoadToken(t *testing.T) {
	id := uuid.New()

	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
	}

	if err := SaveToken(id, token); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	defer DeleteToken(id)

	loaded, err := LoadToken(id)
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}

	if loaded.AccessToken != token.AccessToken {
		t.Errorf("AccessToken = %q, want %q",
			loaded.AccessToken, token.AccessToken)
	}

	if loaded.RefreshToken != token.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q",
			loaded.RefreshToken, token.RefreshToken)
	}
}

func TestTokenFileEncrypted(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("DPAPI encryption test requires Windows")
	}

	id := uuid.New()

	token := &oauth2.Token{
		AccessToken:  "plain-access-token",
		RefreshToken: "plain-refresh-token",
		TokenType:    "Bearer",
	}

	if err := SaveToken(id, token); err != nil {
		t.Fatal(err)
	}
	defer DeleteToken(id)

	path, err := tokenPath(id)
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

	if len(data) == 0 {
		t.Fatal("encrypted token file is empty")
	}
}

func TestLoadTokenMissing(t *testing.T) {
	_, err := LoadToken(uuid.New())

	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestDeleteToken(t *testing.T) {
	id := uuid.New()

	token := &oauth2.Token{
		AccessToken: "test-access-token",
	}

	if err := SaveToken(id, token); err != nil {
		t.Fatal(err)
	}

	if err := DeleteToken(id); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}

	if _, err := LoadToken(id); !os.IsNotExist(err) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}
