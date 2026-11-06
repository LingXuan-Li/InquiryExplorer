package file

import (
	"fmt"
	"os"
	"path/filepath"
)

type FolderInfo struct {
	Name    string       `json:"name"`
	Path    string       `json:"path"`
	IsDir   bool         `json:"is_dir"`
	Folders []FolderInfo `json:"folders"`
	Files   []FileInfo   `json:"files"`
}

type FileInfo struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	IsDir        bool   `json:"is_dir"`
	BaseName     string `json:"baseName"`
	Extension    string `json:"extension"`
	Size         int64  `json:"size"`
	ReadableSize string `json:"readable_size"`
	Mode         string `json:"mode"`
	ModTime      string `json:"modTime"`
}

func GetDesktopInfo() (*FolderInfo, error) {
	desktopPath, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	desktopPath = filepath.Join(desktopPath, "Desktop")

	folderInfo, err := SearchFolder(desktopPath)
	if err != nil {
		return nil, err
	}

	return folderInfo, nil
}

func SearchFolder(path string) (*FolderInfo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("This path is not a directory: %s", absPath)
	}

	folder := &FolderInfo{
		Name:  info.Name(),
		Path:  absPath,
		IsDir: true,
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		entryPath := filepath.Join(absPath, entry.Name())

		if entry.IsDir() {
			subFolder, err := SearchFolder(entryPath)
			if err != nil {
				continue
			}

			folder.Folders = append(folder.Folders, *subFolder)
		} else {
			fileInfo, err := entry.Info()
			if err != nil {
				continue
			}

			extension := filepath.Ext(entry.Name())

			file := FileInfo{
				Name:         entry.Name(),
				Path:         entryPath,
				IsDir:        false,
				BaseName:     entry.Name()[:len(entry.Name())-len(extension)],
				Extension:    extension,
				Size:         fileInfo.Size(),
				ReadableSize: formatSize(fileInfo.Size()),
				Mode:         fileInfo.Mode().String(),
				ModTime:      fileInfo.ModTime().Format("2006-01-02 15:04:05"),
			}

			folder.Files = append(folder.Files, file)
		}
	}

	return folder, nil
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

	return fmt.Sprintf("%.2f %cB",
		float64(size)/float64(div),
		"KMGTPE"[exp])
}
