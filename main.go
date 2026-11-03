package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"InquiryExplorer/account"
	"InquiryExplorer/file"
	"InquiryExplorer/function"
	"InquiryExplorer/splash"

	"github.com/crgimenes/glaze"
	"github.com/joho/godotenv"
)

func init() {
	// GUI はメインスレッド固定が必須(特に macOS)
	runtime.LockOSThread()
}

// appResources は初期化処理で用意し、本体ウィンドウで使うもの。
type appResources struct {
	root           string
	indexHTML      []byte
	accountManager *account.Manager
}

func main() {
	os.Exit(run())
}

func run() int {
	// ===== 起動部分 =====

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ×ボタンで閉じられたら ctx をキャンセルして初期化を中断する。
	s, err := splash.New(cancel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "splash:", err)
		return 1
	}

	if err := s.StartLogging(); err != nil {
		// ログ連携に失敗してもスプラッシュ自体は続行できる
		fmt.Fprintln(os.Stderr, "start logging:", err)
	}

	type initResult struct {
		res *appResources
		err error
	}
	resultCh := make(chan initResult, 1)

	// 初期化は別 goroutine。メインスレッドはスプラッシュ表示に専念する。
	go func() {
		res, err := initialize(ctx)

		if err != nil {
			log.Printf("初期化に失敗しました: %v", err)
			time.Sleep(2 * time.Second) // エラーを読めるように残す
		} else {
			log.Println("起動完了")
			time.Sleep(300 * time.Millisecond) // 最終ログの反映待ち
		}

		resultCh <- initResult{res: res, err: err}
		s.Close() // メインスレッドの s.Run() を戻す
	}()

	// メインスレッドをブロックしてスプラッシュを表示
	s.Run()

	// ===== 終了部分 =====

	// ×ボタンで閉じられた場合はアプリごと終了
	if ctx.Err() != nil {
		return 0
	}

	r := <-resultCh
	if r.err != nil {
		fmt.Fprintln(os.Stderr, "initialization failed:", r.err)
		return 1
	}

	// 初期化成功: 本体ウィンドウを起動(メインスレッド)
	if err := runMainWindow(r.res); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return 0
}

// initialize はバックグラウンドで行う初期化処理。
// log.Fatal は使わず、エラーを返してスプラッシュ側で表示する。
func initialize(ctx context.Context) (*appResources, error) {
	res := &appResources{}

	steps := []struct {
		name string
		fn   func() error
	}{
		{"環境設定(.env)を読み込み中", func() error {
			if err := godotenv.Load(); err != nil {
				return fmt.Errorf("load .env: %w", err)
			}
			return nil
		}},
		{"設定フォルダを作成中", file.MakeConfigFolder},
		{"テンプレートをダウンロード中", file.DownloadSetupFolder},
		{"OAuth フォルダを作成中", file.MakeOAuthFolder},
		{"テンプレートを準備中", func() error {
			templ, err := file.TemplateFolderPath()
			if err != nil {
				return err
			}

			// テンプレートディレクトリを絶対パスに解決
			root, err := filepath.Abs(templ)
			if err != nil {
				return err
			}
			res.root = root
			log.Println("Template root:", root)

			// index.html を Go テンプレートとして解析
			tmpl, err := template.ParseFiles(filepath.Join(root, "index.html"))
			if err != nil {
				return err
			}

			// HTML テンプレートを描画
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, nil); err != nil {
				return err
			}
			res.indexHTML = append([]byte(nil), buf.Bytes()...)

			return nil
		}},
		{"アカウント情報を読み込み中", func() error {
			m, err := account.NewManager()
			if err != nil {
				return err
			}
			res.accountManager = m
			return nil
		}},
	}

	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		log.Println(step.name + "...")

		if err := step.fn(); err != nil {
			return nil, fmt.Errorf("%s: %w", step.name, err)
		}
	}

	return res, nil
}

// runMainWindow は本体ウィンドウを作成して実行する(メインスレッド専用)。
func runMainWindow(res *appResources) error {
	w, err := glaze.NewWithOptions(glaze.Options{
		Debug: true,

		SchemeHandlers: map[string]glaze.SchemeHandler{
			"inquiry": newSchemeHandler(res.root, res.indexHTML),
		},
	})
	if err != nil {
		return fmt.Errorf("create main window: %w", err)
	}
	defer w.Destroy()

	w.SetTitle("Inquiry Explorer")
	w.SetSize(900, 650, glaze.HintNone)

	// Go のメソッドを JavaScript にバインド
	app := &function.App{
		AccountManager: res.accountManager,
	}

	bound, err := glaze.BindMethods(w, "app", app)
	if err != nil {
		return fmt.Errorf("bind methods: %w", err)
	}

	log.Printf("Registered methods: %#v", bound)
	log.Println("Glaze BindMethods completed")

	// index.html を開く
	w.Navigate("inquiry://templates/index.html")

	w.Run()

	return nil
}

// newSchemeHandler はカスタムスキームのリクエストを処理するハンドラを返す。
func newSchemeHandler(root string, indexHTML []byte) glaze.SchemeHandler {
	return func(req *glaze.SchemeRequest) *glaze.SchemeResponse {
		// URL を解析
		parsedURL, err := url.Parse(req.URL)
		if err != nil {
			log.Println("URL parse error:", err)
			return nil
		}

		// スキームとホストを検証
		if (parsedURL.Scheme != "inquiry" && parsedURL.Scheme != "app") ||
			parsedURL.Host != "templates" {
			return nil
		}

		// URL からファイルパスを取得
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
			strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
			return nil
		}

		// index.html(描画済みのものを返す)
		if relativePath == "index.html" {
			return &glaze.SchemeResponse{
				Body:     indexHTML,
				MIMEType: "text/html; charset=utf-8",
			}
		}

		// フルパスを解決
		fullPath := filepath.Join(root, relativePath)

		// root の外へのアクセスを防止
		rel, err := filepath.Rel(root, fullPath)
		if err != nil ||
			rel == ".." ||
			strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return nil
		}

		log.Println("Full path:", fullPath)

		// ファイルを読み込み
		data, err := os.ReadFile(fullPath)
		if err != nil {
			log.Printf("File not found: %s", fullPath)
			return nil
		}

		// MIME タイプを決定
		ext := filepath.Ext(fullPath)

		contentType := mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		switch ext {
		case ".js":
			contentType = "text/javascript; charset=utf-8"
		case ".css":
			contentType = "text/css; charset=utf-8"
		case ".html":
			contentType = "text/html; charset=utf-8"
		}

		return &glaze.SchemeResponse{
			Body:     data,
			MIMEType: contentType,
		}
	}
}
