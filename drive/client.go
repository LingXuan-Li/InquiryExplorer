package drive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"InquiryExplorer/account"
	"InquiryExplorer/cloud"
	"InquiryExplorer/storage"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	googleDrive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

var (
	ErrInvalidAccount = errors.New("invalid Google account")
	ErrTokenMissing   = errors.New("account token not found")
)

// persistentTokenSource saves refreshed tokens to storage.
type persistentTokenSource struct {
	mu        sync.Mutex
	accountID uuid.UUID
	source    oauth2.TokenSource
	lastToken *oauth2.Token
}

// Token returns a valid token and persists it when changed.
func (s *persistentTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	token, err := s.source.Token()
	if err != nil {
		return nil, err
	}

	if !sameToken(s.lastToken, token) {
		if err := storage.SaveToken(s.accountID, token); err != nil {
			return nil, fmt.Errorf("failed to save refreshed token: %w", err)
		}

		s.lastToken = token
	}

	return token, nil
}

// sameToken compares the relevant OAuth token fields.
func sameToken(a, b *oauth2.Token) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.AccessToken == b.AccessToken &&
		a.RefreshToken == b.RefreshToken &&
		a.TokenType == b.TokenType &&
		a.Expiry.Equal(b.Expiry)
}

// NewClient creates a Google Drive API client for an account.
func NewClient(
	ctx context.Context,
	acc account.Account,
) (*googleDrive.Service, error) {
	if acc.ID == uuid.Nil || acc.Vendor != cloud.GoogleProvider {
		return nil, ErrInvalidAccount
	}

	// Load the saved OAuth token.
	token, err := storage.LoadToken(acc.ID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrTokenMissing
		}

		return nil, fmt.Errorf("failed to load token: %w", err)
	}

	// Load Google OAuth configuration.
	oauthConfig, err := cloud.GoogleConfig()
	if err != nil {
		return nil, err
	}

	// Create a token source with automatic refresh.
	source := oauthConfig.TokenSource(ctx, token)

	persistentSource := &persistentTokenSource{
		accountID: acc.ID,
		source:    source,
		lastToken: token,
	}

	// Create Google Drive API service.
	service, err := googleDrive.NewService(
		ctx,
		option.WithTokenSource(persistentSource),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Drive client: %w", err)
	}

	return service, nil
}
