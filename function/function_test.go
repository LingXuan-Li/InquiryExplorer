package function

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"InquiryExplorer/database"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openFunctionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestStoreFileBinary(t *testing.T) {
	db := openFunctionTestDB(t)
	path := filepath.Join(t.TempDir(), "payload.bin")
	want := []byte("file contents\x00with binary data\xff")
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	app := &App{DB: db}
	id, err := app.StoreFileBinary(path)
	if err != nil {
		t.Fatalf("StoreFileBinary() error = %v", err)
	}
	got, err := database.LoadBinary(db, id)
	if err != nil {
		t.Fatalf("LoadBinary() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("stored content = %v, want %v", got, want)
	}
}
