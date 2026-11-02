package main

import (
	"bytes"
	"html/template"
	"log"

	"InquiryExplorer/file"

	"github.com/crgimenes/glaze"
)

type App struct{}

// JavaScriptから呼び出す関数
func (a *App) Greet(name string) string {
	if name == "" {
		name = "ゲスト"
	}
	return "こんにちは、" + name + "さん！"
}

func main() {
	err := file.MakeConfigFolder()
	if err != nil {
		log.Fatal(err)
	}

	err = file.MakeDataFolder()
	if err != nil {
		log.Fatal(err)
	}

	w, err := glaze.New(true)

	if err != nil {
		log.Fatal(err)
	}
	defer w.Destroy()

	w.SetTitle("Glaze Desktop App")
	w.SetSize(900, 650, glaze.HintNone)

	app := &App{}

	// GoのメソッドをJavaScriptに登録
	_, err = glaze.BindMethods(w, "app", app)
	if err != nil {
		log.Printf("BindMethods error: %v", err)
		return
	}

	// HTMLテンプレート読み込み
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		log.Fatal(err)
	}

	// テンプレートをHTML文字列に変換
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	if err != nil {
		log.Fatal(err)
	}

	// HTMLをGlazeに設定
	w.SetHtml(buf.String())

	w.Run()
}
