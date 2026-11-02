package scan

import (
	"testing"
)

func TestGetStorageInfo(t *testing.T) {
	storage, err := GetStorageInfo()

	if err != nil {
		t.Fatalf("ストレージ情報の取得に失敗しました: %v", err)
	}

	if len(storage) == 0 {
		t.Fatal("ストレージ情報が取得できませんでした")
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
			t.Error("Mountpointが空です")
		}

		if s.Total == 0 {
			t.Errorf("Totalが0です: %s", s.Device)
		}

		if s.Used > s.Total {
			t.Errorf("UsedがTotalを超えています: %s", s.Device)
		}

		if s.Free > s.Total {
			t.Errorf("FreeがTotalを超えています: %s", s.Device)
		}

		if s.UsedPercent < 0 || s.UsedPercent > 100 {
			t.Errorf("UsedPercentが不正です: %.2f", s.UsedPercent)
		}
	}
}
