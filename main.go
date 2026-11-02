package main

import (
	"bytes"
	"html/template"
	"log"
	"path/filepath"

	"InquiryExplorer/file"
	"InquiryExplorer/function"

	"github.com/crgimenes/glaze"
)

func main() {
	err := file.MakeConfigFolder()
	if err != nil {
		log.Fatal(err)
	}

	err = file.DownloadSetupFolder()
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

	app := &function.App{}

	// GoのメソッドをJavaScriptに登録
	_, err = glaze.BindMethods(w, "app", app)
	if err != nil {
		log.Printf("BindMethods error: %v", err)
		return
	}

	// HTMLテンプレート読み込み
	templ, err := file.TemplateFolderPath()
	if err != nil {
		log.Fatal(err)
	}
	tmpl, err := template.ParseFiles(filepath.Join(templ, "index.html"))
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
