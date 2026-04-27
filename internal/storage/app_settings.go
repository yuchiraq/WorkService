package storage

import (
	"encoding/json"
	"os"
	"strings"
	"sync"

	"project/internal/models"
)

var (
	appSettings      models.AppSettings
	appSettingsMutex sync.RWMutex
	appSettingsFile  = "storage/app_settings.json"
)

func LoadAppSettings() error {
	appSettingsMutex.Lock()
	defer appSettingsMutex.Unlock()

	appSettings = models.AppSettings{}
	file, err := os.ReadFile(appSettingsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(file) == 0 {
		return nil
	}
	if err := json.Unmarshal(file, &appSettings); err != nil {
		return err
	}
	normalizeAppSettings(&appSettings)
	return nil
}

func normalizeAppSettings(settings *models.AppSettings) {
	settings.TelegramBotToken = strings.TrimSpace(settings.TelegramBotToken)
	settings.TelegramBotUsername = strings.TrimSpace(strings.TrimPrefix(settings.TelegramBotUsername, "@"))
	settings.TelegramSiteURL = strings.TrimSpace(settings.TelegramSiteURL)
	if settings.TelegramUpdateOffset < 0 {
		settings.TelegramUpdateOffset = 0
	}
}

func saveAppSettings() error {
	data, err := json.MarshalIndent(appSettings, "", "    ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll("storage", 0o755); err != nil {
		return err
	}
	return os.WriteFile(appSettingsFile, data, 0o644)
}

func GetAppSettings() (models.AppSettings, error) {
	appSettingsMutex.RLock()
	defer appSettingsMutex.RUnlock()
	return appSettings, nil
}

func UpdateAppSettings(settings models.AppSettings) error {
	appSettingsMutex.Lock()
	defer appSettingsMutex.Unlock()

	normalizeAppSettings(&settings)
	appSettings = settings
	return saveAppSettings()
}
