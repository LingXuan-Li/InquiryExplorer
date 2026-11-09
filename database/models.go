package database

import (
	"time"

	"gorm.io/gorm"
)

// DBFile represents a logical file.
type DBFile struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// File metadata
	Path      string    `gorm:"type:text;not null;uniqueIndex" json:"path"`
	FileName  string    `gorm:"type:text;not null" json:"file_name"`
	Size      int64     `gorm:"not null;default:0" json:"size"`
	BaseName  string    `gorm:"type:text;not null" json:"base_name"`
	Extension string    `gorm:"type:varchar(20);not null;default:''" json:"extension"`
	ModTime   time.Time `gorm:"not null" json:"mod_time"`

	// Public identifier
	FileULID string `gorm:"type:char(26);uniqueIndex;not null" json:"ulid"`

	// Relations
	History []DBFileHistory `gorm:"foreignKey:FileULID;references:FileULID" json:"history,omitempty"`
	Tags    []DBFileTag     `gorm:"foreignKey:FileULID;references:FileULID" json:"tags,omitempty"`
}

// DBFileHistory represents a file version.
//
// Multiple records may reference the same HistoryULID.
// This allows a file to return to a previously existing content.
type DBFileHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	FileULID string `gorm:"type:char(26);not null;index" json:"ulid"`

	HistoryULID string `gorm:"type:char(26);not null;index" json:"history_id"`

	History DBHistory `gorm:"foreignKey:HistoryULID;references:HistoryULID" json:"history,omitempty"`
}

// DBHistory represents unique binary content.
//
// SHA256 is used for candidate lookup.
// Actual content equality must be verified using binary comparison.
type DBHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	HistoryULID string `gorm:"type:char(26);uniqueIndex;not null" json:"history_id"`

	SHA256 string `gorm:"type:char(64);index;not null" json:"sha256"`

	Size int64 `gorm:"not null;default:0" json:"size"`

	// Content is omitted from ordinary JSON responses.
	// It can later be replaced by an external storage reference.
	Content []byte `gorm:"type:blob;not null" json:"-"`

	// Relations
	Files []DBFileHistory `gorm:"foreignKey:HistoryULID;references:HistoryULID" json:"files,omitempty"`
}

// DBFileTag represents the many-to-many relationship
// between files and tags.
type DBFileTag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	FileULID string `gorm:"type:char(26);not null;uniqueIndex:idx_file_tag" json:"ulid"`

	TagID uint `gorm:"not null;uniqueIndex:idx_file_tag;index" json:"tag_id"`

	Tag DBTag `gorm:"foreignKey:TagID;references:ID" json:"tag,omitempty"`
}

// DBTag represents a file tag.
type DBTag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Name        string `gorm:"type:varchar(100);uniqueIndex;not null" json:"name"`
	Description string `gorm:"type:varchar(256);not null;default:''" json:"description"`

	Files []DBFileTag `gorm:"foreignKey:TagID;references:ID" json:"files,omitempty"`
}

type DBScanState struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	RootPath      string    `gorm:"type:varchar(256);not null" json:"root_path"`
	LastScannedAt time.Time `json:"last_scanned_at"`
	IsScanning    bool      `json:"is_scanning"`
}

type DBFolder struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Path       string      `gorm:"type:text;not null;uniqueIndex" json:"path"`
	Name       string      `gorm:"type:text;not null" json:"name"`
	FolderULID string      `gorm:"type:char(26);uniqueIndex;not null" json:"folder_ulid"`
	ParentPath string      `gorm:"type:text;not null" json:"parent_path"`
	ModTime    time.Time   `gorm:"not null" json:"mod_time"`
	Files      []FileGroup `gorm:"foreignKey:FolderULID;references:FolderULID" json:"files,omitempty"`
}

type FileGroup struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	FolderULID string `gorm:"type:char(26);not null;index" json:"folder_ulid"`
	FileULID   string `gorm:"type:char(26);not null;index" json:"file_ulid"`
}
