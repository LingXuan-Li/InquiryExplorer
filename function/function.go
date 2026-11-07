package function

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"InquiryExplorer/account"
	"InquiryExplorer/auth"
	"InquiryExplorer/file"
	"InquiryExplorer/scan"
)

type App struct {
	AccountManager *account.Manager
}

// Functions exposed to JavaScript.

func (a *App) GetStorageInfo() ([]scan.StorageInfo, error) {
	storage, err := scan.GetStorageInfo()
	if err != nil {
		return nil, err
	}

	return storage, nil
}

func (a *App) GetDesktopInfo() (*file.FolderInfo, error) {
	folder, err := file.GetDesktopInfo()
	if err != nil {
		return nil, err
	}

	if err := file.MakeDesktopFolder(); err != nil {
		return nil, err
	}

	dataDir, err := file.DesktopFolderPath()
	if err != nil {
		return nil, err
	}

	jsonData, err := json.MarshalIndent(folder, "", "  ")
	if err != nil {
		return nil, err
	}

	jsonPath := filepath.Join(dataDir, "desktop_info.json")

	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return nil, err
	}

	// fmt.Printf("Desktop info saved: %s\n", jsonPath)

	return folder, nil
}

// RegisterGoogleAccount starts Google OAuth authentication.
func (a *App) RegisterGoogleAccount() error {
	if a.AccountManager == nil {
		return errors.New("account manager is not initialized")
	}

	_, err := auth.GoogleAuth(
		context.Background(),
		a.AccountManager,
	)
	if err != nil {
		return err
	}

	return nil
}

// GetAccounts returns all registered accounts.
func (a *App) GetAccounts() ([]account.Account, error) {
	if a.AccountManager == nil {
		return nil, errors.New("account manager is not initialized")
	}

	return a.AccountManager.List(), nil
}
