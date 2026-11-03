package auth

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"InquiryExplorer/account"
)

// TestGenerateState checks whether a valid OAuth state is generated.
func TestGenerateState(t *testing.T) {
	state, err := generateState()

	if err != nil {
		t.Fatalf("generateState() returned an error: %v", err)
	}

	if state == "" {
		t.Fatal("generateState() returned an empty state")
	}

	// Check whether the generated state can be decoded.
	decoded, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		t.Fatalf("generated state is not valid Base64 URL encoding: %v", err)
	}

	// Check whether the random data is 32 bytes.
	if len(decoded) != 32 {
		t.Errorf(
			"expected decoded state length to be 32, got %d",
			len(decoded),
		)
	}

	// Check whether the state contains only URL-safe characters.
	if strings.ContainsAny(state, "+/=") {
		t.Errorf("state contains invalid URL characters: %s", state)
	}
}

// TestGenerateStateUniqueness checks whether different states are generated.
func TestGenerateStateUniqueness(t *testing.T) {
	state1, err := generateState()
	if err != nil {
		t.Fatalf("first generateState() returned an error: %v", err)
	}

	state2, err := generateState()
	if err != nil {
		t.Fatalf("second generateState() returned an error: %v", err)
	}

	if state1 == state2 {
		t.Error("generateState() generated duplicate states")
	}
}

// TestGoogleAuthNilManager checks whether GoogleAuth handles a nil manager.
func TestGoogleAuthNilManager(t *testing.T) {
	ctx := context.Background()

	result, err := GoogleAuth(ctx, nil)

	if err == nil {
		t.Fatal("expected an error when manager is nil")
	}

	if result != nil {
		t.Errorf("expected nil account, got %v", result)
	}

	expectedError := "account manager is nil"

	if err.Error() != expectedError {
		t.Errorf(
			"expected error %q, got %q",
			expectedError,
			err.Error(),
		)
	}
}

// TestGenerateStateFormat checks whether the state is URL-safe.
func TestGenerateStateFormat(t *testing.T) {
	state, err := generateState()

	if err != nil {
		t.Fatalf("generateState() returned an error: %v", err)
	}

	if strings.Contains(state, " ") {
		t.Error("state contains whitespace")
	}

	if strings.Contains(state, "\n") {
		t.Error("state contains newline")
	}
}

// TestGoogleAuthNilManagerWithDifferentContexts
// checks nil manager handling with different contexts.
func TestGoogleAuthNilManagerWithDifferentContexts(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
	}{
		{
			name: "background context",
			ctx:  context.Background(),
		},
		{
			name: "cancelled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GoogleAuth(tt.ctx, (*account.Manager)(nil))

			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			if result != nil {
				t.Errorf("expected nil account, got %v", result)
			}
		})
	}
}
