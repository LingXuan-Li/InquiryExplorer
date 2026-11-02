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
	"time"
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

func OAuthFolderPath() (string, error) {
	appDir, err := ConfigFolderPath()
	if err != nil {
		return "", err
	}

	oauthDir := filepath.Join(appDir, "oauth")
	return oauthDir, nil
}

func MakeOAuthFolder() error {
	oauthDir, err := OAuthFolderPath()
	if err != nil {
		return err
	}

	return os.MkdirAll(oauthDir, 0755)
}

// LatestCommitHash returns the latest commit SHA affecting the setup directory on GitHub.
func LatestCommitHash() (string, error) {
	url := "https://api.github.com/repos/LingXuan-Li/InquiryExplorer/commits?sha=main&path=setup&per_page=1"

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	var commits []struct {
		SHA string `json:"sha"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return "", err
	}

	if len(commits) == 0 || len(commits[0].SHA) != 40 {
		return "", fmt.Errorf("invalid commit SHA")
	}

	return commits[0].SHA, nil
}

// LocalCommitHash returns the commit SHA saved locally.
func LocalCommitHash() (string, error) {
	appDir, err := ConfigFolderPath()
	if err != nil {
		return "", err
	}

	path := filepath.Join(appDir, "setup_version.txt")

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	hash := strings.TrimSpace(string(b))

	if hash != "" && len(hash) != 40 {
		return "", fmt.Errorf("invalid local commit SHA")
	}

	return hash, nil
}

// SaveCommitHash saves a commit SHA locally.
func SaveCommitHash(hash string) error {
	if len(hash) != 40 {
		return fmt.Errorf("invalid commit SHA")
	}

	appDir, err := ConfigFolderPath()
	if err != nil {
		return err
	}

	path := filepath.Join(appDir, "setup_version.txt")

	return os.WriteFile(path, []byte(hash), 0644)
}

func DownloadSetupFolder() error {
	appDir, err := ConfigFolderPath()
	if err != nil {
		return err
	}

	dataDir := filepath.Join(appDir, "data")
	tempDir := filepath.Join(appDir, "data_temp")
	backupDir := filepath.Join(appDir, "data_backup")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	// Recover if the previous directory swap was interrupted.
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if _, backupErr := os.Stat(backupDir); backupErr == nil {
			if err := os.Rename(backupDir, dataDir); err != nil {
				return err
			}
		}
	}

	// Remove any leftover temporary directory.
	if err := os.RemoveAll(tempDir); err != nil {
		return err
	}

	// Get the latest commit SHA.
	latestHash, err := LatestCommitHash()
	if err != nil {
		return err
	}

	// Get the locally saved commit SHA.
	localHash, err := LocalCommitHash()
	if err != nil {
		return err
	}

	// Skip the download if the local setup is already up to date.
	if latestHash == localHash {
		fmt.Println("Setup is already up to date:", latestHash)
		return nil
	}

	fmt.Println("Updating setup:")
	fmt.Println("Local :", localHash)
	fmt.Println("Remote:", latestHash)

	// Download the directory recursively.
	download := func(src, dst string) error {
		return nil
	}

	var recursiveDownload func(string, string) error

	recursiveDownload = func(src, dst string) error {
		apiURL := fmt.Sprintf(
			"https://api.github.com/repos/LingXuan-Li/InquiryExplorer/contents/%s?ref=%s",
			src,
			latestHash,
		)

		client := &http.Client{
			Timeout: 60 * time.Second,
		}

		resp, err := client.Get(apiURL)
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
			relativePath := strings.TrimPrefix(
				strings.TrimPrefix(f.Path, src),
				"/",
			)

			if relativePath == "" ||
				filepath.IsAbs(relativePath) ||
				filepath.Clean(relativePath) == ".." ||
				strings.HasPrefix(
					filepath.Clean(relativePath),
					".."+string(filepath.Separator),
				) {
				return fmt.Errorf("invalid file path: %s", f.Path)
			}

			localPath := filepath.Join(dst, relativePath)

			switch f.Type {
			case "dir":
				if err := recursiveDownload(f.Path, localPath); err != nil {
					return err
				}

			case "file":
				r, err := client.Get(f.DownloadURL)
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

	_ = download

	// Download the latest version to a temporary directory.
	if err := recursiveDownload("setup", tempDir); err != nil {
		os.RemoveAll(tempDir)
		return err
	}

	// Remove the previous backup.
	if err := os.RemoveAll(backupDir); err != nil {
		os.RemoveAll(tempDir)
		return err
	}

	// Back up the existing setup directory.
	if _, err := os.Stat(dataDir); err == nil {
		if err := os.Rename(dataDir, backupDir); err != nil {
			os.RemoveAll(tempDir)
			return err
		}
	} else if !os.IsNotExist(err) {
		os.RemoveAll(tempDir)
		return err
	}

	// Move the temporary directory into place as the active setup.
	if err := os.Rename(tempDir, dataDir); err != nil {
		// Restore the previous directory if the replacement fails.
		if _, backupErr := os.Stat(backupDir); backupErr == nil {
			if restoreErr := os.Rename(backupDir, dataDir); restoreErr != nil {
				return fmt.Errorf(
					"install failed: %v; rollback failed: %v",
					err,
					restoreErr,
				)
			}
		}

		os.RemoveAll(tempDir)
		return err
	}

	// Save the new commit SHA.
	if err := SaveCommitHash(latestHash); err != nil {
		return fmt.Errorf("setup installed but version save failed: %w", err)
	}

	// Remove the backup.
	if err := os.RemoveAll(backupDir); err != nil {
		return err
	}

	fmt.Println("Setup updated successfully:", latestHash)

	return nil
}

func TemplateFolderPath() (string, error) {
	appDir, err := ConfigFolderPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(appDir, "data", "templates"), nil
}
