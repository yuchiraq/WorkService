package telegrambot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"project/internal/models"
	"project/internal/storage"
)

var (
	ErrBotNotConfigured = errors.New("telegram bot is not configured")
	ErrChatNotLinked    = errors.New("telegram chat is not linked to this phone")
)

type SyncSummary struct {
	Processed int
	Linked    int
}

type botResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	Description string `json:"description"`
}

type updateResult struct {
	UpdateID int `json:"update_id"`
	Message  struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"from"`
		Contact *struct {
			PhoneNumber string `json:"phone_number"`
			FirstName   string `json:"first_name"`
			LastName    string `json:"last_name"`
		} `json:"contact"`
	} `json:"message"`
}

func apiURL(token, method string) string {
	return "https://api.telegram.org/bot" + token + "/" + method
}

func loadSettings() (models.AppSettings, error) {
	settings, err := storage.GetAppSettings()
	if err != nil {
		return models.AppSettings{}, err
	}
	if strings.TrimSpace(settings.TelegramBotToken) == "" {
		return models.AppSettings{}, ErrBotNotConfigured
	}
	return settings, nil
}

func SyncContacts() (SyncSummary, error) {
	settings, err := loadSettings()
	if err != nil {
		return SyncSummary{}, err
	}

	req, err := http.NewRequest(http.MethodGet, apiURL(settings.TelegramBotToken, "getUpdates"), nil)
	if err != nil {
		return SyncSummary{}, err
	}
	q := req.URL.Query()
	if settings.TelegramUpdateOffset > 0 {
		q.Set("offset", fmt.Sprintf("%d", settings.TelegramUpdateOffset))
	}
	q.Set("timeout", "1")
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return SyncSummary{}, err
	}
	defer resp.Body.Close()

	var payload botResponse[[]updateResult]
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return SyncSummary{}, err
	}
	if !payload.OK {
		if strings.TrimSpace(payload.Description) != "" {
			return SyncSummary{}, errors.New(payload.Description)
		}
		return SyncSummary{}, errors.New("telegram getUpdates failed")
	}

	summary := SyncSummary{}
	maxUpdateID := settings.TelegramUpdateOffset
	for _, update := range payload.Result {
		summary.Processed++
		if update.UpdateID >= maxUpdateID {
			maxUpdateID = update.UpdateID + 1
		}
		if update.Message.Contact == nil || update.Message.Chat.ID == 0 {
			continue
		}

		firstName := strings.TrimSpace(update.Message.Contact.FirstName)
		lastName := strings.TrimSpace(update.Message.Contact.LastName)
		if firstName == "" {
			firstName = strings.TrimSpace(update.Message.From.FirstName)
		}
		if lastName == "" {
			lastName = strings.TrimSpace(update.Message.From.LastName)
		}

		if err := storage.UpsertTelegramContact(models.TelegramContactLink{
			Phone:     update.Message.Contact.PhoneNumber,
			ChatID:    update.Message.Chat.ID,
			Username:  update.Message.From.Username,
			FirstName: firstName,
			LastName:  lastName,
			UpdatedAt: time.Now().Format(time.RFC3339),
		}); err == nil {
			summary.Linked++
		}
	}

	settings.TelegramUpdateOffset = maxUpdateID
	if err := storage.UpdateAppSettings(settings); err != nil {
		return summary, err
	}

	return summary, nil
}

func SendAccountCreatedNotification(user models.User, plainPassword string) error {
	if strings.TrimSpace(plainPassword) == "" {
		return nil
	}

	settings, err := loadSettings()
	if err != nil {
		return err
	}

	_, _ = SyncContacts()

	contact, err := storage.FindTelegramContactByPhone(user.Phone)
	if err != nil {
		return ErrChatNotLinked
	}

	siteURL := strings.TrimSpace(settings.TelegramSiteURL)
	if siteURL == "" {
		siteURL = "Укажите адрес сайта в настройках"
	}

	message := strings.Join([]string{
		"Для вас создан аккаунт в ЧСУП \"АВАЮССТРОЙ\".",
		"",
		"Сайт: " + siteURL,
		"Логин: " + strings.TrimSpace(user.Username),
		"Пароль: " + plainPassword,
		"",
		"Как установить PWA:",
		"iPhone / Safari: откройте сайт, нажмите «Поделиться» -> «На экран Домой».",
		"Android / Chrome: откройте сайт, меню браузера -> «Добавить на главный экран» или «Установить приложение».",
		"",
		"После первого входа удалите это сообщение с паролем.",
	}, "\n")

	body, _ := json.Marshal(map[string]any{
		"chat_id":                  contact.ChatID,
		"text":                     message,
		"disable_web_page_preview": true,
	})

	req, err := http.NewRequest(http.MethodPost, apiURL(settings.TelegramBotToken, "sendMessage"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var payload botResponse[map[string]any]
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	if !payload.OK {
		if strings.TrimSpace(payload.Description) != "" {
			return errors.New(payload.Description)
		}
		return errors.New("telegram sendMessage failed")
	}
	return nil
}
