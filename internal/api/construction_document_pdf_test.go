package api

import (
	"bytes"
	"strings"
	"testing"

	"project/internal/models"
)

func TestConstructionPDFLongRows(t *testing.T) {
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
