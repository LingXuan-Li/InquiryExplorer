package function

import (
	"InquiryExplorer/file"
	"InquiryExplorer/scan"
	"path/filepath"
)

type App struct{}

// Functions exposed to JavaScript.
func (a *App) Greet(name string) string {
	if name == "" {
		name = "Guest"
	}
	return "Hello, " + name + "!"
}

func (a *App) GetStorageInfo() ([]scan.StorageInfo, error) {
	storage, err := scan.GetStorageInfo()
	if err != nil {
		return nil, err
	}
	return storage, nil
}

func (a *App) GetTemplatePage(page string) string {
	appDir, err := file.ConfigFolderPath()
	if err != nil {
		return ""
	}

	templatePath := filepath.Join(appDir, "data", "templates", "pages", page+".html")

	return templatePath
}
