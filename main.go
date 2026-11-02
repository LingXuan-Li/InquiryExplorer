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

	"InquiryExplorer/account"
	"InquiryExplorer/file"
	"InquiryExplorer/function"

	"github.com/crgimenes/glaze"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Failed to load .env:", err)
	}

	// Create the configuration directory.
	if err := file.MakeConfigFolder(); err != nil {
		log.Fatal(err)
	}

	// Create the configuration directory.
	if err := file.MakeConfigFolder(); err != nil {
		log.Fatal(err)
	}

	// Download the template directory.
	if err := file.DownloadSetupFolder(); err != nil {
		log.Fatal(err)
	}

	if err := file.MakeOAuthFolder(); err != nil {
		log.Fatal(err)
	}

	templ, err := file.TemplateFolderPath()
	if err != nil {
		log.Fatal(err)
	}

	// Resolve the template directory to an absolute path.
	root, err := filepath.Abs(templ)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Template root:", root)

	// Parse index.html as a Go template.
	tmpl, err := template.ParseFiles(
		filepath.Join(root, "index.html"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Render the HTML template.
	var buf bytes.Buffer

	if err := tmpl.Execute(&buf, nil); err != nil {
		log.Fatal(err)
	}

	indexHTML := append([]byte(nil), buf.Bytes()...)

	// Register a custom URL scheme with Glaze.
	w, err := glaze.NewWithOptions(glaze.Options{
		Debug: true,

		SchemeHandlers: map[string]glaze.SchemeHandler{
			"app": func(req *glaze.SchemeRequest) *glaze.SchemeResponse {

				// Parse the URL.
				parsedURL, err := url.Parse(req.URL)
				if err != nil {
					log.Println("URL parse error:", err)
					return nil
				}

				// Validate the app scheme and host.
				if parsedURL.Scheme != "app" ||
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

				// Resolve the full file path.
				fullPath := filepath.Join(root, relativePath)

				// Prevent access outside the root directory.
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

				// Read the file.
				data, err := os.ReadFile(fullPath)
				if err != nil {
					log.Printf("File not found: %s", fullPath)
					return nil
				}

				// Determine the MIME type.
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

	// Bind Go methods to JavaScript.
	// Initialize account manager.
	accountManager, err := account.NewManager()
	if err != nil {
		log.Fatal(err)
	}

	// Bind Go methods to JavaScript.
	app := &function.App{
		AccountManager: accountManager,
	}

	bound, err := glaze.BindMethods(w, "app", app)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Registered methods: %#v", bound)
	log.Println("Glaze BindMethods completed")

	log.Println("Registered methods:", bound)

	// Open index.html.
	w.Navigate("app://templates/index.html")

	w.Run()
}
