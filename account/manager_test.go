package account

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"InquiryExplorer/storage"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

// setupEnv redirects every per-user directory to a temporary directory so
// that tests never touch the real OAuth folder / token storage.
//
// NOTE: This assumes file.OAuthFolderPath() and the storage package resolve
// their paths from the user's home/config directory. Adjust if they use
// something else (e.g. an OS keyring).
func setupEnv(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	for _, key := range []string{
		"HOME",
		"USERPROFILE",
		"XDG_CONFIG_HOME",
		"XDG_DATA_HOME",
		"XDG_STATE_HOME",
		"APPDATA",
		"LOCALAPPDATA",
	} {
		t.Setenv(key, dir)
	}
}

func newTestAccount(email string) Account {
	return Account{
		ID:          uuid.New(),
		Email:       email,
		DisplayName: "Test " + email,
		Vendor:      "google",
		CreatedAt:   time.Now().UTC().Truncate(time.Second),
	}
}

func newTestToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour).UTC().Truncate(time.Second),
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	setupEnv(t)

	m, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

func TestNewManager_Empty(t *testing.T) {
	m := newTestManager(t)

	if got := m.List(); got == nil || len(got) != 0 {
		t.Errorf("List() = %v, want empty non-nil slice", got)
	}
}

func TestManager_Add_Validation(t *testing.T) {
	valid := newTestAccount("valid@example.com")

	tests := []struct {
		name    string
		account Account
		token   *oauth2.Token
	}{
		{
			name:    "nil account ID",
			account: Account{Email: "a@example.com", Vendor: "google"},
			token:   newTestToken(),
		},
		{
			name:    "nil token",
			account: valid,
			token:   nil,
		},
		{
			name:    "empty email",
			account: Account{ID: uuid.New(), Vendor: "google"},
			token:   newTestToken(),
		},
		{
			name:    "empty vendor",
			account: Account{ID: uuid.New(), Email: "a@example.com"},
			token:   newTestToken(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestManager(t)

			if err := m.Add(tt.account, tt.token); err == nil {
				t.Fatal("expected an error")
			}

			if n := len(m.List()); n != 0 {
				t.Errorf("List() has %d accounts, want 0", n)
			}

			if tt.account.ID != uuid.Nil {
				if _, err := storage.LoadToken(tt.account.ID); err == nil {
					t.Error("token must not be saved when validation fails")
				}
			}
		})
	}
}

func TestManager_Add_Success(t *testing.T) {
	m := newTestManager(t)

	acc := newTestAccount("user@example.com")
	token := newTestToken()

	if err := m.Add(acc, token); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// In-memory state.
	got, err := m.Get(acc.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Email != acc.Email || got.Vendor != acc.Vendor {
		t.Errorf("Get() = %+v, want %+v", got, acc)
	}

	// Token persisted.
	saved, err := storage.LoadToken(acc.ID)
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if saved.AccessToken != token.AccessToken || saved.RefreshToken != token.RefreshToken {
		t.Errorf("saved token = %+v, want %+v", saved, token)
	}

	// Registry persisted: a fresh manager must see the account.
	reloaded, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager (reload): %v", err)
	}
	list := reloaded.List()
	if len(list) != 1 {
		t.Fatalf("reloaded List() has %d accounts, want 1", len(list))
	}
	if list[0].ID != acc.ID ||
		list[0].Email != acc.Email ||
		list[0].DisplayName != acc.DisplayName ||
		!list[0].CreatedAt.Equal(acc.CreatedAt) {
		t.Errorf("reloaded account = %+v, want %+v", list[0], acc)
	}
}

func TestManager_Add_IndexFilePermission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission bits are not meaningful on Windows")
	}

	m := newTestManager(t)

	if err := m.Add(newTestAccount("user@example.com"), newTestToken()); err != nil {
		t.Fatal(err)
	}

	path, err := indexPath()
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("index.json permission = %o, want 600", perm)
	}

	// No temporary files should be left behind.
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), "index-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Errorf("leftover temp files: %v", matches)
	}
}

func TestManager_Add_Duplicate(t *testing.T) {
	m := newTestManager(t)

	acc := newTestAccount("user@example.com")
	if err := m.Add(acc, newTestToken()); err != nil {
		t.Fatal(err)
	}

	err := m.Add(acc, newTestToken())
	if !errors.Is(err, ErrAccountExists) {
		t.Fatalf("err = %v, want %v", err, ErrAccountExists)
	}

	if n := len(m.List()); n != 1 {
		t.Errorf("List() has %d accounts, want 1", n)
	}
}

func TestManager_Add_RollsBackTokenWhenSaveFails(t *testing.T) {
	m := newTestManager(t)

	// Make index.json a non-empty directory so that the final os.Rename fails.
	path, err := indexPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(path, "blocker"), 0o700); err != nil {
		t.Fatal(err)
	}

	acc := newTestAccount("user@example.com")
	if err := m.Add(acc, newTestToken()); err == nil {
		t.Fatal("expected Add to fail when the registry cannot be written")
	}

	if n := len(m.List()); n != 0 {
		t.Errorf("in-memory accounts = %d, want 0 (state must not change on failure)", n)
	}

	if _, err := storage.LoadToken(acc.ID); err == nil {
		t.Error("token must be deleted when saving the registry fails")
	}
}

func TestManager_Remove(t *testing.T) {
	m := newTestManager(t)

	a := newTestAccount("a@example.com")
	b := newTestAccount("b@example.com")
	c := newTestAccount("c@example.com")

	for _, acc := range []Account{a, b, c} {
		if err := m.Add(acc, newTestToken()); err != nil {
			t.Fatal(err)
		}
	}

	if err := m.Remove(b.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Remaining accounts keep their order.
	list := m.List()
	if len(list) != 2 || list[0].ID != a.ID || list[1].ID != c.ID {
		t.Fatalf("List() = %+v, want [a, c]", list)
	}

	if _, err := m.Get(b.ID); !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("Get(removed) err = %v, want %v", err, ErrAccountNotFound)
	}

	// Token deleted, other tokens untouched.
	if _, err := storage.LoadToken(b.ID); err == nil {
		t.Error("token of the removed account should be deleted")
	}
	if _, err := storage.LoadToken(a.ID); err != nil {
		t.Errorf("token of another account must remain: %v", err)
	}

	// Persisted.
	reloaded, err := NewManager()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(reloaded.List()); n != 2 {
		t.Errorf("reloaded List() has %d accounts, want 2", n)
	}
}

func TestManager_Remove_Errors(t *testing.T) {
	m := newTestManager(t)

	if err := m.Remove(uuid.Nil); err == nil {
		t.Error("expected an error for uuid.Nil")
	}

	if err := m.Remove(uuid.New()); !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("err = %v, want %v", err, ErrAccountNotFound)
	}
}

func TestManager_Remove_OnlyAccount(t *testing.T) {
	m := newTestManager(t)

	acc := newTestAccount("a@example.com")
	if err := m.Add(acc, newTestToken()); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove(acc.ID); err != nil {
		t.Fatal(err)
	}

	if got := m.List(); len(got) != 0 {
		t.Errorf("List() = %v, want empty", got)
	}
}

func TestManager_List_ReturnsCopy(t *testing.T) {
	m := newTestManager(t)

	acc := newTestAccount("a@example.com")
	if err := m.Add(acc, newTestToken()); err != nil {
		t.Fatal(err)
	}

	list := m.List()
	list[0].Email = "tampered@example.com"

	if got := m.List()[0].Email; got != acc.Email {
		t.Errorf("internal state changed through List(): %q", got)
	}
}

func TestManager_Get(t *testing.T) {
	m := newTestManager(t)

	acc := newTestAccount("a@example.com")
	if err := m.Add(acc, newTestToken()); err != nil {
		t.Fatal(err)
	}

	got, err := m.Get(acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != acc.ID {
		t.Errorf("ID = %v, want %v", got.ID, acc.ID)
	}

	// Returned value is a copy.
	got.Email = "tampered@example.com"
	again, err := m.Get(acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Email != acc.Email {
		t.Errorf("internal state changed through Get(): %q", again.Email)
	}

	if _, err := m.Get(uuid.New()); !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("err = %v, want %v", err, ErrAccountNotFound)
	}
}

func TestManager_Load(t *testing.T) {
	writeIndex := func(t *testing.T, content string) {
		t.Helper()
		path, err := indexPath()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("missing file gives empty list", func(t *testing.T) {
		m := newTestManager(t)
		if err := m.Load(); err != nil {
			t.Fatal(err)
		}
		if got := m.List(); len(got) != 0 {
			t.Errorf("List() = %v, want empty", got)
		}
	})

	t.Run("null accounts gives empty non-nil list", func(t *testing.T) {
		m := newTestManager(t)
		writeIndex(t, `{"accounts": null}`)

		if err := m.Load(); err != nil {
			t.Fatal(err)
		}
		if got := m.List(); got == nil || len(got) != 0 {
			t.Errorf("List() = %v, want empty non-nil", got)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		m := newTestManager(t)
		writeIndex(t, `{not json`)

		if err := m.Load(); err == nil {
			t.Fatal("expected a JSON error")
		}
	})

	t.Run("picks up externally written data", func(t *testing.T) {
		m := newTestManager(t)
		id := uuid.New()
		writeIndex(t, fmt.Sprintf(
			`{"accounts":[{"id":%q,"email":"x@example.com","display_name":"X","vendor":"google","created_at":"2026-01-01T00:00:00Z"}]}`,
			id.String(),
		))

		if err := m.Load(); err != nil {
			t.Fatal(err)
		}
		got, err := m.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Email != "x@example.com" {
			t.Errorf("Email = %q", got.Email)
		}
	})

	t.Run("NewManager fails on corrupted registry", func(t *testing.T) {
		setupEnv(t)
		writeIndex(t, `garbage`)

		if _, err := NewManager(); err == nil {
			t.Fatal("expected NewManager to fail")
		}
	})
}

func TestManager_Save(t *testing.T) {
	m := newTestManager(t)

	if err := m.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path, err := indexPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("index.json should exist after Save: %v", err)
	}
}

// Run with `go test -race` to make this test meaningful.
func TestManager_ConcurrentAccess(t *testing.T) {
	m := newTestManager(t)

	const n = 20

	var wg sync.WaitGroup
	ids := make([]uuid.UUID, n)

	for i := 0; i < n; i++ {
		acc := newTestAccount(fmt.Sprintf("user%d@example.com", i))
		ids[i] = acc.ID

		wg.Add(2)

		go func() {
			defer wg.Done()
			if err := m.Add(acc, newTestToken()); err != nil {
				t.Errorf("Add: %v", err)
			}
		}()

		go func() {
			defer wg.Done()
			_ = m.List()
			_, _ = m.Get(acc.ID)
		}()
	}

	wg.Wait()

	if got := len(m.List()); got != n {
		t.Fatalf("List() has %d accounts, want %d", got, n)
	}

	reloaded, err := NewManager()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(reloaded.List()); got != n {
		t.Errorf("reloaded List() has %d accounts, want %d", got, n)
	}

	for _, id := range ids {
		if _, err := storage.LoadToken(id); err != nil {
			t.Errorf("token for %v missing: %v", id, err)
		}
	}
}
