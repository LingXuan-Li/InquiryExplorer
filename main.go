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
	"InquiryExplorer/database"
	"InquiryExplorer/file"
	"InquiryExplorer/function"
	"InquiryExplorer/splash"

	"github.com/crgimenes/glaze"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func init() {
	// The GUI must stay on the main thread, especially on macOS.
	runtime.LockOSThread()
}

// appResources holds values prepared during initialization for the main window.
type appResources struct {
	root           string
	indexHTML      []byte
	accountManager *account.Manager
	db             *gorm.DB
}

func main() {
	os.Exit(run())
}

func run() int {
	// ===== Startup =====

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel initialization if the window is closed with the close button.
	s, err := splash.New(cancel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "splash:", err)
		return 1
	}

	if err := s.StartLogging(); err != nil {
		// The splash screen can continue even if log integration fails.
		fmt.Fprintln(os.Stderr, "start logging:", err)
	}

	type initResult struct {
		res *appResources
		err error
	}
	resultCh := make(chan initResult, 1)

	// Initialize in a separate goroutine so the main thread can display the splash screen.
	go func() {
		res, err := initialize(ctx)

		if err != nil {
			log.Printf("Initialization failed: %v", err)
			time.Sleep(2 * time.Second) // Keep the error visible long enough to read.
		} else {
			log.Println("Startup complete")
			time.Sleep(300 * time.Millisecond) // Allow the final log message to appear.
		}

		resultCh <- initResult{res: res, err: err}
		s.Close() // Unblock s.Run() on the main thread.
	}()

	// Block the main thread while displaying the splash screen.
	s.Run()

	// ===== Shutdown =====

	// Exit the application if the window was closed with the close button.
	if ctx.Err() != nil {
		return 0
	}

	r := <-resultCh
	if r.err != nil {
		fmt.Fprintln(os.Stderr, "initialization failed:", r.err)
		return 1
	}

	// Initialization succeeded; start the main window on the main thread.
	if err := runMainWindow(r.res); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return 0
}

// initialize performs setup in the background.
// It returns errors instead of calling log.Fatal so the splash screen can display them.
func initialize(ctx context.Context) (*appResources, error) {
	res := &appResources{}

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Loading environment settings (.env)", func() error {
			if err := godotenv.Load(); err != nil {
				return fmt.Errorf("load .env: %w", err)
			}
			return nil
		}},
		{"Creating configuration directory", file.MakeConfigFolder},
		{"Downloading templates", file.DownloadSetupFolder},
		{"Creating OAuth directory", file.MakeOAuthFolder},
		{"Preparing templates", func() error {
			templ, err := file.TemplateFolderPath()
			if err != nil {
				return err
			}

			// Resolve the template directory to an absolute path.
			root, err := filepath.Abs(templ)
			if err != nil {
				return err
			}
			res.root = root
			log.Println("Template root:", root)

			// Parse index.html as a Go template.
			tmpl, err := template.ParseFiles(filepath.Join(root, "index.html"))
			if err != nil {
				return err
			}

			// Render the HTML template.
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, nil); err != nil {
				return err
			}
			res.indexHTML = append([]byte(nil), buf.Bytes()...)

			return nil
		}},
		{"Loading account information", func() error {
			m, err := account.NewManager()
			if err != nil {
				return err
			}
			res.accountManager = m
			return nil
		}},
		{"Loading account information", func() error {
			m, err := account.NewManager()
			if err != nil {
				return err
			}
			res.accountManager = m
			return nil
		}},

		{"Initializing SQLite database", func() error {
			db, err := database.InitDB("inquiry_explorer.db")
			if err != nil {
				return err
			}

			res.db = db
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

// runMainWindow creates and runs the main window. It must be called on the main thread.
func runMainWindow(res *appResources) error {
	defer func() {
		if err := database.CloseDB(res.db); err != nil {
			log.Println("Database close error:", err)
		}
	}()
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

	// Bind Go methods to JavaScript.
	app := &function.App{
		AccountManager: res.accountManager,
		DB:             res.db,
	}

	bound, err := glaze.BindMethods(w, "app", app)
	if err != nil {
		return fmt.Errorf("bind methods: %w", err)
	}

	log.Printf("Registered methods: %#v", bound)
	log.Println("Glaze BindMethods completed")

	// Open index.html.
	w.Navigate("inquiry://templates/index.html")

	w.Run()

	return nil
}

// newSchemeHandler returns a handler for requests made through the custom URL scheme.
func newSchemeHandler(root string, indexHTML []byte) glaze.SchemeHandler {
	return func(req *glaze.SchemeRequest) *glaze.SchemeResponse {
		// Parse the URL.
		parsedURL, err := url.Parse(req.URL)
		if err != nil {
			log.Println("URL parse error:", err)
			return nil
		}

		// Validate the scheme and host.
		if (parsedURL.Scheme != "inquiry" && parsedURL.Scheme != "app") ||
			parsedURL.Host != "templates" {
			return nil
		}

		// Get the file path from the URL.
		relativePath := filepath.Clean(
			filepath.FromSlash(
				strings.TrimPrefix(parsedURL.Path, "/"),
			),
		)

		log.Println("Requested URL:", req.URL)
		log.Println("Relative path:", relativePath)

		// Prevent path traversal.
		if filepath.IsAbs(relativePath) ||
			relativePath == ".." ||
			strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
			return nil
		}

		// Return the rendered index.html.
		if relativePath == "index.html" {
			return &glaze.SchemeResponse{
				Body:     indexHTML,
				MIMEType: "text/html; charset=utf-8",
			}
		}

		// Resolve the full path.
		fullPath := filepath.Join(root, relativePath)

		// Prevent access outside the root directory.
		rel, err := filepath.Rel(root, fullPath)
		if err != nil ||
			rel == ".." ||
			strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return nil
		}

		log.Println("Full path:", fullPath)

		// Read the file.
		data, err := os.ReadFile(fullPath)
		if err != nil {
			log.Printf("File not found: %s", fullPath)
			return nil
		}

		// Determine the MIME type.
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
