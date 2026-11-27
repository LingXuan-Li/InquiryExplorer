package database

import (
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	// A struct condition ignores zero values, so an empty ID would silently
	// match (and return) an arbitrary row.
	if historyULID == "" {
		return nil, fmt.Errorf("history ULID is empty")
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
		&DBScanState{},
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

// SaveDesktopFiles synchronizes the cached desktop tree with a fresh scan.
//
// The cache is updated in place instead of being cleared and rebuilt:
//   - rows that still exist keep their ULIDs, so tags and history attached to
//     them survive a refresh;
//   - files that disappeared are soft-deleted (and restored if they come back);
//   - folders that disappeared are removed together with their links;
//   - only rows located below the scanned root are ever touched.
func SaveDesktopFiles(db *gorm.DB, folder *file.FolderInfo) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}

	if folder == nil {
		return fmt.Errorf("folder is nil")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		return syncDesktopTree(tx, folder)
	})
}

type folderFileLink struct {
	folderULID string
	fileULID   string
}

func syncDesktopTree(tx *gorm.DB, root *file.FolderInfo) error {
	rootPath := filepath.Clean(root.Path)
	rootPrefix := rootPath
	if !strings.HasSuffix(rootPrefix, string(filepath.Separator)) {
		rootPrefix += string(filepath.Separator)
	}
	inScope := func(p string) bool {
		p = filepath.Clean(p)
		return p == rootPath || strings.HasPrefix(p, rootPrefix)
	}

	var existingFolders []DBFolder
	if err := tx.Find(&existingFolders).Error; err != nil {
		return fmt.Errorf("failed to load cached folders: %w", err)
	}
	foldersByPath := make(map[string]*DBFolder, len(existingFolders))
	for i := range existingFolders {
		foldersByPath[existingFolders[i].Path] = &existingFolders[i]
	}

	// Unscoped: soft-deleted files still occupy their unique path.
	var existingFiles []DBFile
	if err := tx.Unscoped().Find(&existingFiles).Error; err != nil {
		return fmt.Errorf("failed to load cached files: %w", err)
	}
	filesByPath := make(map[string]*DBFile, len(existingFiles))
	for i := range existingFiles {
		filesByPath[existingFiles[i].Path] = &existingFiles[i]
	}

	var existingLinks []FileGroup
	if err := tx.Find(&existingLinks).Error; err != nil {
		return fmt.Errorf("failed to load cached folder links: %w", err)
	}

	seenFolders := make(map[string]bool)
	seenFiles := make(map[string]bool)
	wanted := make(map[folderFileLink]bool)

	var walk func(f *file.FolderInfo) error
	walk = func(f *file.FolderInfo) error {
		modTime := time.Now()
		if folderInfo, err := os.Stat(f.Path); err == nil {
			modTime = folderInfo.ModTime()
		}

		dbFolder, ok := foldersByPath[f.Path]
		if ok {
			dbFolder.Name = f.Name
			dbFolder.ParentPath = filepath.Dir(f.Path)
			dbFolder.ModTime = modTime
			if err := tx.Omit(clause.Associations).Save(dbFolder).Error; err != nil {
				return fmt.Errorf("failed to update folder %s: %w", f.Path, err)
			}
		} else {
			dbFolder = &DBFolder{
				Path:       f.Path,
				Name:       f.Name,
				FolderULID: ulid.Make().String(),
				ParentPath: filepath.Dir(f.Path),
				ModTime:    modTime,
			}
			if err := tx.Omit(clause.Associations).Create(dbFolder).Error; err != nil {
				return fmt.Errorf("failed to save folder %s: %w", f.Path, err)
			}
			foldersByPath[f.Path] = dbFolder
		}
		seenFolders[f.Path] = true

		for _, fi := range f.Files {
			fileModTime, err := time.ParseInLocation(
				"2006-01-02 15:04:05",
				fi.ModTime,
				time.Local,
			)
			if err != nil {
				return fmt.Errorf("invalid mod time for %s: %w", fi.Path, err)
			}

			dbFile, ok := filesByPath[fi.Path]
			if ok {
				dbFile.FileName = fi.Name
				dbFile.Size = fi.Size
				dbFile.BaseName = fi.BaseName
				dbFile.Extension = fi.Extension
				dbFile.ModTime = fileModTime
				// Restore the file if it had been soft-deleted earlier.
				dbFile.DeletedAt = gorm.DeletedAt{}
				if err := tx.Unscoped().Omit(clause.Associations).Save(dbFile).Error; err != nil {
					return fmt.Errorf("failed to update file %s: %w", fi.Path, err)
				}
			} else {
				dbFile = &DBFile{
					Path:      fi.Path,
					FileName:  fi.Name,
					Size:      fi.Size,
					BaseName:  fi.BaseName,
					Extension: fi.Extension,
					ModTime:   fileModTime,
					FileULID:  ulid.Make().String(),
				}
				if err := tx.Omit(clause.Associations).Create(dbFile).Error; err != nil {
					return fmt.Errorf("failed to save file %s: %w", fi.Path, err)
				}
				filesByPath[fi.Path] = dbFile
			}
			seenFiles[fi.Path] = true
			wanted[folderFileLink{dbFolder.FolderULID, dbFile.FileULID}] = true
		}

		for i := range f.Folders {
			if err := walk(&f.Folders[i]); err != nil {
				return err
			}
		}

		return nil
	}

	if err := walk(root); err != nil {
		return err
	}

	// Folder <-> file links: keep wanted ones, drop stale and duplicate links
	// below the scanned root (and links to folders that no longer exist).
	knownFolderULIDs := make(map[string]bool, len(foldersByPath))
	scopedFolderULIDs := make(map[string]bool)
	for p, f := range foldersByPath {
		knownFolderULIDs[f.FolderULID] = true
		if inScope(p) {
			scopedFolderULIDs[f.FolderULID] = true
		}
	}

	kept := make(map[folderFileLink]bool)
	var staleLinkIDs []uint
	for _, l := range existingLinks {
		key := folderFileLink{l.FolderULID, l.FileULID}
		if wanted[key] && !kept[key] {
			kept[key] = true
			continue
		}
		if scopedFolderULIDs[l.FolderULID] || !knownFolderULIDs[l.FolderULID] {
			staleLinkIDs = append(staleLinkIDs, l.ID)
		}
	}
	if err := deleteByIDs(tx, &FileGroup{}, staleLinkIDs); err != nil {
		return fmt.Errorf("failed to remove stale folder links: %w", err)
	}

	// Folders that no longer exist (their links are already gone).
	var staleFolderIDs []uint
	for p, f := range foldersByPath {
		if inScope(p) && !seenFolders[p] {
			staleFolderIDs = append(staleFolderIDs, f.ID)
		}
	}
	if err := deleteByIDs(tx, &DBFolder{}, staleFolderIDs); err != nil {
		return fmt.Errorf("failed to remove stale folders: %w", err)
	}

	// Files that no longer exist are soft-deleted so that their tags and
	// history are kept in case the file comes back.
	var staleFileIDs []uint
	for p, f := range filesByPath {
		if inScope(p) && !seenFiles[p] && !f.DeletedAt.Valid {
			staleFileIDs = append(staleFileIDs, f.ID)
		}
	}
	if err := deleteByIDs(tx, &DBFile{}, staleFileIDs); err != nil {
		return fmt.Errorf("failed to remove stale files: %w", err)
	}

	var newLinks []FileGroup
	for key := range wanted {
		if !kept[key] {
			newLinks = append(newLinks, FileGroup{FolderULID: key.folderULID, FileULID: key.fileULID})
		}
	}
	if len(newLinks) > 0 {
		if err := tx.CreateInBatches(&newLinks, 200).Error; err != nil {
			return fmt.Errorf("failed to link files to folders: %w", err)
		}
	}

	return nil
}

// deleteByIDs deletes rows by primary key in chunks, staying below SQLite's
// bound-variable limit.
func deleteByIDs(tx *gorm.DB, model interface{}, ids []uint) error {
	const chunk = 500
	for start := 0; start < len(ids); start += chunk {
		end := start + chunk
		if end > len(ids) {
			end = len(ids)
		}
		if err := tx.Delete(model, ids[start:end]).Error; err != nil {
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

	var folders []DBFolder
	if err := db.Order("path ASC").Find(&folders).Error; err != nil {
		return nil, err
	}
	if len(folders) == 0 {
		return nil, nil
	}

	var files []DBFile
	if err := db.Order("path ASC").Find(&files).Error; err != nil {
		return nil, err
	}

	byPath := make(map[string]*DBFolder, len(folders))
	for i := range folders {
		byPath[filepath.Clean(folders[i].Path)] = &folders[i]
	}

	children := make(map[string][]string)
	var roots []string
	for i := range folders {
		key := filepath.Clean(folders[i].Path)
		parent := filepath.Clean(folders[i].ParentPath)
		if _, ok := byPath[parent]; ok && parent != key {
			children[parent] = append(children[parent], key)
		} else {
			roots = append(roots, key)
		}
	}
	if len(roots) == 0 {
		return nil, nil
	}

	filesByFolder := make(map[string][]file.FileInfo)
	for _, f := range files {
		dir := filepath.Clean(filepath.Dir(f.Path))
		if _, ok := byPath[dir]; !ok {
			continue
		}
		filesByFolder[dir] = append(filesByFolder[dir], file.FileInfo{
			Path:         f.Path,
			Name:         f.FileName,
			Size:         f.Size,
			BaseName:     f.BaseName,
			Extension:    f.Extension,
			ReadableSize: formatSize(f.Size),
			ModTime:      f.ModTime.Format("2006-01-02 15:04:05"),
		})
	}

	// Build bottom-up. Children must be complete before they are copied into
	// their parent, otherwise nested folders and their files get lost.
	var build func(key string) file.FolderInfo
	build = func(key string) file.FolderInfo {
		f := byPath[key]
		info := file.FolderInfo{
			Name:    f.Name,
			Path:    f.Path,
			IsDir:   true,
			Folders: make([]file.FolderInfo, 0, len(children[key])),
			Files:   make([]file.FileInfo, 0, len(filesByFolder[key])),
		}
		for _, child := range children[key] {
			info.Folders = append(info.Folders, build(child))
		}
		info.Files = append(info.Files, filesByFolder[key]...)
		return info
	}

	// If several roots are cached (e.g. the root path changed), use the one
	// that was refreshed most recently.
	rootKey := roots[0]
	for _, key := range roots[1:] {
		if byPath[key].UpdatedAt.After(byPath[rootKey].UpdatedAt) {
			rootKey = key
		}
	}

	root := build(rootKey)
	return &root, nil
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
