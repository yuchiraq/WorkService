package storage

import (
	"os"
	"path/filepath"
	"testing"

	"project/internal/models"
	"project/internal/persistence"
)

func TestConstructionFailedWriteDoesNotPublish(t *testing.T) {
	oldBackend, oldDocs := backend, constructionDocuments
	t.Cleanup(func() { backend, constructionDocuments = oldBackend, oldDocs })
	constructionDocuments = nil
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	backend = persistence.OpenJSON(path)
	_, err := SaveConstructionDocument("object", "", 0, models.ConstructionContent{Kind: "act", Fields: map[string]string{}}, "admin", "Admin", "")
	if err == nil {
		t.Fatal("write should fail")
	}
	if len(GetConstructionDocuments("object")) != 0 {
		t.Fatal("failed save published")
	}
}
