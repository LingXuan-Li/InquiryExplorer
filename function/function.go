package function

import (
	"InquiryExplorer/scan"
)

type App struct{}

// JavaScriptから呼び出す関数
func (a *App) Greet(name string) string {
	if name == "" {
		name = "ゲスト"
	}
	return "こんにちは、" + name + "さん！"
}

func (a *App) GetStorageInfo() ([]scan.StorageInfo, error) {
	storage, err := scan.GetStorageInfo()
	if err != nil {
		return nil, err
	}
	return storage, nil
}
