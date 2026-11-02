package scan

import (
	"testing"
)

func TestGetStorageInfo(t *testing.T) {
	storage, err := GetStorageInfo()

	if err != nil {
		t.Fatalf("Failed to get storage information: %v", err)
	}

	if len(storage) == 0 {
		t.Fatal("No storage information was returned")
	}

	for _, s := range storage {
		t.Logf("Device: %s", s.Device)
		t.Logf("Mountpoint: %s", s.Mountpoint)
		t.Logf("Filesystem: %s", s.Filesystem)
		t.Logf("Total: %d", s.Total)
		t.Logf("Used: %d", s.Used)
		t.Logf("Free: %d", s.Free)
		t.Logf("UsedPercent: %.2f%%", s.UsedPercent)

		if s.Mountpoint == "" {
			t.Error("Mountpoint is empty")
		}

		if s.Total == 0 {
			t.Errorf("Total is zero: %s", s.Device)
		}

		if s.Used > s.Total {
			t.Errorf("Used exceeds Total: %s", s.Device)
		}

		if s.Free > s.Total {
			t.Errorf("Free exceeds Total: %s", s.Device)
		}

		if s.UsedPercent < 0 || s.UsedPercent > 100 {
			t.Errorf("UsedPercent is out of range: %.2f", s.UsedPercent)
		}
	}
}
