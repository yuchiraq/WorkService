package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"project/internal/models"
	"project/internal/persistence"
)

func TestMigrationRoundtrip(t *testing.T) {
	dir := t.TempDir()
	docs := map[string][]byte{
		"users.json":             []byte(`[{"id":"u","username":"admin","name":"Admin","password":"hash-preserved","status":"admin","future":9007199254740993}]`),
		"workers.json":           []byte(`[{"id":"w","userId":"u"}]`),
		"objects.json":           []byte(`[{"id":"o","responsibleUserId":"u"}]`),
		"timesheets.json":        []byte(`[{"id":"t","workerIds":["w"],"objectIds":["o"],"periodEnd":"2026-10-05"}]`),
		"vehicles.json":          []byte(`{"vehicles":[{"id":"v","assignedUserId":"u","ownerHistory":[{"at":"2026-10-01","userId":"u","userName":"Snapshot"}]}],"records":[{"id":"r","vehicleId":"v","liters":3.25}]}`),
		"inventory.json":         []byte(`{"catalog":[{"name":"Нерж 160","unit":"pcs","material":"Нержавейка"}],"items":[{"id":"i","object_id":"o","name":"Нерж 160","unit":"pcs","quantity":1234,"version":1,"material":"Нержавейка"}],"history":[{"id":"h","object_id":"o","after":{"id":"i","quantity":1234,"material":"Нержавейка"}}]}`),
		"app_settings.json":      []byte(`{"telegramBotToken":"secret"}`),
		"telegram_contacts.json": []byte(`[{"phone":"+375290000000","chatId":1234567890123456}]`),
		"improvements.json":      []byte(`[]`), "object_photos.json": []byte(`[]`),
	}
	for name, data := range docs {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	before, report, err := ReadMigrationSource(dir, filepath.Join(dir, "object_photos"))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Warnings) != 0 || report.Counts["vehicle_owner_changes"] != 1 || report.Counts["inventory_materials"] != 1 {
		t.Fatal(report)
	}
	db, err := persistence.OpenSQLite(filepath.Join(t.TempDir(), "data.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Import(before); err != nil {
		t.Fatal(err)
	}
	after, err := db.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err = CompareDocuments(before, after); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "export")
	if err = ExportDocuments(after, dir, out); err != nil {
		t.Fatal(err)
	}
	if err = ExportDocuments(after, dir, out); err == nil {
		t.Fatal("overwrote export")
	}
	exported, _, err := ReadMigrationSource(out, filepath.Join(out, "object_photos"))
	if err != nil {
		t.Fatal(err)
	}
	if err = CompareDocuments(before, exported); err != nil {
		t.Fatal(err)
	}
	for name, data := range docs {
		got, _ := os.ReadFile(filepath.Join(dir, name))
		if string(got) != string(data) {
			t.Fatal("source modified", name)
		}
	}
	bad := map[string][]byte{"users.json": []byte(`[{"id":"u","username":"a","password":"x","status":"admin"},{"id":"u","username":"b","password":"x"}]`)}
	if _, err = ValidateDocuments(bad, dir); err == nil {
		t.Fatal("duplicate id accepted")
	}
}

func TestPhotoStorageAndBackup(t *testing.T) {
	previousBackend, previousObjects, previousPhotos, previousFile := backend, objects, objectPhotos, photosFile
	t.Cleanup(func() {
		backend = previousBackend
		objects = previousObjects
		objectPhotos = previousPhotos
		photosFile = previousFile
	})
	dir := t.TempDir()
	t.Setenv("WORKSERVICE_PHOTO_DIR", filepath.Join(dir, "object_photos"))
	backend = persistence.OpenJSON(dir)
	objects = []models.Object{{ID: "o"}}
	objectPhotos = nil
	photosFile = filepath.Join(dir, "object_photos.json")
	for name, data := range map[string][]byte{"users.json": []byte(`[{"id":"u","username":"admin","password":"hash","status":"admin"}]`), "objects.json": []byte(`[{"id":"o"}]`)} {
		if err := backend.Write(name, data); err != nil {
			t.Fatal(err)
		}
	}
	photo, duplicate, err := CreateObjectPhoto(models.ObjectPhoto{ObjectID: "o", CreatedByID: "u", Name: "photo.jpg"}, []byte("photo fixture"), []byte("thumbnail fixture"))
	if err != nil || duplicate {
		t.Fatal(photo, duplicate, err)
	}
	other, duplicate, err := CreateObjectPhoto(models.ObjectPhoto{ObjectID: "o", CreatedByID: "u"}, []byte("photo fixture"), []byte("thumbnail fixture"))
	if err != nil || !duplicate || other.ID != photo.ID {
		t.Fatal("dedupe", err)
	}
	if err = UpdateObjectPhoto("o", photo.ID, "caption", 1); err != nil {
		t.Fatal(err)
	}
	if err = DeleteObjectPhoto("o", photo.ID, 1); err != ErrPhotoConflict {
		t.Fatal("stale delete", err)
	}
	if err = DeleteObject("o"); err == nil {
		t.Fatal("object with photos deleted")
	}
	out := filepath.Join(t.TempDir(), "backup")
	if err = CreateDataBackup(out); err != nil {
		t.Fatal(err)
	}
	exported, _, err := ReadMigrationSource(out, filepath.Join(out, "object_photos"))
	if err != nil {
		t.Fatal(err)
	}
	var saved []models.ObjectPhoto
	_ = json.Unmarshal(exported["object_photos.json"], &saved)
	if len(saved) != 1 || saved[0].Caption != "caption" {
		t.Fatal("photo metadata lost")
	}
	if _, err = PhotoPath(dir, "../../outside", false); err == nil {
		t.Fatal("unsafe photo path")
	}
	if err = DeleteObjectPhoto("o", photo.ID, 2); err != nil {
		t.Fatal(err)
	}
	if len(GetObjectPhotos("o")) != 0 {
		t.Fatal("photo not deleted")
	}
	path, _ := PhotoPath(ObjectPhotosDirectory(), photo.ID, false)
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("photo file not removed", err)
	}
	if err = LoadObjectPhotos(); err != nil || len(GetObjectPhotos("o")) != 0 {
		t.Fatal("deleted photo came back", err)
	}
}

func TestSQLiteWriteFailureRollback(t *testing.T) {
	oldBackend, oldUsers, oldObjects, oldPhotos, oldSettings := backend, users, objects, objectPhotos, appSettings
	t.Cleanup(func() {
		backend, users, objects, objectPhotos, appSettings = oldBackend, oldUsers, oldObjects, oldPhotos, oldSettings
	})
	dir := t.TempDir()
	t.Setenv("WORKSERVICE_PHOTO_DIR", filepath.Join(dir, "photos"))
	db, err := persistence.OpenSQLite(filepath.Join(dir, "closed.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	backend = db
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	users = []models.User{{ID: "u", Name: "Original", Username: "admin", Password: "$2a$unchanged", Status: "admin"}}
	objects = []models.Object{{ID: "o", Name: "Original", Address: "Minsk", ResponsibleUserID: "u"}}
	objectPhotos = nil
	changed := users[0]
	changed.Name = "Changed"
	if err = UpdateUser(changed); err == nil {
		t.Fatal("write to closed database succeeded")
	}
	if users[0].Name != "Original" {
		t.Fatal("failed update leaked into user cache")
	}
	if err = DeleteUser("u"); err == nil || len(users) != 1 {
		t.Fatal("failed delete lost user")
	}
	changedObject := objects[0]
	changedObject.Name = "Changed"
	if err = UpdateObject(changedObject); err == nil || objects[0].Name != "Original" {
		t.Fatal("failed object update leaked")
	}
	if err = UpdateAppSettings(models.AppSettings{TelegramBotToken: "unsaved"}); err == nil || appSettings.TelegramBotToken == "unsaved" {
		t.Fatal("failed settings update leaked")
	}
	if _, _, err = CreateObjectPhoto(models.ObjectPhoto{ObjectID: "o", CreatedByID: "u"}, []byte("image"), []byte("thumb")); err == nil {
		t.Fatal("photo without metadata accepted")
	}
	entries, err := os.ReadDir(ObjectPhotosDirectory())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 || len(objectPhotos) != 0 {
		t.Fatal("failed photo left data behind")
	}
}
