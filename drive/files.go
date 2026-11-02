package drive

import (
	"context"
	"errors"
	"fmt"

	googleDrive "google.golang.org/api/drive/v3"
)

// DriveFile represents a Google Drive file.
type DriveFile struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	MimeType     string   `json:"mime_type"`
	Size         int64    `json:"size"`
	ModifiedTime string   `json:"modified_time"`
	WebViewLink  string   `json:"web_view_link"`
	Parents      []string `json:"parents"`
	Trashed      bool     `json:"trashed"`
}

// StorageInfo represents Google account storage information.
// All values are measured in bytes.
type StorageInfo struct {
	Limit             *int64 `json:"limit,omitempty"`
	Usage             *int64 `json:"usage,omitempty"`
	UsageInDrive      *int64 `json:"usage_in_drive,omitempty"`
	UsageInDriveTrash *int64 `json:"usage_in_drive_trash,omitempty"`
	FreeBytes         *int64 `json:"free_bytes,omitempty"`
	Unlimited         bool   `json:"unlimited"`
}

// DriveOverview represents a complete Drive overview.
type DriveOverview struct {
	Files   []DriveFile `json:"files"`
	Storage StorageInfo `json:"storage"`
}

var (
	ErrInvalidService   = errors.New("invalid Google Drive service")
	ErrQuotaUnavailable = errors.New("storage quota information unavailable")
)

// ListFiles retrieves all non-trashed files from Google Drive.
func ListFiles(
	ctx context.Context,
	service *googleDrive.Service,
) ([]DriveFile, error) {
	if service == nil {
		return nil, ErrInvalidService
	}

	files := make([]DriveFile, 0)

	pageToken := ""

	for {
		call := service.Files.List().
			Q("trashed = false").
			PageSize(1000).
			Fields(
				"nextPageToken",
				"files(id,name,mimeType,size,modifiedTime,webViewLink,parents,trashed)",
			)

		if pageToken != "" {
			call.PageToken(pageToken)
		}

		result, err := call.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list Drive files: %w", err)
		}

		for _, file := range result.Files {
			item := DriveFile{
				ID:           file.Id,
				Name:         file.Name,
				MimeType:     file.MimeType,
				ModifiedTime: file.ModifiedTime,
				WebViewLink:  file.WebViewLink,
				Parents:      file.Parents,
				Trashed:      file.Trashed,
			}

			if file.Size != 0 {
				item.Size = file.Size
			}

			files = append(files, item)
		}

		pageToken = result.NextPageToken

		if pageToken == "" {
			break
		}
	}

	return files, nil
}

// GetStorageInfo retrieves Google account storage information.
func GetStorageInfo(
	ctx context.Context,
	service *googleDrive.Service,
) (*StorageInfo, error) {
	if service == nil {
		return nil, ErrInvalidService
	}

	about, err := service.About.Get().
		Fields("storageQuota(limit,usage,usageInDrive,usageInDriveTrash)").
		Context(ctx).
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve storage information: %w", err)
	}

	if about.StorageQuota == nil {
		return nil, ErrQuotaUnavailable
	}

	quota := about.StorageQuota

	info := &StorageInfo{
		Usage:             int64Pointer(quota.Usage),
		UsageInDrive:      int64Pointer(quota.UsageInDrive),
		UsageInDriveTrash: int64Pointer(quota.UsageInDriveTrash),
	}

	// A missing limit is represented as zero by the generated Go client.
	// Google Drive does not provide a finite quota limit in this case.
	if quota.Limit <= 0 {
		info.Unlimited = true
		return info, nil
	}

	info.Limit = int64Pointer(quota.Limit)

	free := quota.Limit - quota.Usage
	if free < 0 {
		free = 0
	}

	info.FreeBytes = int64Pointer(free)

	return info, nil
}

// GetDriveOverview retrieves files and storage information.
func GetDriveOverview(
	ctx context.Context,
	service *googleDrive.Service,
) (*DriveOverview, error) {
	files, err := ListFiles(ctx, service)
	if err != nil {
		return nil, err
	}

	storage, err := GetStorageInfo(ctx, service)
	if err != nil {
		return nil, err
	}

	return &DriveOverview{
		Files:   files,
		Storage: *storage,
	}, nil
}

// int64Pointer returns a pointer to an int64 value.
func int64Pointer(value int64) *int64 {
	return &value
}
