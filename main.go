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

	// index.htmlの内容を保存
	indexHTML := append([]byte(nil), buf.Bytes()...)

	// GlazeのカスタムURLスキームを登録
	w, err := glaze.NewWithOptions(glaze.Options{
		Debug: true,

		SchemeHandlers: map[string]glaze.SchemeHandler{
			"app": func(req *glaze.SchemeRequest) *glaze.SchemeResponse {

				// URLからファイルパスを取得
				parsedURL, err := url.Parse(req.URL)
				if err != nil {
					return nil
				}

				relativePath := filepath.Join(
					parsedURL.Host,
					strings.TrimPrefix(parsedURL.Path, "/"),
				)

				relativePath = filepath.Clean(relativePath)

				// パストラバーサル対策
				if filepath.IsAbs(relativePath) ||
					relativePath == ".." ||
					strings.HasPrefix(
						relativePath,
						".."+string(os.PathSeparator),
					) {
					return nil
				}

				// index.htmlはGoテンプレートの実行結果を返す
				if relativePath == "templates/index.html" {
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

				// ファイル読み込み
				data, err := os.ReadFile(fullPath)
				if err != nil {
					log.Printf("File not found: %s", fullPath)
					return nil
				}

				// MIMEタイプ取得
				contentType := mime.TypeByExtension(
					filepath.Ext(fullPath),
				)

				if contentType == "" {
					contentType = "application/octet-stream"
				}

				// JavaScriptのMIMEタイプ補正
				if filepath.Ext(fullPath) == ".js" {
					contentType = "text/javascript; charset=utf-8"
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

	w.SetTitle("Glaze Desktop App")
	w.SetSize(900, 650, glaze.HintNone)

	// GoメソッドをJavaScriptに登録
	app := &function.App{}

	bound, err := glaze.BindMethods(w, "app", app)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Registered methods:", bound)

	// index.htmlをapp://から開く
	w.Navigate("app://templates/index.html")

	// アプリ起動
	w.Run()
}
