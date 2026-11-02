package file

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func OSScan() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "linux":
		return "linux"
	case "darwin":
		return "macOS"
	default:
		return "unknown"
	}
}

func ConfigFolderPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(configDir, "InquiryExplorer_Go")
	return appDir, nil
}

func MakeConfigFolder() error {
	appDir, err := ConfigFolderPath()
	if err != nil {
		return err
	}

	return os.MkdirAll(appDir, 0755)
}

func MakeDataFolder() error {
	appDir, err := ConfigFolderPath()
	if err != nil {
		return err
	}
	dataDir := filepath.Join(appDir, "data")

	return os.MkdirAll(dataDir, 0755)
}

func DownloadSetupFolder() error {
	appDir, err := ConfigFolderPath()
	if err != nil {
		return err
	}

	var download func(string, string) error

	download = func(src, dst string) error {
		resp, err := http.Get(fmt.Sprintf(
			"https://api.github.com/repos/LingXuan-Li/InquiryExplorer/contents/%s?ref=main",
			src,
		))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GitHub API error: %s", resp.Status)
		}

		var files []struct {
			Path        string `json:"path"`
			Type        string `json:"type"`
			DownloadURL string `json:"download_url"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
			return err
		}

		for _, f := range files {
			localPath := filepath.Join(
				dst,
				strings.TrimPrefix(strings.TrimPrefix(f.Path, src), "/"),
			)

			if f.Type == "dir" {
				if err := download(f.Path, localPath); err != nil {
					return err
				}
			} else if f.Type == "file" {
				r, err := http.Get(f.DownloadURL)
				if err != nil {
					return err
				}

				if r.StatusCode != http.StatusOK {
					r.Body.Close()
					return fmt.Errorf("download error: %s", r.Status)
				}

				if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
					r.Body.Close()
					return err
				}

				out, err := os.Create(localPath)
				if err != nil {
					r.Body.Close()
					return err
				}

				_, copyErr := io.Copy(out, r.Body)
				closeErr := out.Close()
				r.Body.Close()

				if copyErr != nil {
					return copyErr
				}
				if closeErr != nil {
					return closeErr
				}
			}
		}

		return nil
	}

	return download("setup", filepath.Join(appDir, "data"))
}

func TemplateFolderPath() (string, error) {
	appDir, err := ConfigFolderPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(appDir, "data", "templates"), nil
}
