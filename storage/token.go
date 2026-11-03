package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"InquiryExplorer/file"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

var tokenMagic = []byte("INQUIRYEXPLORER-DPAPI-V1\n")

// tokenPath returns the encrypted token file path.
func tokenPath(accountID uuid.UUID) (string, error) {
	if accountID == uuid.Nil {
		return "", errors.New("invalid account ID")
	}

	dir, err := file.OAuthFolderPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, accountID.String()+".token"), nil
}

// legacyTokenPath returns the legacy JSON token file path.
func legacyTokenPath(accountID uuid.UUID) (string, error) {
	if accountID == uuid.Nil {
		return "", errors.New("invalid account ID")
	}

	dir, err := file.OAuthFolderPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, accountID.String()+".json"), nil
}

// SaveToken encrypts and saves an OAuth token.
func SaveToken(accountID uuid.UUID, token *oauth2.Token) error {
	if token == nil {
		return errors.New("token is nil")
	}

	path, err := tokenPath(accountID)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	// Serialize the token.
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to serialize token: %w", err)
	}

	// Encrypt the serialized token.
	encrypted, err := protectData(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt token: %w", err)
	}

	// Add the storage format header.
	output := make([]byte, 0, len(tokenMagic)+len(encrypted))
	output = append(output, tokenMagic...)
	output = append(output, encrypted...)

	// Save the encrypted token.
	if err := os.WriteFile(path, output, 0600); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("failed to set token permissions: %w", err)
	}

	return nil
}

// LoadToken loads and decrypts an OAuth token.
func LoadToken(accountID uuid.UUID) (*oauth2.Token, error) {
	path, err := tokenPath(accountID)
	if err != nil {
		return nil, err
	}

	// Check the new .token file first.
	data, err := os.ReadFile(path)

	if errors.Is(err, os.ErrNotExist) {
		// Fall back to the legacy .json file.
		legacyPath, pathErr := legacyTokenPath(accountID)
		if pathErr != nil {
			return nil, pathErr
		}

		data, err = os.ReadFile(legacyPath)
		if err != nil {
			return nil, err
		}

		var token oauth2.Token

		if err := json.Unmarshal(data, &token); err != nil {
			return nil, fmt.Errorf(
				"failed to deserialize legacy token: %w", err)
		}

		// Migrate the legacy token to encrypted .token storage.
		if err := SaveToken(accountID, &token); err != nil {
			return nil, fmt.Errorf(
				"failed to migrate legacy token: %w", err)
		}

		// Delete the legacy file only after successful migration.
		if err := os.Remove(legacyPath); err != nil {
			return nil, fmt.Errorf(
				"token migrated but failed to remove legacy file: %w", err)
		}

		return &token, nil
	}

	if err != nil {
		return nil, err
	}

	var tokenData []byte

	if bytes.HasPrefix(data, tokenMagic) {
		// Encrypted token.
		encrypted := data[len(tokenMagic):]

		tokenData, err = unprotectData(encrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt token: %w", err)
		}
	} else {
		// Legacy plaintext JSON stored in .token.
		tokenData = data
	}

	var token oauth2.Token

	if err := json.Unmarshal(tokenData, &token); err != nil {
		return nil, fmt.Errorf("failed to deserialize token: %w", err)
	}

	// Migrate plaintext tokens to encrypted storage.
	if !bytes.HasPrefix(data, tokenMagic) {
		if err := SaveToken(accountID, &token); err != nil {
			return nil, fmt.Errorf(
				"failed to migrate token to encrypted storage: %w", err)
		}
	}

	return &token, nil
}

// DeleteToken deletes an OAuth token.
func DeleteToken(accountID uuid.UUID) error {
	path, err := tokenPath(accountID)
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}
