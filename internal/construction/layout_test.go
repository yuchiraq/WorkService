package construction

import (
	"math"
	"strings"
	"testing"

	"project/internal/models"
)

func TestDocumentLayoutsCoverFields(t *testing.T) {
	for _, kind := range []string{"act", "journal"} {
		layout := ActLayout
		if kind == "journal" {
			layout = CoverLayout
		}
		seen := map[string]int{}
		for _, block := range layout {
			if block.Key != "" {
				seen[block.Key]++
			}
		}
		if kind == "act" {
			for _, s := range Signatories {
				seen[s.Name]++
				seen[s.At]++
			}
		}
		for _, field := range Fields(kind) {
			if seen[field.Key] != 1 {
				t.Errorf("%s: %s appears %d times", kind, field.Key, seen[field.Key])
			}
		}
	}
	for _, section := range JournalSections {
		seen := map[string]bool{}
		width := 0.0
		for _, column := range Columns(section.Number) {
			width += column.Width
			for _, key := range column.Keys {
				seen[key] = true
			}
		}
		if math.Abs(width-257) > .01 {
			t.Errorf("section %d: width %f", section.Number, width)
		}
		for _, field := range section.Fields {
			if field.Type == "hidden" {
				continue
			}
			if !seen[field.Key] {
				t.Errorf("section %d: missing %s", section.Number, field.Key)
			}
		}
	}
}

func TestJournalTitleIsSeparateFromGeneralInformation(t *testing.T) {
	titles, summaries := 0, 0
	for _, block := range JournalTitleLayout {
		if block.Kind == "title" && block.Text == "ЖУРНАЛ" {
			titles++
		}
		if block.Kind == "cover-summary" {
			summaries++
		}
		if block.Key != "" {
			t.Fatal("title must reuse requisites, not introduce independent fields")
		}
	}
	if titles != 1 || summaries != 1 {
		t.Fatal("incomplete outer title")
	}
	for _, block := range CoverLayout {
		if block.Kind == "title" {
			t.Fatal("outer title must not be mixed into section 1")
		}
	}
}

func TestActSigningFields(t *testing.T) {
	for _, value := range []string{"", "2026-10-27T10:30"} {
		if err := Validate(models.ConstructionContent{Kind: "act", Fields: map[string]string{"contractor_signed_at": value}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"tomorrow", "2026-02-30T10:30", "2026-10-27T", "2026-10-27T25:00"} {
		if err := Validate(models.ConstructionContent{Kind: "act", Fields: map[string]string{"contractor_signed_at": value}}); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	if err := Validate(models.ConstructionContent{Kind: "act", Fields: map[string]string{"contractor_signer": strings.Repeat("И", 151)}}); err == nil {
		t.Fatal("accepted oversized signer")
	}
}
