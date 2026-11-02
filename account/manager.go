package account

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"InquiryExplorer/file"
	"InquiryExplorer/storage"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

var (
	ErrAccountNotFound = errors.New("account not found")
	ErrAccountExists   = errors.New("account already exists")
)

type Manager struct {
	mu       sync.RWMutex
	accounts []Account
}

type accountIndex struct {
	Accounts []Account `json:"accounts"`
}

// NewManager creates an account manager.
func NewManager() (*Manager, error) {
	manager := &Manager{
		accounts: []Account{},
	}

	if err := manager.Load(); err != nil {
		return nil, err
	}

	return manager, nil
}

// indexPath returns the account registry path.
func indexPath() (string, error) {
	dir, err := file.OAuthFolderPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "index.json"), nil
}

// Load loads the account registry.
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path, err := indexPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		m.accounts = []Account{}
		return nil
	}
	if err != nil {
		return err
	}

	var index accountIndex

	if err := json.Unmarshal(data, &index); err != nil {
		return err
	}

	if index.Accounts == nil {
		index.Accounts = []Account{}
	}

	m.accounts = index.Accounts

	return nil
}

// saveLocked saves the account registry.
// The caller must hold m.mu.
func (m *Manager) saveLocked(accounts []Account) error {
	path, err := indexPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	index := accountIndex{
		Accounts: accounts,
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(dir, "index-*.tmp")
	if err != nil {
		return err
	}

	tempPath := tempFile.Name()

	defer os.Remove(tempPath)

	if err := tempFile.Chmod(0600); err != nil {
		tempFile.Close()
		return err
	}

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return err
	}

	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return err
	}

	if err := tempFile.Close(); err != nil {
		return err
	}

	return os.Rename(tempPath, path)
}

// Save saves the account registry.
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.saveLocked(m.accounts)
}

// Add registers a new account.
func (m *Manager) Add(account Account, token *oauth2.Token) error {
	if account.ID == uuid.Nil {
		return errors.New("invalid account ID")
	}

	if token == nil {
		return errors.New("token is nil")
	}

	if account.Email == "" {
		return errors.New("email is required")
	}

	if account.Vendor == "" {
		return errors.New("vendor is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.accounts {
		if existing.ID == account.ID {
			return ErrAccountExists
		}
	}

	next := make([]Account, len(m.accounts), len(m.accounts)+1)
	copy(next, m.accounts)
	next = append(next, account)

	if err := storage.SaveToken(account.ID, token); err != nil {
		return err
	}

	if err := m.saveLocked(next); err != nil {
		_ = storage.DeleteToken(account.ID)
		return err
	}

	m.accounts = next

	return nil
}

// Remove deletes an account.
func (m *Manager) Remove(accountID uuid.UUID) error {
	if accountID == uuid.Nil {
		return errors.New("invalid account ID")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	index := -1

	for i, account := range m.accounts {
		if account.ID == accountID {
			index = i
			break
		}
	}

	if index == -1 {
		return ErrAccountNotFound
	}

	next := make([]Account, 0, len(m.accounts)-1)
	next = append(next, m.accounts[:index]...)
	next = append(next, m.accounts[index+1:]...)

	if err := m.saveLocked(next); err != nil {
		return err
	}

	m.accounts = next

	if err := storage.DeleteToken(accountID); err != nil {
		return err
	}

	return nil
}

// List returns all registered accounts.
func (m *Manager) List() []Account {
	m.mu.RLock()
	defer m.mu.RUnlock()

	accounts := make([]Account, len(m.accounts))
	copy(accounts, m.accounts)

	return accounts
}

// Get returns an account by ID.
func (m *Manager) Get(accountID uuid.UUID) (*Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.accounts {
		if m.accounts[i].ID == accountID {
			account := m.accounts[i]
			return &account, nil
		}
	}

	return nil, ErrAccountNotFound
}
