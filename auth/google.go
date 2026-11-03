package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"InquiryExplorer/account"
	"InquiryExplorer/cloud"
	"InquiryExplorer/storage"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	googleDrive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

var (
	ErrInvalidState = errors.New("invalid OAuth state")
	ErrAuthTimeout  = errors.New("Google authentication timed out")
)

// authResult represents the OAuth callback result.
type authResult struct {
	Code  string
	Error error
}

// GoogleAuth starts Google OAuth authentication.
func GoogleAuth(
	ctx context.Context,
	manager *account.Manager,
) (*account.Account, error) {
	if manager == nil {
		return nil, errors.New("account manager is nil")
	}

	// Generate a random state value.
	state, err := generateState()
	if err != nil {
		return nil, err
	}

	// Generate a PKCE code verifier.
	verifier := oauth2.GenerateVerifier()

	// Create a local callback listener.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	defer listener.Close()

	redirectURL := fmt.Sprintf(
		"http://127.0.0.1:%d/callback",
		listener.Addr().(*net.TCPAddr).Port,
	)

	// Load Google OAuth configuration.
	baseConfig, err := cloud.GoogleConfig()
	if err != nil {
		return nil, err
	}

	oauthConfig := *baseConfig
	oauthConfig.RedirectURL = redirectURL

	resultCh := make(chan authResult, 1)

	mux := http.NewServeMux()

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		receivedState := r.URL.Query().Get("state")

		if subtle.ConstantTimeCompare(
			[]byte(state),
			[]byte(receivedState),
		) != 1 {
			http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
			return
		}

		if oauthError := r.URL.Query().Get("error"); oauthError != "" {
			resultCh <- authResult{
				Error: fmt.Errorf("Google OAuth error: %s", oauthError),
			}

			fmt.Fprintln(w, "Authentication failed. You can close this window.")
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			resultCh <- authResult{
				Error: errors.New("authorization code is missing"),
			}

			http.Error(w, "Authorization code is missing", http.StatusBadRequest)
			return
		}

		resultCh <- authResult{
			Code: code,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(w, `
			<!doctype html>
			<html lang="en">
			<head>
				<meta charset="utf-8">
				<title>Authentication complete</title>
			</head>
			<body>
				<h2>Authentication complete</h2>
				<p>You can close this window and return to the application.</p>
			</body>
			</html>
		`)
	})

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		_ = server.Serve(listener)
	}()

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		_ = server.Shutdown(shutdownCtx)
	}()

	// Generate the Google authorization URL with PKCE.
	authURL := oauthConfig.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.S256ChallengeOption(verifier),
	)

	// Open the authorization URL in the default browser.
	if err := openBrowser(authURL); err != nil {
		return nil, fmt.Errorf("failed to open browser: %w", err)
	}

	// Wait for the OAuth callback.
	authCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	var result authResult

	select {
	case result = <-resultCh:
	case <-authCtx.Done():
		if errors.Is(authCtx.Err(), context.DeadlineExceeded) {
			return nil, ErrAuthTimeout
		}
		return nil, authCtx.Err()
	}

	if result.Error != nil {
		return nil, result.Error
	}

	// Exchange the authorization code for a token using PKCE.
	token, err := oauthConfig.Exchange(
		authCtx,
		result.Code,
		oauth2.VerifierOption(verifier),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}

	// Retrieve the Google account information.
	client := oauthConfig.Client(authCtx, token)

	driveService, err := googleDrive.NewService(
		authCtx,
		option.WithHTTPClient(client),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Drive service: %w", err)
	}

	about, err := driveService.About.Get().
		Fields("user(emailAddress,displayName)").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve Google account: %w", err)
	}

	if about.User == nil || about.User.EmailAddress == "" {
		return nil, errors.New("Google account information is unavailable")
	}

	// Check whether this Google account is already registered.
	for _, existing := range manager.List() {
		if existing.Vendor == cloud.GoogleProvider &&
			existing.Email == about.User.EmailAddress {

			// Preserve the previous refresh token if Google
			// did not return a new one.
			if token.RefreshToken == "" {
				oldToken, loadErr := storage.LoadToken(existing.ID)
				if loadErr == nil {
					token.RefreshToken = oldToken.RefreshToken
				}
			}

			if err := storage.SaveToken(existing.ID, token); err != nil {
				return nil, fmt.Errorf("failed to update token: %w", err)
			}

			return &existing, nil
		}
	}

	// Register a new Google account.
	newAccount := account.Account{
		ID:          uuid.New(),
		Email:       about.User.EmailAddress,
		DisplayName: about.User.DisplayName,
		Vendor:      cloud.GoogleProvider,
		CreatedAt:   time.Now(),
	}

	if err := manager.Add(newAccount, token); err != nil {
		return nil, fmt.Errorf("failed to register account: %w", err)
	}

	return &newAccount, nil
}

// generateState generates a cryptographically random OAuth state.
func generateState() (string, error) {
	randomBytes := make([]byte, 32)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

// openBrowser opens a URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command(
			"rundll32",
			"url.dll,FileProtocolHandler",
			url,
		)

	case "darwin":
		cmd = exec.Command("open", url)

	case "linux":
		cmd = exec.Command("xdg-open", url)

	default:
		return fmt.Errorf(
			"unsupported operating system: %s",
			runtime.GOOS,
		)
	}

	return cmd.Start()
}
