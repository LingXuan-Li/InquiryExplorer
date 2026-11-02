package main

import (
	"bytes"
	"html/template"
	"log"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"InquiryExplorer/file"
	"InquiryExplorer/function"

	"github.com/crgimenes/glaze"
)

func main() {
	// 設定フォルダ作成
	if err := file.MakeConfigFolder(); err != nil {
		log.Fatal(err)
	}

	// テンプレートフォルダ取得・ダウンロード
	if err := file.DownloadSetupFolder(); err != nil {
		log.Fatal(err)
	}

	templ, err := file.TemplateFolderPath()
	if err != nil {
		log.Fatal(err)
	}

	// テンプレートの絶対パス
	root, err := filepath.Abs(templ)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Template root:", root)

	// index.htmlをGoテンプレートとして読み込む
	tmpl, err := template.ParseFiles(
		filepath.Join(root, "index.html"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// HTMLテンプレートを実行
	var buf bytes.Buffer

	if err := tmpl.Execute(&buf, nil); err != nil {
		log.Fatal(err)
	}

	indexHTML := append([]byte(nil), buf.Bytes()...)

	// GlazeのカスタムURLスキーム
	w, err := glaze.NewWithOptions(glaze.Options{
		Debug: true,

		SchemeHandlers: map[string]glaze.SchemeHandler{
			"app": func(req *glaze.SchemeRequest) *glaze.SchemeResponse {

				// URL解析
				parsedURL, err := url.Parse(req.URL)
				if err != nil {
					log.Println("URL parse error:", err)
					return nil
				}

				// appスキームとホストを確認
				if parsedURL.Scheme != "app" ||
					parsedURL.Host != "templates" {
					return nil
				}

				// URLのパスからファイル名を取得
				relativePath := filepath.Clean(
					filepath.FromSlash(
						strings.TrimPrefix(parsedURL.Path, "/"),
					),
				)

				log.Println("Requested URL:", req.URL)
				log.Println("Relative path:", relativePath)

				// パストラバーサル対策
				if filepath.IsAbs(relativePath) ||
					relativePath == ".." ||
					strings.HasPrefix(
						relativePath,
						".."+string(os.PathSeparator),
					) {
					return nil
				}

				// index.html
				if relativePath == "index.html" {
					return &glaze.SchemeResponse{
						Body:     indexHTML,
						MIMEType: "text/html; charset=utf-8",
					}
				}

				// 実際のファイルパス
				fullPath := filepath.Join(root, relativePath)

				// ルートフォルダ外へのアクセスを禁止
				rel, err := filepath.Rel(root, fullPath)
				if err != nil ||
					rel == ".." ||
					strings.HasPrefix(
						rel,
						".."+string(os.PathSeparator),
					) {
					return nil
				}

				log.Println("Full path:", fullPath)

				// ファイル読み込み
				data, err := os.ReadFile(fullPath)
				if err != nil {
					log.Printf("File not found: %s", fullPath)
					return nil
				}

				// MIMEタイプ
				contentType := mime.TypeByExtension(
					filepath.Ext(fullPath),
				)

				if contentType == "" {
					contentType = "application/octet-stream"
				}

				if filepath.Ext(fullPath) == ".js" {
					contentType = "text/javascript; charset=utf-8"
				}

				if filepath.Ext(fullPath) == ".css" {
					contentType = "text/css; charset=utf-8"
				}

				if filepath.Ext(fullPath) == ".html" {
					contentType = "text/html; charset=utf-8"
				}

				return &glaze.SchemeResponse{
					Body:     data,
					MIMEType: contentType,
				}
			},
		},
	})

	if err != nil {
		log.Fatal(err)
	}

	defer w.Destroy()

	w.SetTitle("Inquiry Explorer")
	w.SetSize(900, 650, glaze.HintNone)

	// GoメソッドをJavaScriptに登録
	app := &function.App{}

	bound, err := glaze.BindMethods(w, "app", app)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Registered methods:", bound)

	// index.htmlを開く
	w.Navigate("app://templates/index.html")

	w.Run()
}
