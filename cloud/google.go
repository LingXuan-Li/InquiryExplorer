package cloud

import (
	"errors"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	GoogleProvider = "google"

	GoogleDriveReadOnlyScope = "https://www.googleapis.com/auth/drive.readonly"
)

var (
	ErrGoogleClientIDMissing     = errors.New("google client ID is not configured")
	ErrGoogleClientSecretMissing = errors.New("google client secret is not configured")
)

// GoogleConfig returns the Google OAuth2 configuration.
func GoogleConfig() (*oauth2.Config, error) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" {
		return nil, ErrGoogleClientIDMissing
	}

	if clientSecret == "" {
		return nil, ErrGoogleClientSecretMissing
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes: []string{
			GoogleDriveReadOnlyScope,
		},
	}, nil
}
