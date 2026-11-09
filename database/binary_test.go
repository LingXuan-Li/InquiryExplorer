package database

import (
	"bytes"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openBinaryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&DBHistory{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestCompressBinaryRoundTrip(t *testing.T) {
	original := bytes.Repeat([]byte("inquiry explorer binary payload\x00"), 128)
	compressed, err := CompressBinary(original)
	if err != nil {
		t.Fatalf("CompressBinary() error = %v", err)
	}
	if len(compressed) >= len(original) {
		t.Fatalf("compressed payload not smaller: %d >= %d", len(compressed), len(original))
	}
	got, err := DecompressBinary(compressed)
	if err != nil {
		t.Fatalf("DecompressBinary() error = %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatal("decompressed payload differs from original")
	}
}

func TestSaveBinaryAndLoadBinary(t *testing.T) {
	db := openBinaryTestDB(t)
	original := []byte{0, 1, 2, 3, 0xff, 0x80}
	id, err := SaveBinary(db, original)
	if err != nil {
		t.Fatalf("SaveBinary() error = %v", err)
	}
	got, err := LoadBinary(db, id)
	if err != nil {
		t.Fatalf("LoadBinary() error = %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("LoadBinary() = %v, want %v", got, original)
	}
	again, err := SaveBinary(db, original)
	if err != nil {
		t.Fatalf("second SaveBinary() error = %v", err)
	}
	if again != id {
		t.Fatalf("duplicate content id = %q, want %q", again, id)
	}
}
