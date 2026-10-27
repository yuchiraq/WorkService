package api

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"project/internal/documentfont"
	"project/internal/models"
)

func TestConstructionPDFLongRows(t *testing.T) {
	// Licensed open fonts stand in for system fonts in this portable renderer test.
	// Production never falls back; the browser preview uses real installed Times.
	dir := t.TempDir()
	for name, b := range map[string][]byte{"times.ttf": documentfont.Regular, "timesbd.ttf": documentfont.Bold, "timesi.ttf": documentfont.Italic} {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("WORKSERVICE_DOCUMENT_FONT_DIR", dir)
	for _, kind := range []string{"act", "journal"} {
		doc := models.ConstructionDocument{ID: "doc", Version: 1, UpdatedAt: "2026-10-26T12:00:00Z", Content: models.ConstructionContent{Kind: kind, Number: "001", Fields: map[string]string{"object_name": strings.Repeat("Объект ", 600), "works": strings.Repeat("Скрытые работы и отметки. ", 200)}}}
		if kind == "journal" {
			delete(doc.Content.Fields, "works")
			doc.Content.Rows = []models.ConstructionRow{{ID: "row", Section: 4, Fields: map[string]string{"date": "2026-10-26", "description": strings.Repeat("Длинное описание измерений и конструкций. ", 120), "control": strings.Repeat("Результат контроля. ", 180)}}}
		}
		pdf, err := buildConstructionPDF(doc)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
			t.Fatal("not PDF")
		}
	}
}
