package main

import (
	"html/template"
	"log"
	"path/filepath"
	"strings"

	"project/internal/router"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load initial data
	if err := storage.LoadUsers(); err != nil {
		log.Fatalf("Failed to load users: %v", err)
	}
	if err := storage.LoadWorkers(); err != nil {
		log.Fatalf("Failed to load workers: %v", err)
	}

	r := gin.Default()

	// --- Custom Template Loading with FuncMap ---
	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
	}

	templates, err := loadTemplates("web/templates", funcMap)
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}
	r.SetHTMLTemplate(templates)
	// --- End Custom Template Loading ---

	// Setup all routes from the router package
	router.SetupRouter(r)

	log.Println("Starting HTTP server on port 8099")
	if err := r.Run(":8099"); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

// loadTemplates initializes a new template system that can handle layouts.
func loadTemplates(templatesDir string, funcMap template.FuncMap) (*template.Template, error) {
	templates := template.New("").Funcs(funcMap)

	// Parse layout files separately
	layouts, err := filepath.Glob(filepath.Join(templatesDir, "layout.html"))
	if err != nil {
		return nil, err
	}

	// Parse all other template files (including includes like sidebar)
	pages, err := filepath.Glob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		return nil, err
	}

	// Combine all files for parsing. The layout must be the first one.
	files := append(layouts, pages...)

	// Parse all files. The name of the template will be the base name of the file.
	templates, err = templates.ParseFiles(files...)
	if err != nil {
		return nil, err
	}

	return templates, nil
}
