package account

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAccount_JSONRoundTrip(t *testing.T) {
	original := Account{
		ID:          uuid.New(),
		Email:       "user@example.com",
		DisplayName: "Test User",
		Vendor:      "google",
		CreatedAt:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Account
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != original.ID ||
		decoded.Email != original.Email ||
		decoded.DisplayName != original.DisplayName ||
		decoded.Vendor != original.Vendor ||
		!decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("round trip mismatch:\n got  %+v\n want %+v", decoded, original)
	}
}

func TestAccount_JSONFieldNames(t *testing.T) {
	acc := Account{
		ID:          uuid.New(),
		Email:       "user@example.com",
		DisplayName: "Test User",
		Vendor:      "google",
		CreatedAt:   time.Now(),
	}

	data, err := json.Marshal(acc)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"id", "email", "display_name", "vendor", "created_at"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("JSON key %q is missing: %s", key, data)
		}
	}

	if len(raw) != 5 {
		t.Errorf("unexpected number of JSON keys: %d (%s)", len(raw), data)
	}
}

func TestAccount_UnmarshalInvalidUUID(t *testing.T) {
	var acc Account
	err := json.Unmarshal([]byte(`{"id":"not-a-uuid"}`), &acc)
	if err == nil {
		t.Fatal("expected an error for invalid UUID")
	}
}

func TestAccount_ZeroValue(t *testing.T) {
	var acc Account

	if acc.ID != uuid.Nil {
		t.Errorf("zero ID = %v, want uuid.Nil", acc.ID)
	}
	if !acc.CreatedAt.IsZero() {
		t.Error("zero CreatedAt should be zero time")
	}
}
