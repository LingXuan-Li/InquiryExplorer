package cloud

import (
	"errors"
	"testing"

	"golang.org/x/oauth2/google"
)

func TestGoogleConfig_MissingClientID(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")

	cfg, err := GoogleConfig()

	if !errors.Is(err, ErrGoogleClientIDMissing) {
		t.Fatalf("err = %v, want %v", err, ErrGoogleClientIDMissing)
	}
	if cfg != nil {
		t.Errorf("cfg = %+v, want nil", cfg)
	}
}

func TestGoogleConfig_Success(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")

	cfg, err := GoogleConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}

	if cfg.ClientID != "test-client-id" {
		t.Errorf("ClientID = %q, want %q", cfg.ClientID, "test-client-id")
	}

	if cfg.Endpoint != google.Endpoint {
		t.Errorf("Endpoint = %+v, want %+v", cfg.Endpoint, google.Endpoint)
	}

	if len(cfg.Scopes) != 1 || cfg.Scopes[0] != GoogleDriveReadOnlyScope {
		t.Errorf("Scopes = %v, want [%s]", cfg.Scopes, GoogleDriveReadOnlyScope)
	}
}

func TestGoogleConfig_ReturnsIndependentInstances(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")

	a, err := GoogleConfig()
	if err != nil {
		t.Fatal(err)
	}
	b, err := GoogleConfig()
	if err != nil {
		t.Fatal(err)
	}

	if a == b {
		t.Fatal("GoogleConfig should return a new instance on each call")
	}

	a.Scopes[0] = "modified"
	if b.Scopes[0] != GoogleDriveReadOnlyScope {
		t.Error("modifying one config must not affect another")
	}
}

func TestConstants(t *testing.T) {
	if GoogleProvider != "google" {
		t.Errorf("GoogleProvider = %q, want %q", GoogleProvider, "google")
	}
	if GoogleDriveReadOnlyScope != "https://www.googleapis.com/auth/drive.readonly" {
		t.Errorf("unexpected scope: %q", GoogleDriveReadOnlyScope)
	}
}
