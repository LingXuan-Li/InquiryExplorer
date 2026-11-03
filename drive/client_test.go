package drive

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"InquiryExplorer/account"
	"InquiryExplorer/cloud"
	"InquiryExplorer/storage"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

// setupEnv redirects every per-user directory to a temporary directory so
// that tests never touch the real OAuth folder / token storage.
//
// NOTE: This assumes the storage package and file.OAuthFolderPath() resolve
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

func newTestToken(access string, expiry time.Time) *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  access,
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       expiry,
	}
}

// fakeTokenSource returns the configured tokens in order
// (the last one is repeated once the list is exhausted).
type fakeTokenSource struct {
	tokens []*oauth2.Token
	err    error
	calls  int
}

func (f *fakeTokenSource) Token() (*oauth2.Token, error) {
	f.calls++

	if f.err != nil {
		return nil, f.err
	}

	idx := f.calls - 1
	if idx >= len(f.tokens) {
		idx = len(f.tokens) - 1
	}
	return f.tokens[idx], nil
}

func TestSameToken(t *testing.T) {
	expiry := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	base := newTestToken("access", expiry)

	tests := []struct {
		name string
		a, b *oauth2.Token
		want bool
	}{
		{"both nil", nil, nil, true},
		{"a nil", nil, base, false},
		{"b nil", base, nil, false},
		{"identical", base, newTestToken("access", expiry), true},
		{
			"different access token",
			base,
			newTestToken("other", expiry),
			false,
		},
		{
			"different refresh token",
			base,
			&oauth2.Token{AccessToken: "access", RefreshToken: "other", TokenType: "Bearer", Expiry: expiry},
			false,
		},
		{
			"different token type",
			base,
			&oauth2.Token{AccessToken: "access", RefreshToken: "refresh-token", TokenType: "MAC", Expiry: expiry},
			false,
		},
		{
			"different expiry",
			base,
			newTestToken("access", expiry.Add(time.Second)),
			false,
		},
		{
			"same instant in a different location",
			base,
			newTestToken("access", expiry.In(time.FixedZone("JST", 9*60*60))),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameToken(tt.a, tt.b); got != tt.want {
				t.Errorf("sameToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPersistentTokenSource_UnchangedTokenIsNotSaved(t *testing.T) {
	setupEnv(t)

	id := uuid.New()
	expiry := time.Now().Add(time.Hour)
	initial := newTestToken("access", expiry)

	src := &persistentTokenSource{
		accountID: id,
		source:    &fakeTokenSource{tokens: []*oauth2.Token{newTestToken("access", expiry)}},
		lastToken: initial,
	}

	got, err := src.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got.AccessToken != "access" {
		t.Errorf("AccessToken = %q", got.AccessToken)
	}

	if _, err := storage.LoadToken(id); err == nil {
		t.Error("an unchanged token must not be written to storage")
	}
}

func TestPersistentTokenSource_ChangedTokenIsSavedOnce(t *testing.T) {
	setupEnv(t)

	id := uuid.New()
	expiry := time.Now().Add(time.Hour)

	refreshed := newTestToken("new-access", expiry)

	src := &persistentTokenSource{
		accountID: id,
		source:    &fakeTokenSource{tokens: []*oauth2.Token{refreshed}},
		lastToken: newTestToken("old-access", expiry.Add(-time.Hour)),
	}

	got, err := src.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got.AccessToken != "new-access" {
		t.Errorf("AccessToken = %q, want new-access", got.AccessToken)
	}

	saved, err := storage.LoadToken(id)
	if err != nil {
		t.Fatalf("refreshed token was not persisted: %v", err)
	}
	if saved.AccessToken != "new-access" {
		t.Errorf("saved AccessToken = %q, want new-access", saved.AccessToken)
	}

	if !sameToken(src.lastToken, refreshed) {
		t.Error("lastToken should be updated after saving")
	}

	// A second call returning the same token must not write again.
	if err := storage.DeleteToken(id); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}
	if _, err := src.Token(); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.LoadToken(id); err == nil {
		t.Error("the same token must not be saved twice")
	}
}

func TestPersistentTokenSource_SourceError(t *testing.T) {
	setupEnv(t)

	id := uuid.New()
	sourceErr := errors.New("refresh failed")

	src := &persistentTokenSource{
		accountID: id,
		source:    &fakeTokenSource{err: sourceErr},
		lastToken: newTestToken("old", time.Now()),
	}

	token, err := src.Token()
	if !errors.Is(err, sourceErr) {
		t.Fatalf("err = %v, want %v", err, sourceErr)
	}
	if token != nil {
		t.Errorf("token = %+v, want nil", token)
	}
	if _, err := storage.LoadToken(id); err == nil {
		t.Error("nothing should be saved when the source fails")
	}
}

// Verifies the real refresh flow: an expired token is refreshed through an
// OAuth2 token endpoint and the result is written back to storage.
func TestPersistentTokenSource_PersistsRefreshedToken(t *testing.T) {
	setupEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != "refresh_token" {
			t.Errorf("grant_type = %q, want refresh_token", got)
		}
		if got := r.Form.Get("refresh_token"); got != "refresh-token" {
			t.Errorf("refresh_token = %q, want refresh-token", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"refreshed-access","token_type":"Bearer","expires_in":3600}`))
	}))
	defer srv.Close()

	cfg := &oauth2.Config{
		ClientID: "test-client-id",
		Endpoint: oauth2.Endpoint{TokenURL: srv.URL},
	}

	id := uuid.New()
	expired := newTestToken("expired-access", time.Now().Add(-time.Hour))

	src := &persistentTokenSource{
		accountID: id,
		source:    cfg.TokenSource(context.Background(), expired),
		lastToken: expired,
	}

	got, err := src.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got.AccessToken != "refreshed-access" {
		t.Errorf("AccessToken = %q, want refreshed-access", got.AccessToken)
	}

	saved, err := storage.LoadToken(id)
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if saved.AccessToken != "refreshed-access" {
		t.Errorf("saved AccessToken = %q", saved.AccessToken)
	}
	if saved.RefreshToken != "refresh-token" {
		t.Errorf("refresh token should be retained, got %q", saved.RefreshToken)
	}
}

func TestNewClient_InvalidAccount(t *testing.T) {
	tests := []struct {
		name string
		acc  account.Account
	}{
		{"nil ID", account.Account{Vendor: cloud.GoogleProvider}},
		{"non-google vendor", account.Account{ID: uuid.New(), Vendor: "dropbox"}},
		{"empty vendor", account.Account{ID: uuid.New()}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewClient(context.Background(), tt.acc)

			if !errors.Is(err, ErrInvalidAccount) {
				t.Fatalf("err = %v, want %v", err, ErrInvalidAccount)
			}
			if svc != nil {
				t.Error("service should be nil")
			}
		})
	}
}

func TestNewClient_TokenMissing(t *testing.T) {
	setupEnv(t)
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")

	acc := account.Account{ID: uuid.New(), Vendor: cloud.GoogleProvider}

	// Requires storage.LoadToken to wrap os.ErrNotExist for a missing token.
	svc, err := NewClient(context.Background(), acc)
	if !errors.Is(err, ErrTokenMissing) {
		t.Fatalf("err = %v, want %v", err, ErrTokenMissing)
	}
	if svc != nil {
		t.Error("service should be nil")
	}
}

func TestNewClient_ClientIDMissing(t *testing.T) {
	setupEnv(t)
	t.Setenv("GOOGLE_CLIENT_ID", "")

	acc := account.Account{ID: uuid.New(), Vendor: cloud.GoogleProvider}
	if err := storage.SaveToken(acc.ID, newTestToken("access", time.Now().Add(time.Hour))); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	svc, err := NewClient(context.Background(), acc)
	if !errors.Is(err, cloud.ErrGoogleClientIDMissing) {
		t.Fatalf("err = %v, want %v", err, cloud.ErrGoogleClientIDMissing)
	}
	if svc != nil {
		t.Error("service should be nil")
	}
}

func TestNewClient_Success(t *testing.T) {
	setupEnv(t)
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")

	acc := account.Account{ID: uuid.New(), Vendor: cloud.GoogleProvider}
	if err := storage.SaveToken(acc.ID, newTestToken("access", time.Now().Add(time.Hour))); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	svc, err := NewClient(context.Background(), acc)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if svc == nil {
		t.Fatal("service is nil")
	}
}
