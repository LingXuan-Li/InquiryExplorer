package drive

import (
	"context"
	"strings"
	"testing"
	"time"

	"InquiryExplorer/account"
	"InquiryExplorer/cloud"
	"InquiryExplorer/storage"

	"github.com/google/uuid"
)

// setupManagerWithAccounts registers the given accounts (each with a valid,
// non-expired token) in a fresh manager backed by an isolated directory.
func setupManagerWithAccounts(t *testing.T, accounts ...account.Account) *account.Manager {
	t.Helper()
	setupEnv(t)

	m, err := account.NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	for _, acc := range accounts {
		token := newTestToken("access-"+acc.Email, time.Now().Add(time.Hour))
		if err := m.Add(acc, token); err != nil {
			t.Fatalf("Add(%s): %v", acc.Email, err)
		}
	}

	return m
}

func newGoogleAccount(email string) account.Account {
	return account.Account{
		ID:          uuid.New(),
		Email:       email,
		DisplayName: email,
		Vendor:      cloud.GoogleProvider,
		CreatedAt:   time.Now().UTC().Truncate(time.Second),
	}
}

func TestGetAllAccountsOverview_NilManager(t *testing.T) {
	result, err := GetAllAccountsOverview(context.Background(), nil)

	if err == nil {
		t.Fatal("expected an error for a nil manager")
	}
	if result != nil {
		t.Errorf("result = %+v, want nil", result)
	}
}

func TestGetAllAccountsOverview_NoAccounts(t *testing.T) {
	m := setupManagerWithAccounts(t)

	result, err := GetAllAccountsOverview(context.Background(), m)
	if err != nil {
		t.Fatalf("GetAllAccountsOverview: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Accounts == nil || len(result.Accounts) != 0 {
		t.Errorf("Accounts = %v, want empty non-nil slice", result.Accounts)
	}
}

// A failure for one account must not stop the others, and the original order
// must be preserved.
func TestGetAllAccountsOverview_ErrorsAreReportedPerAccount(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")

	nonGoogle := account.Account{
		ID:          uuid.New(),
		Email:       "dropbox@example.com",
		DisplayName: "Dropbox",
		Vendor:      "dropbox",
		CreatedAt:   time.Now().UTC().Truncate(time.Second),
	}
	missingToken := newGoogleAccount("missing-token@example.com")
	apiFailure := newGoogleAccount("api-failure@example.com")

	m := setupManagerWithAccounts(t, nonGoogle, missingToken, apiFailure)

	// Simulate an account whose token has disappeared from storage.
	if err := storage.DeleteToken(missingToken.ID); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}

	// A canceled context makes the Drive API call fail immediately without
	// touching the network, after the client itself was created successfully.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := GetAllAccountsOverview(ctx, m)
	if err != nil {
		t.Fatalf("GetAllAccountsOverview must not fail as a whole: %v", err)
	}
	if len(result.Accounts) != 3 {
		t.Fatalf("len(Accounts) = %d, want 3", len(result.Accounts))
	}

	tests := []struct {
		index   int
		wantID  uuid.UUID
		wantErr string
	}{
		{0, nonGoogle.ID, "client creation failed"},
		{1, missingToken.ID, "client creation failed"},
		{2, apiFailure.ID, "Drive retrieval failed"},
	}

	for _, tt := range tests {
		item := result.Accounts[tt.index]

		if item.Account.ID != tt.wantID {
			t.Errorf("[%d] Account.ID = %v, want %v", tt.index, item.Account.ID, tt.wantID)
		}
		if item.Overview != nil {
			t.Errorf("[%d] Overview = %+v, want nil", tt.index, item.Overview)
		}
		if !strings.Contains(item.Error, tt.wantErr) {
			t.Errorf("[%d] Error = %q, want it to contain %q", tt.index, item.Error, tt.wantErr)
		}
	}

	// The specific cause should be included in the message.
	if !strings.Contains(result.Accounts[0].Error, ErrInvalidAccount.Error()) {
		t.Errorf("non-google account error = %q, want %q inside", result.Accounts[0].Error, ErrInvalidAccount)
	}
	if !strings.Contains(result.Accounts[1].Error, ErrTokenMissing.Error()) {
		t.Errorf("missing token error = %q, want %q inside", result.Accounts[1].Error, ErrTokenMissing)
	}
}

func TestGetAllAccountsOverview_ClientIDMissing(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")

	a := newGoogleAccount("a@example.com")
	b := newGoogleAccount("b@example.com")
	m := setupManagerWithAccounts(t, a, b)

	// setupManagerWithAccounts → setupEnv does not touch GOOGLE_CLIENT_ID,
	// but make sure it is still empty.
	t.Setenv("GOOGLE_CLIENT_ID", "")

	result, err := GetAllAccountsOverview(context.Background(), m)
	if err != nil {
		t.Fatalf("GetAllAccountsOverview: %v", err)
	}

	if len(result.Accounts) != 2 {
		t.Fatalf("len(Accounts) = %d, want 2", len(result.Accounts))
	}

	for i, item := range result.Accounts {
		if item.Overview != nil {
			t.Errorf("[%d] Overview should be nil", i)
		}
		if !strings.Contains(item.Error, "client creation failed") ||
			!strings.Contains(item.Error, cloud.ErrGoogleClientIDMissing.Error()) {
			t.Errorf("[%d] Error = %q", i, item.Error)
		}
	}
}
