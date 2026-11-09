package function

import (
	"context"
	"errors"

	"InquiryExplorer/account"
	"InquiryExplorer/auth"
	"InquiryExplorer/database"
	"InquiryExplorer/file"
	"InquiryExplorer/scan"

	"gorm.io/gorm"
)

type App struct {
	AccountManager *account.Manager
	DB             *gorm.DB
}

// Functions exposed to JavaScript.

func (a *App) GetStorageInfo() ([]scan.StorageInfo, error) {
	storage, err := scan.GetStorageInfo()
	if err != nil {
		return nil, err
	}

	return storage, nil
}

// StoreFileBinary reads a file and stores its DEFLATE-compressed contents in SQLite.
func (a *App) StoreFileBinary(path string) (string, error) {
	data, err := file.ReadBinary(path)
	if err != nil {
		return "", err
	}
	return database.SaveBinary(a.DB, data)
}

func (a *App) GetDesktopInfo(forceRefresh ...bool) (*file.FolderInfo, error) {
	if len(forceRefresh) > 0 && forceRefresh[0] {
		folder, err := file.GetDesktopInfo()
		if err != nil {
			return nil, err
		}
		if err := database.SaveDesktopFiles(a.DB, folder); err != nil {
			return nil, err
		}
		return folder, nil
	}
	// DBからキャッシュを取得
	cached, err := database.GetDesktopFiles(a.DB)
	if err != nil {
		return nil, err
	}

	// キャッシュが存在する場合
	if cached != nil {
		return cached, nil
	}

	// キャッシュがない場合は実際のファイル情報を取得
	folder, err := file.GetDesktopInfo()
	if err != nil {
		return nil, err
	}

	if err := database.SaveDesktopFiles(a.DB, folder); err != nil {
		return nil, err
	}

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
