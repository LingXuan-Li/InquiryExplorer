package file

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSearchFolder(t *testing.T) {
	// テスト用ディレクトリを作成
	root := t.TempDir()

	// サブディレクトリを作成
	subDir := filepath.Join(root, "subfolder")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	nestedDir := filepath.Join(subDir, "nested")
	if err := os.Mkdir(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}

	// テスト用ファイルを作成
	files := map[string]string{
		"test.txt":              "Hello World",
		"image.png":             "image data",
		"subfolder/a.go":        "package main",
		"subfolder/nested/b.md": "markdown",
	}

	for name, content := range files {
		path := filepath.Join(root, name)

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// SearchFolderを実行
	result, err := SearchFolder(root)
	if err != nil {
		t.Fatalf("SearchFolder() error = %v", err)
	}

	// フォルダ情報を検証
	if result.Name != filepath.Base(root) {
		t.Errorf("Name = %q, want %q", result.Name, filepath.Base(root))
	}

	if result.Path != root {
		t.Errorf("Path = %q, want %q", result.Path, root)
	}

	if !result.IsDir {
		t.Error("IsDir = false, want true")
	}

	// ルート直下のファイル数
	if len(result.Files) != 2 {
		t.Errorf("Files count = %d, want 2", len(result.Files))
	}

	// サブフォルダ数
	if len(result.Folders) != 1 {
		t.Fatalf("Folders count = %d, want 1", len(result.Folders))
	}

	// サブフォルダの検証
	subFolder := result.Folders[0]

	if subFolder.Name != "subfolder" {
		t.Errorf("Subfolder Name = %q, want %q", subFolder.Name, "subfolder")
	}

	if len(subFolder.Files) != 1 {
		t.Errorf("Subfolder Files count = %d, want 1", len(subFolder.Files))
	}

	// ネストされたフォルダの検証
	if len(subFolder.Folders) != 1 {
		t.Fatalf("Nested Folders count = %d, want 1", len(subFolder.Folders))
	}

	nestedFolder := subFolder.Folders[0]

	if nestedFolder.Name != "nested" {
		t.Errorf("Nested Folder Name = %q, want %q", nestedFolder.Name, "nested")
	}

	if len(nestedFolder.Files) != 1 {
		t.Errorf("Nested Files count = %d, want 1", len(nestedFolder.Files))
	}
}

func TestSearchFolderFileInfo(t *testing.T) {
	root := t.TempDir()

	content := "Hello World"
	filePath := filepath.Join(root, "hello.txt")

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := SearchFolder(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("Files count = %d, want 1", len(result.Files))
	}

	file := result.Files[0]

	if file.Name != "hello.txt" {
		t.Errorf("Name = %q, want hello.txt", file.Name)
	}

	if file.BaseName != "hello" {
		t.Errorf("BaseName = %q, want hello", file.BaseName)
	}

	if file.Extension != ".txt" {
		t.Errorf("Extension = %q, want .txt", file.Extension)
	}

	if file.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", file.Size, len(content))
	}

	if file.ReadableSize != "11 B" {
		t.Errorf("ReadableSize = %q, want %q", file.ReadableSize, "11 B")
	}

	if file.IsDir {
		t.Error("IsDir = true, want false")
	}

	if file.Path != filePath {
		t.Errorf("Path = %q, want %q", file.Path, filePath)
	}

	if file.Mode == "" {
		t.Error("Mode is empty")
	}

	if file.ModTime == "" {
		t.Error("ModTime is empty")
	}

	// 更新日時のフォーマットを検証
	_, err = time.Parse("2006-01-02 15:04:05", file.ModTime)
	if err != nil {
		t.Errorf("Invalid ModTime format: %v", err)
	}
}

func TestSearchFolderInvalidPath(t *testing.T) {
	t.Run("Nonexistent path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "not-exist")

		_, err := SearchFolder(path)

		if err == nil {
			t.Error("Expected error for nonexistent path")
		}
	})

	t.Run("File instead of directory", func(t *testing.T) {
		root := t.TempDir()
		filePath := filepath.Join(root, "test.txt")

		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := SearchFolder(filePath)

		if err == nil {
			t.Fatal("Expected error for file path")
		}

		if !strings.Contains(err.Error(), "not a directory") {
			t.Errorf("Unexpected error message: %v", err)
		}
	})
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name string
		size int64
		want string
	}{
		{
			name: "Zero bytes",
			size: 0,
			want: "0 B",
		},
		{
			name: "Bytes",
			size: 100,
			want: "100 B",
		},
		{
			name: "Less than KB",
			size: 1023,
			want: "1023 B",
		},
		{
			name: "Kilobytes",
			size: 1024,
			want: "1.00 KB",
		},
		{
			name: "Megabytes",
			size: 1024 * 1024,
			want: "1.00 MB",
		},
		{
			name: "Gigabytes",
			size: 1024 * 1024 * 1024,
			want: "1.00 GB",
		},
		{
			name: "Terabytes",
			size: 1024 * 1024 * 1024 * 1024,
			want: "1.00 TB",
		},
		{
			name: "Fractional size",
			size: 1536,
			want: "1.50 KB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSize(tt.size)

			if got != tt.want {
				t.Errorf("formatSize(%d) = %q, want %q",
					tt.size, got, tt.want)
			}
		})
	}
}

func TestGetDesktopInfo(t *testing.T) {
	// 実際のユーザーのDesktopフォルダを参照する
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot get user home directory: %v", err)
	}

	desktop := filepath.Join(home, "Desktop")

	_, err = os.Stat(desktop)
	if os.IsNotExist(err) {
		t.Skip("Desktop directory does not exist")
	}
	if err != nil {
		t.Fatalf("Cannot access Desktop directory: %v", err)
	}

	result, err := GetDesktopInfo()
	if err != nil {
		t.Fatalf("GetDesktopInfo() error = %v", err)
	}

	if result == nil {
		t.Fatal("GetDesktopInfo() returned nil, want FolderInfo")
	}

	if result.Name != filepath.Base(desktop) {
		t.Errorf("Name = %q, want %q",
			result.Name, filepath.Base(desktop))
	}

	if !result.IsDir {
		t.Error("Desktop IsDir = false, want true")
	}

	if result.Folders == nil {
		t.Error("Folders is nil, want initialized slice")
	}

	if result.Files == nil {
		t.Error("Files is nil, want initialized slice")
	}
}
