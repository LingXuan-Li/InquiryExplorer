package database

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"InquiryExplorer/file"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openDesktopTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func testFile(dir, name string) file.FileInfo {
	ext := filepath.Ext(name)
	return file.FileInfo{
		Name:      name,
		Path:      filepath.Join(dir, name),
		BaseName:  strings.TrimSuffix(name, ext),
		Extension: ext,
		Size:      10,
		ModTime:   "2026-01-02 03:04:05",
	}
}

// testTree builds root/{a.txt, sub/{b.txt, deep/c.txt}}. b.txt is optional.
func testTree(root string, withB bool) *file.FolderInfo {
	sub := filepath.Join(root, "sub")
	deep := filepath.Join(sub, "deep")
	subFiles := []file.FileInfo{}
	if withB {
		subFiles = append(subFiles, testFile(sub, "b.txt"))
	}
	return &file.FolderInfo{
		Name:  filepath.Base(root),
		Path:  root,
		IsDir: true,
		Files: []file.FileInfo{testFile(root, "a.txt")},
		Folders: []file.FolderInfo{{
			Name:  "sub",
			Path:  sub,
			IsDir: true,
			Files: subFiles,
			Folders: []file.FolderInfo{{
				Name:    "deep",
				Path:    deep,
				IsDir:   true,
				Files:   []file.FileInfo{testFile(deep, "c.txt")},
				Folders: []file.FolderInfo{},
			}},
		}},
	}
}

func countRows(t *testing.T, db *gorm.DB, model interface{}, unscoped bool) int64 {
	t.Helper()
	var n int64
	q := db.Model(model)
	if unscoped {
		q = q.Unscoped()
	}
	if err := q.Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestSaveDesktopFilesRefreshKeepsRowsAndTags(t *testing.T) {
	db := openDesktopTestDB(t)
	root := filepath.Join(t.TempDir(), "Desktop")

	if err := SaveDesktopFiles(db, testTree(root, true)); err != nil {
		t.Fatalf("first save: %v", err)
	}

	var before DBFile
	if err := db.Where("path = ?", filepath.Join(root, "a.txt")).First(&before).Error; err != nil {
		t.Fatalf("load a.txt: %v", err)
	}
	tag := DBTag{Name: "keep-me"}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := db.Create(&DBFileTag{FileULID: before.FileULID, TagID: tag.ID}).Error; err != nil {
		t.Fatalf("tag file: %v", err)
	}

	// Refresh several times: rows must not be wiped, duplicated or hidden.
	for i := 0; i < 3; i++ {
		if err := SaveDesktopFiles(db, testTree(root, true)); err != nil {
			t.Fatalf("refresh %d: %v", i, err)
		}
	}

	if got := countRows(t, db, &DBFile{}, true); got != 3 {
		t.Fatalf("file rows = %d, want 3", got)
	}
	if got := countRows(t, db, &DBFolder{}, false); got != 3 {
		t.Fatalf("folder rows = %d, want 3", got)
	}
	if got := countRows(t, db, &FileGroup{}, false); got != 3 {
		t.Fatalf("link rows = %d, want 3", got)
	}

	var after DBFile
	if err := db.Where("path = ?", before.Path).First(&after).Error; err != nil {
		t.Fatalf("reload a.txt: %v", err)
	}
	if after.FileULID != before.FileULID {
		t.Fatalf("FileULID changed: %s -> %s", before.FileULID, after.FileULID)
	}
	if got := countRows(t, db, &DBFileTag{}, false); got != 1 {
		t.Fatalf("tag links = %d, want 1", got)
	}
}

func TestSaveDesktopFilesSoftDeletesMissingAndRestores(t *testing.T) {
	db := openDesktopTestDB(t)
	root := filepath.Join(t.TempDir(), "Desktop")
	bPath := filepath.Join(root, "sub", "b.txt")

	if err := SaveDesktopFiles(db, testTree(root, true)); err != nil {
		t.Fatalf("save: %v", err)
	}
	var original DBFile
	if err := db.Where("path = ?", bPath).First(&original).Error; err != nil {
		t.Fatalf("load b.txt: %v", err)
	}

	if err := SaveDesktopFiles(db, testTree(root, false)); err != nil {
		t.Fatalf("save without b: %v", err)
	}
	if got := countRows(t, db, &DBFile{}, false); got != 2 {
		t.Fatalf("visible files = %d, want 2", got)
	}
	if got := countRows(t, db, &DBFile{}, true); got != 3 {
		t.Fatalf("total files = %d, want 3 (b.txt soft-deleted)", got)
	}

	if err := SaveDesktopFiles(db, testTree(root, true)); err != nil {
		t.Fatalf("save with b again: %v", err)
	}
	var restored DBFile
	if err := db.Where("path = ?", bPath).First(&restored).Error; err != nil {
		t.Fatalf("b.txt not restored: %v", err)
	}
	if restored.FileULID != original.FileULID {
		t.Fatalf("restored FileULID = %s, want %s", restored.FileULID, original.FileULID)
	}
}

func TestSaveDesktopFilesLeavesOtherRootsAlone(t *testing.T) {
	db := openDesktopTestDB(t)
	base := t.TempDir()
	other := &file.FolderInfo{
		Name:    "Other",
		Path:    filepath.Join(base, "Other"),
		IsDir:   true,
		Files:   []file.FileInfo{testFile(filepath.Join(base, "Other"), "keep.txt")},
		Folders: []file.FolderInfo{},
	}
	if err := SaveDesktopFiles(db, other); err != nil {
		t.Fatalf("save other: %v", err)
	}
	if err := SaveDesktopFiles(db, testTree(filepath.Join(base, "Desktop"), true)); err != nil {
		t.Fatalf("save desktop: %v", err)
	}

	var n int64
	if err := db.Model(&DBFile{}).Where("path = ?", filepath.Join(base, "Other", "keep.txt")).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("file outside the scanned root was removed")
	}
}

func TestGetDesktopFilesRebuildsNestedTree(t *testing.T) {
	db := openDesktopTestDB(t)
	root := filepath.Join(t.TempDir(), "Desktop")

	if got, err := GetDesktopFiles(db); err != nil || got != nil {
		t.Fatalf("empty cache = %v, %v; want nil, nil", got, err)
	}
	if err := SaveDesktopFiles(db, testTree(root, true)); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := GetDesktopFiles(db)
	if err != nil || got == nil {
		t.Fatalf("GetDesktopFiles() = %v, %v", got, err)
	}
	if got.Path != root || len(got.Files) != 1 || len(got.Folders) != 1 {
		t.Fatalf("unexpected root: %+v", got)
	}
	sub := got.Folders[0]
	if len(sub.Files) != 1 || sub.Files[0].Name != "b.txt" || len(sub.Folders) != 1 {
		t.Fatalf("unexpected sub folder: %+v", sub)
	}
	deep := sub.Folders[0]
	if len(deep.Files) != 1 || deep.Files[0].Name != "c.txt" {
		t.Fatalf("unexpected deep folder: %+v", deep)
	}
}

func TestLoadBinaryRejectsEmptyID(t *testing.T) {
	db := openBinaryTestDB(t)
	if _, err := SaveBinary(db, bytes.Repeat([]byte("x"), 32)); err != nil {
		t.Fatalf("SaveBinary() error = %v", err)
	}
	if _, err := LoadBinary(db, ""); err == nil {
		t.Fatal("LoadBinary(\"\") succeeded; want an error")
	}
}
