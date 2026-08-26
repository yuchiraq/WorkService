package storage

import (
	"path/filepath"
	"testing"

	"project/internal/models"
)

func TestUpdateGalleryDirectoryPreservesTelegramSettings(t *testing.T) {
	oldFile := appSettingsFile
	oldSettings := appSettings
	defer func() {
		appSettingsFile = oldFile
		appSettings = oldSettings
	}()

	appSettingsFile = filepath.Join(t.TempDir(), "app_settings.json")
	appSettings = models.AppSettings{TelegramBotToken: "token", TelegramSiteURL: "https://example.test"}
	if err := UpdateGalleryDirectory(" /var/www/gallery "); err != nil {
		t.Fatalf("UpdateGalleryDirectory() error = %v", err)
	}
	if appSettings.GalleryDirectory != "/var/www/gallery" {
		t.Fatalf("gallery directory = %q", appSettings.GalleryDirectory)
	}
	if appSettings.TelegramBotToken != "token" || appSettings.TelegramSiteURL != "https://example.test" {
		t.Fatal("telegram settings were overwritten")
	}
}
