package drive

import (
	"context"
	"errors"
	"fmt"

	"InquiryExplorer/account"
)

// AccountDriveData represents one account's Drive data.
type AccountDriveData struct {
	Account  account.Account `json:"account"`
	Overview *DriveOverview  `json:"overview,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// AllAccountsOverview represents Drive data from all accounts.
type AllAccountsOverview struct {
	Accounts []AccountDriveData `json:"accounts"`
}

// GetAllAccountsOverview retrieves Drive data from all accounts.
func GetAllAccountsOverview(
	ctx context.Context,
	manager *account.Manager,
) (*AllAccountsOverview, error) {
	if manager == nil {
		return nil, errors.New("account manager is nil")
	}

	accounts := manager.List()

	result := &AllAccountsOverview{
		Accounts: make([]AccountDriveData, 0, len(accounts)),
	}

	for _, acc := range accounts {
		item := AccountDriveData{
			Account: acc,
		}

		service, err := NewClient(ctx, acc)
		if err != nil {
			item.Error = fmt.Sprintf("client creation failed: %v", err)
			result.Accounts = append(result.Accounts, item)
			continue
		}

		overview, err := GetDriveOverview(ctx, service)
		if err != nil {
			item.Error = fmt.Sprintf("Drive retrieval failed: %v", err)
			result.Accounts = append(result.Accounts, item)
			continue
		}

		item.Overview = overview
		result.Accounts = append(result.Accounts, item)
	}

	return result, nil
}
