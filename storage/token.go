package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"InquiryExplorer/file"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

func tokenPath(accountID uuid.UUID) (string, error) {
	if accountID == uuid.Nil {
		return "", errors.New("invalid account ID")
	}

	dir, err := file.OAuthFolderPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, accountID.String()+".json"), nil
}

// SaveToken saves an OAuth token.
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

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// LoadToken loads an OAuth token.
func LoadToken(accountID uuid.UUID) (*oauth2.Token, error) {
	path, err := tokenPath(accountID)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var token oauth2.Token

	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
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
