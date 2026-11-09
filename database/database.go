package database

import (
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"InquiryExplorer/file"

	"github.com/glebarez/sqlite"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CompressBinary compresses data using DEFLATE.
func CompressBinary(data []byte) ([]byte, error) {
	var compressed bytes.Buffer
	w, err := flate.NewWriter(&compressed, flate.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return compressed.Bytes(), nil
}

// DecompressBinary decompresses a DEFLATE payload.
func DecompressBinary(data []byte) ([]byte, error) {
	r := flate.NewReader(bytes.NewReader(data))
	defer r.Close()
	return io.ReadAll(r)
}

// SaveBinary compresses data and stores it in DBHistory, deduplicating by SHA-256.
func SaveBinary(db *gorm.DB, data []byte) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database is nil")
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	var existing DBHistory
	if err := db.Where("sha256 = ? AND size = ?", hash, len(data)).First(&existing).Error; err == nil {
		return existing.HistoryULID, nil
	} else if err != gorm.ErrRecordNotFound {
		return "", err
	}
	compressed, err := CompressBinary(data)
	if err != nil {
		return "", err
	}
	history := DBHistory{HistoryULID: ulid.Make().String(), SHA256: hash, Size: int64(len(data)), Content: compressed}
	if err := db.Create(&history).Error; err != nil {
		return "", fmt.Errorf("failed to save binary data: %w", err)
	}
	return history.HistoryULID, nil
}

// LoadBinary retrieves and decompresses a stored history payload.
func LoadBinary(db *gorm.DB, historyULID string) ([]byte, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	var history DBHistory
	if err := db.Where(&DBHistory{HistoryULID: historyULID}).First(&history).Error; err != nil {
		return nil, err
	}
	data, err := DecompressBinary(history.Content)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != history.Size || fmt.Sprintf("%x", sha256.Sum256(data)) != history.SHA256 {
		return nil, fmt.Errorf("stored binary data integrity check failed")
	}
	return data, nil
}

// GetDatabasePath returns the SQLite database path.
func databasePath(name string) (string, error) {
	dir, err := file.DatabaseFolderPath()
	if err != nil {
		return "", err
	}

	if err := file.MakeDatabaseFolder(); err != nil {
		return "", err
	}

	return filepath.Join(dir, name), nil
}

// OpenSQLite opens a SQLite database using Pure Go SQLite.
func OpenSQLite(name string) (*gorm.DB, error) {
	path, err := databasePath(name)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping sqlite: %w", err)
	}

	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return db, nil
}

// AutoMigrate creates or updates database tables.
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}

	return db.AutoMigrate(
		&DBFile{},
		&DBFolder{},
		&FileGroup{},
		&DBFileHistory{},
		&DBHistory{},
		&DBFileTag{},
		&DBTag{},
	)
}

// InitDB initializes SQLite and migrates tables.
func InitDB(name string) (*gorm.DB, error) {
	db, err := OpenSQLite(name)
	if err != nil {
		return nil, err
	}

	if err := AutoMigrate(db); err != nil {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

// SaveDesktopFiles saves desktop file metadata recursively.
func SaveDesktopFiles(db *gorm.DB, folder *file.FolderInfo) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}

	if folder == nil {
		return fmt.Errorf("folder is nil")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&DBFile{}).Error; err != nil {
			return fmt.Errorf("failed to clear cached files: %w", err)
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&DBFolder{}).Error; err != nil {
			return fmt.Errorf("failed to clear cached folders: %w", err)
		}
		return saveFolderFiles(tx, folder)
	})
}

func saveFolderFiles(tx *gorm.DB, folder *file.FolderInfo) error {
	modTime := time.Now()
	if folderInfo, err := os.Stat(folder.Path); err == nil {
		modTime = folderInfo.ModTime()
	}
	dbFolder := DBFolder{Path: folder.Path, Name: folder.Name, FolderULID: ulid.Make().String(), ParentPath: filepath.Dir(folder.Path), ModTime: modTime}
	if err := tx.Create(&dbFolder).Error; err != nil {
		return fmt.Errorf("failed to save folder %s: %w", folder.Path, err)
	}
	for _, f := range folder.Files {
		modTime, err := time.ParseInLocation(
			"2006-01-02 15:04:05",
			f.ModTime,
			time.Local,
		)
		if err != nil {
			return fmt.Errorf("invalid mod time for %s: %w", f.Path, err)
		}

		dbFile := DBFile{
			Path:      f.Path,
			FileName:  f.Name,
			Size:      f.Size,
			BaseName:  f.BaseName,
			Extension: f.Extension,
			ModTime:   modTime,
			FileULID:  ulid.Make().String(),
		}

		err = tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "path"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"file_name",
				"size",
				"base_name",
				"extension",
				"mod_time",
				"updated_at",
			}),
		}).Create(&dbFile).Error

		if err != nil {
			return fmt.Errorf("failed to save file %s: %w", f.Path, err)
		}
		var savedFile DBFile
		if err := tx.Where("path = ?", f.Path).First(&savedFile).Error; err != nil {
			return fmt.Errorf("failed to load saved file %s: %w", f.Path, err)
		}
		if err := tx.Create(&FileGroup{FolderULID: dbFolder.FolderULID, FileULID: savedFile.FileULID}).Error; err != nil {
			return fmt.Errorf("failed to link file %s to folder: %w", f.Path, err)
		}
	}

	for i := range folder.Folders {
		if err := saveFolderFiles(tx, &folder.Folders[i]); err != nil {
			return err
		}
	}

	return nil
}

// GetAllFiles retrieves all files.
func GetAllFiles(db *gorm.DB) ([]DBFile, error) {
	var files []DBFile

	err := db.Order("path ASC").Find(&files).Error
	if err != nil {
		return nil, err
	}

	return files, nil
}

// GetDesktopFiles retrieves the cached desktop tree.
func GetDesktopFiles(db *gorm.DB) (*file.FolderInfo, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}

	var files []DBFile

	if err := db.Order("path ASC").Find(&files).Error; err != nil {
		return nil, err
	}

	var folders []DBFolder
	if err := db.Order("path ASC").Find(&folders).Error; err != nil {
		return nil, err
	}
	if len(folders) == 0 {
		return nil, nil
	}
	folderMap := make(map[string]*file.FolderInfo, len(folders))
	for _, f := range folders {
		folderMap[filepath.Clean(f.Path)] = &file.FolderInfo{Name: f.Name, Path: f.Path, IsDir: true, Folders: make([]file.FolderInfo, 0), Files: make([]file.FileInfo, 0)}
	}
	var root *file.FolderInfo
	for _, f := range folders {
		current := folderMap[filepath.Clean(f.Path)]
		if parent := folderMap[filepath.Clean(f.ParentPath)]; parent != nil {
			parent.Folders = append(parent.Folders, *current)
		} else {
			root = current
		}
	}
	for _, f := range files {
		if parent := folderMap[filepath.Clean(filepath.Dir(f.Path))]; parent != nil {
			parent.Files = append(parent.Files, file.FileInfo{Path: f.Path, Name: f.FileName, Size: f.Size, BaseName: f.BaseName, Extension: f.Extension, ReadableSize: formatSize(f.Size), ModTime: f.ModTime.Format("2006-01-02 15:04:05")})
		}
	}
	return root, nil
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// CloseDB closes the underlying SQLite connection.
func CloseDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
