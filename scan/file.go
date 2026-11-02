package scan

import (
	"os"
	"path/filepath"
	"runtime"
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

	appDir := filepath.Join(configDir, "MyApp")
	return appDir, nil
}

func MakeConfigFolder() error {
	appDir, err := ConfigFolderPath()
	if err != nil {
		return err
	}

	return os.MkdirAll(appDir, 0755)
}
