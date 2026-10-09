package storage

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"project/internal/models"
	"project/internal/persistence"
)

func cloudFixture(t *testing.T) string {
	t.Helper()
	oldBackend, oldItems, oldFile, oldQuota, oldLimit := backend, cloudItems, cloudFile, cloudQuota, cloudFileLimit
	t.Cleanup(func() {
		backend, cloudItems, cloudFile, cloudQuota, cloudFileLimit = oldBackend, oldItems, oldFile, oldQuota, oldLimit
	})
	dir := t.TempDir()
	backend = persistence.OpenJSON(dir)
	cloudItems, cloudFile, cloudQuota, cloudFileLimit = nil, filepath.Join(dir, "cloud.json"), 100, 80
	t.Setenv("WORKSERVICE_CLOUD_DIR", filepath.Join(dir, "cloud"))
	t.Setenv("WORKSERVICE_CLOUD_QUOTA_MB", "1")
	t.Setenv("WORKSERVICE_CLOUD_MAX_FILE_MB", "1")
	return dir
}

func cloudFolder(t *testing.T, owner, parent, name string) models.CloudItem {
	t.Helper()
	if err := ChangeCloud(owner, CloudMutation{Action: "folder", ParentID: parent, Name: name}); err != nil {
		t.Fatal(err)
	}
	for _, item := range CloudState(owner).Items {
		if item.ParentID == parent && item.Name == name && item.DeletedAt == "" {
			return item
		}
	}
	t.Fatal("folder missing")
	return models.CloudItem{}
}

func cloudUpload(t *testing.T, owner, parent, name, data string) models.CloudItem {
	t.Helper()
	item, err := AddCloudFile(owner, parent, name, uuid.NewString(), strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func cloudCurrent(t *testing.T, owner, id string) models.CloudItem {
	t.Helper()
	for _, item := range CloudState(owner).Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatal("item missing", id)
	return models.CloudItem{}
}

func cloudChange(t *testing.T, owner, action string, item models.CloudItem) {
	t.Helper()
	if err := ChangeCloud(owner, CloudMutation{Action: action, Items: []CloudRef{{item.ID, item.Version}}}); err != nil {
		t.Fatal(err)
	}
}

func TestCloudOwnershipAndOperations(t *testing.T) {
	cloudFixture(t)
	a := cloudFolder(t, "a", "", "Документы")
	b := cloudFolder(t, "b", "", "Документы")
	file := cloudUpload(t, "a", a.ID, "notes.txt", "original bytes")
	duplicate := cloudUpload(t, "a", a.ID, "notes.txt", "second")
	if duplicate.Name != "notes (2).txt" {
		t.Fatal(duplicate.Name)
	}
	retry, err := AddCloudFile("a", a.ID, file.Name, file.ID, strings.NewReader("MUST NOT REPLACE"))
	if err != nil || retry.ID != file.ID || len(CloudState("a").Items) != 3 {
		t.Fatal("retry", err)
	}
	_, stream, err := OpenCloudFile("a", file.ID)
	if err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(stream)
	_ = stream.Close()
	if string(content) != "original bytes" {
		t.Fatal("original changed")
	}
	if _, _, err = OpenCloudFile("b", file.ID); err != ErrCloudNotFound {
		t.Fatal("other owner opened file", err)
	}
	if _, err = AddCloudFile("b", a.ID, "x", uuid.NewString(), strings.NewReader("x")); err != ErrCloudNotFound {
		t.Fatal("other owner uploaded", err)
	}
	for _, mutation := range []CloudMutation{
		{Action: "rename", Name: "stolen", Items: []CloudRef{{file.ID, file.Version}}},
		{Action: "trash", Items: []CloudRef{{file.ID, file.Version}}},
		{Action: "move", ParentID: b.ID, Items: []CloudRef{{file.ID, file.Version}}},
	} {
		if err = ChangeCloud("b", mutation); err != ErrCloudNotFound {
			t.Fatal("other owner mutated", err)
		}
	}
	child := cloudFolder(t, "a", a.ID, "Вложенная")
	if err = ChangeCloud("a", CloudMutation{Action: "move", ParentID: child.ID, Items: []CloudRef{{a.ID, a.Version}}}); err != ErrCloudMove {
		t.Fatal("cycle", err)
	}
	if err = ChangeCloud("a", CloudMutation{Action: "star", Starred: true, Items: []CloudRef{{file.ID, file.Version}, {duplicate.ID, 99}}}); err != ErrCloudConflict {
		t.Fatal("stale batch", err)
	}
	if cloudCurrent(t, "a", file.ID).Starred {
		t.Fatal("partial batch commit")
	}
	for _, name := range []string{"../escape", "CON.txt", "a/b", "x\\y", "..", "<script>", strings.Repeat("x", 181)} {
		if err = ChangeCloud("a", CloudMutation{Action: "folder", Name: name}); err != ErrCloudName {
			t.Fatal("invalid name", name, err)
		}
	}
	var archive bytes.Buffer
	if err = WriteCloudArchive("a", []string{a.ID, file.ID}, &archive); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range z.File {
		if entry.Name == "Документы/notes.txt" {
			r, _ := entry.Open()
			data, _ := io.ReadAll(r)
			_ = r.Close()
			found = string(data) == "original bytes"
		}
	}
	if !found || len(z.File) != 4 {
		t.Fatal("archive hierarchy or bytes", len(z.File))
	}
	if err = WriteCloudArchive("b", []string{a.ID}, io.Discard); err != ErrCloudNotFound {
		t.Fatal("archive leaked", err)
	}
	if err = LoadCloud(); err != nil || len(CloudState("a").Items) != 4 {
		t.Fatal("reload", err)
	}
}

func TestCloudTrashRestoreAndQuota(t *testing.T) {
	cloudFixture(t)
	folder := cloudFolder(t, "a", "", "Folder")
	independent := cloudUpload(t, "a", folder.ID, "old.txt", "old")
	file := cloudUpload(t, "a", folder.ID, "current.txt", "current")
	cloudChange(t, "a", "trash", independent)
	cloudChange(t, "a", "trash", folder)
	if CloudState("a").TrashBytes != 10 {
		t.Fatal("trash size")
	}
	cloudChange(t, "a", "purge", cloudCurrent(t, "a", folder.ID))
	if _, _, err := OpenCloudFile("a", file.ID); err != ErrCloudNotFound {
		t.Fatal("purged file accessible", err)
	}
	path, _ := CloudBlobPath(CloudDirectory(), file.ID)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("blob not deleted", err)
	}
	cloudChange(t, "a", "restore", cloudCurrent(t, "a", independent.ID))
	if cloudCurrent(t, "a", independent.ID).ParentID != "" {
		t.Fatal("missing parent not recovered")
	}
	cloudChange(t, "a", "trash", cloudCurrent(t, "a", independent.ID))
	cloudUpload(t, "a", "", "old.txt", "new")
	cloudChange(t, "a", "restore", cloudCurrent(t, "a", independent.ID))
	if cloudCurrent(t, "a", independent.ID).Name != "old (2).txt" {
		t.Fatal("restore overwrote name")
	}
	large := cloudUpload(t, "a", "", "large", strings.Repeat("a", 75))
	cloudChange(t, "a", "trash", large)
	if _, err := AddCloudFile("a", "", "overflow", uuid.NewString(), strings.NewReader(strings.Repeat("b", 30))); err != ErrCloudQuota {
		t.Fatal("trash excluded from quota", err)
	}
	if _, err := AddCloudFile("a", "", "huge", uuid.NewString(), strings.NewReader(strings.Repeat("b", 81))); err != ErrCloudSize {
		t.Fatal("file limit", err)
	}
	if len(CloudState("a").Items) != 3 {
		t.Fatal("failed uploads left metadata")
	}
}

func TestCloudConcurrentQuotaAndRollback(t *testing.T) {
	dir := cloudFixture(t)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := AddCloudFile("a", "", "file", uuid.NewString(), strings.NewReader(strings.Repeat("x", 60)))
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if err != ErrCloudQuota {
			t.Fatal(err)
		}
	}
	if success != 1 || CloudState("a").UsedBytes != 60 {
		t.Fatal("quota race")
	}
	db, err := persistence.OpenSQLite(filepath.Join(dir, "closed.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	backend = db
	if _, err = AddCloudFile("a", "", "rollback", uuid.NewString(), strings.NewReader("123")); err == nil {
		t.Fatal("closed DB accepted upload")
	}
	files, _ := os.ReadDir(CloudDirectory())
	if len(files) != 1 || len(CloudState("a").Items) != 1 {
		t.Fatal("failed upload left blob/cache", len(files))
	}
	item := CloudState("a").Items[0]
	if err = ChangeCloud("a", CloudMutation{Action: "trash", Items: []CloudRef{{item.ID, item.Version}}}); err == nil || CloudState("a").Items[0].DeletedAt != "" {
		t.Fatal("failed mutation changed cache")
	}
}

func TestCloudBackupAndSQLite(t *testing.T) {
	dir := cloudFixture(t)
	if err := backend.Write("users.json", []byte(`[{"id":"a","username":"a","password":"hash","status":"admin"},{"id":"b","username":"b","password":"hash","status":"admin"}]`)); err != nil {
		t.Fatal(err)
	}
	file := cloudUpload(t, "a", "", "private.txt", "private original")
	trash := cloudUpload(t, "b", "", "trash.txt", "recover me")
	cloudChange(t, "b", "trash", trash)
	out := filepath.Join(t.TempDir(), "backup")
	if err := CreateDataBackup(out); err != nil {
		t.Fatal(err)
	}
	docs, report, err := ReadMigrationSource(out, filepath.Join(out, "object_photos"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Counts["cloud_items"] != 2 || len(report.Warnings) != 0 {
		t.Fatal(report)
	}
	var saved []models.CloudItem
	_ = json.Unmarshal(docs["cloud.json"], &saved)
	if saved[0].OwnerID != "a" || saved[1].OwnerID != "b" || saved[1].DeletedAt == "" {
		t.Fatal("ownership/trash lost")
	}
	db, err := persistence.OpenSQLite(filepath.Join(dir, "data.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Import(docs); err != nil {
		t.Fatal(err)
	}
	snapshot, err := db.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err = CompareDocuments(docs, snapshot); err != nil {
		t.Fatal(err)
	}
	path, _ := CloudBlobPath(filepath.Join(out, "cloud"), file.ID)
	if err = os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = ReadMigrationSource(out, filepath.Join(out, "object_photos")); err == nil {
		t.Fatal("corrupt cloud backup accepted")
	}
}

func TestCloudValidationAndPrivateDirectory(t *testing.T) {
	cloudFixture(t)
	parent := ""
	for i := 0; i < 32; i++ {
		parent = cloudFolder(t, "a", parent, "Nested").ID
	}
	if err := ChangeCloud("a", CloudMutation{Action: "folder", ParentID: parent, Name: "Too deep"}); err != ErrCloudLimit {
		t.Fatal("depth limit", err)
	}
	file := cloudUpload(t, "b", "", "file", "bytes")
	invalid := file
	invalid.ParentID = parent
	if err := ValidateCloudMetadata([]models.CloudItem{invalid}); err == nil {
		t.Fatal("missing parent accepted")
	}
	invalid = file
	invalid.Size = 1<<63 - 1
	if err := ValidateCloudMetadata([]models.CloudItem{invalid}); err == nil {
		t.Fatal("overflowing size accepted")
	}
	t.Setenv("WORKSERVICE_CLOUD_DIR", "web/static/private/new")
	if err := LoadCloud(); err == nil || !strings.Contains(err.Error(), "web/static") {
		t.Fatal("public storage directory accepted", err)
	}
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "alias")
	if err := os.Symlink(target, link); err != nil {
		t.Log("symlink checks unavailable:", err)
		return
	}
	resolved, err := resolveCloudDirectory(filepath.Join(link, "new", "private"))
	if err != nil || resolved != filepath.Join(target, "new", "private") {
		t.Fatal("symlink ancestor not resolved", resolved, err)
	}
}
