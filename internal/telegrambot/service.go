package telegrambot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
		Text string `json:"text"`
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

func sendTelegramMessage(settings models.AppSettings, chatID int64, message string, replyMarkup any) error {
	body := map[string]any{
		"chat_id":                  chatID,
		"text":                     message,
		"disable_web_page_preview": true,
	}
	if replyMarkup != nil {
		body["reply_markup"] = replyMarkup
	}

	payloadBody, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, apiURL(settings.TelegramBotToken, "sendMessage"), bytes.NewReader(payloadBody))
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

func sendContactRequest(settings models.AppSettings, chatID int64) {
	if chatID == 0 {
		return
	}
	message := strings.Join([]string{
		"Чтобы привязать Telegram к аккаунту, отправьте свой контакт.",
		"",
		"Нажмите кнопку ниже: «Отправить контакт». Важно отправить именно свой контакт, а не просто написать номер текстом.",
		"После этого администратор нажмет «Синхронизировать контакты из бота» в настройках сайта.",
	}, "\n")
	replyMarkup := map[string]any{
		"keyboard": [][]map[string]any{
			{
				{
					"text":            "Отправить контакт",
					"request_contact": true,
				},
			},
		},
		"resize_keyboard":         true,
		"one_time_keyboard":       true,
		"input_field_placeholder": "Нажмите кнопку, чтобы отправить контакт",
	}
	_ = sendTelegramMessage(settings, chatID, message, replyMarkup)
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
		if update.Message.Chat.ID == 0 {
			continue
		}
		if update.Message.Contact == nil {
			sendContactRequest(settings, update.Message.Chat.ID)
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

	return sendTelegramMessage(settings, contact.ChatID, message, nil)
}

func formatAssignmentHours(startTime, endTime string, lunch int) string {
	start, err := time.Parse("15:04", startTime)
	if err != nil {
		return ""
	}
	end, err := time.Parse("15:04", endTime)
	if err != nil {
		return ""
	}
	minutes := int(end.Sub(start).Minutes()) - lunch
	if minutes < 0 {
		minutes = 0
	}
	return fmt.Sprintf("%.2f", float64(minutes)/60.0)
}

func assignmentObjectNames(entry models.TimesheetEntry) string {
	objects, err := storage.GetObjects()
	if err != nil {
		return "не указаны"
	}
	objectNames := make([]string, 0, len(entry.ObjectIDs))
	for _, objectID := range entry.ObjectIDs {
		for _, object := range objects {
			if object.ID == objectID {
				objectNames = append(objectNames, object.Name)
				break
			}
		}
	}
	if len(objectNames) == 0 {
		return "не указаны"
	}
	return strings.Join(objectNames, ", ")
}

func assignmentMessage(settings models.AppSettings, worker models.Worker, entry models.TimesheetEntry, title string) string {
	lines := []string{
		title,
		"",
		"Работник: " + strings.TrimSpace(worker.Name),
		"Дата: " + strings.TrimSpace(entry.Date),
	}

	if strings.TrimSpace(entry.UserMark) != "" {
		lines = append(lines, "Отметка: "+strings.TrimSpace(entry.UserMark))
	} else {
		hours := formatAssignmentHours(entry.StartTime, entry.EndTime, entry.LunchBreakMinutes)
		timeLine := "Время: " + strings.TrimSpace(entry.StartTime) + "-" + strings.TrimSpace(entry.EndTime)
		if hours != "" {
			timeLine += " (" + hours + " ч)"
		}
		lines = append(lines, timeLine)
		lines = append(lines, "Объекты: "+assignmentObjectNames(entry))
	}

	if notes := strings.TrimSpace(entry.Notes); notes != "" {
		lines = append(lines, "Комментарий: "+notes)
	}
	if creator := strings.TrimSpace(entry.CreatedByName); creator != "" {
		lines = append(lines, "Создал: "+creator)
	}
	if siteURL := strings.TrimSpace(settings.TelegramSiteURL); siteURL != "" {
		lines = append(lines, "", "Сайт: "+strings.TrimRight(siteURL, "/")+"/schedule")
	}
	return strings.Join(lines, "\n")
}

func SendScheduleEntryNotification(entry models.TimesheetEntry, title string) error {
	settings, err := loadSettings()
	if err != nil {
		return err
	}

	workers, err := storage.GetWorkers()
	if err != nil {
		return err
	}
	workersByID := make(map[string]models.Worker, len(workers))
	for _, worker := range workers {
		workersByID[worker.ID] = worker
	}

	_, _ = SyncContacts()
	sentChats := map[int64]struct{}{}
	failures := make([]string, 0)
	for _, workerID := range entry.WorkerIDs {
		worker, ok := workersByID[workerID]
		if !ok || strings.TrimSpace(worker.Phone) == "" {
			continue
		}
		contact, err := storage.FindTelegramContactByPhone(worker.Phone)
		if err != nil {
			continue
		}
		if _, exists := sentChats[contact.ChatID]; exists {
			continue
		}
		sentChats[contact.ChatID] = struct{}{}
		if err := sendTelegramMessage(settings, contact.ChatID, assignmentMessage(settings, worker, entry, title), nil); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

// SendMonthlyMileageReminders sends one reminder per assigned vehicle during
// the first three days of a month. A successful delivery is persisted so a
// server restart cannot duplicate it.
func SendMonthlyMileageReminders(now time.Time) (int, error) {
	if now.Day() > 3 {
		return 0, nil
	}
	settings, err := loadSettings()
	if err != nil {
		return 0, err
	}
	_, _ = SyncContacts()

	vehicles, err := storage.GetVehicles()
	if err != nil {
		return 0, err
	}
	users, err := storage.GetUsers()
	if err != nil {
		return 0, err
	}
	usersByID := make(map[string]models.User, len(users))
	for _, user := range users {
		usersByID[user.ID] = user
	}

	month := now.Format("2006-01")
	sent := 0
	failures := make([]string, 0)
	for _, vehicle := range vehicles {
		if vehicle.AssignedUserID == "" || vehicle.LastMileageReminderMonth == month || storage.HasMileageRecordForMonth(vehicle.ID, month) {
			continue
		}
		user, exists := usersByID[vehicle.AssignedUserID]
		if !exists {
			continue
		}
		phone := strings.TrimSpace(user.Phone)
		if phone == "" {
			if worker, workerErr := storage.GetWorkerByUserID(user.ID); workerErr == nil {
				phone = strings.TrimSpace(worker.Phone)
			}
		}
		contact, contactErr := storage.FindTelegramContactByPhone(phone)
		if contactErr != nil {
			continue
		}
		message := strings.Join([]string{
			"Уточните пробег автомобиля за текущий месяц.",
			"",
			"ТС: " + strings.TrimSpace(vehicle.Name),
			"Госномер: " + strings.TrimSpace(vehicle.RegistrationNumber),
			"Добавьте на сайте запись типа «Пробег».",
		}, "\n")
		if siteURL := strings.TrimSpace(settings.TelegramSiteURL); siteURL != "" {
			message += "\n\nСайт: " + strings.TrimRight(siteURL, "/") + "/vehicles/" + vehicle.ID
		}
		if err := sendTelegramMessage(settings, contact.ChatID, message, nil); err != nil {
			failures = append(failures, vehicle.RegistrationNumber+": "+err.Error())
			continue
		}
		if err := storage.MarkVehicleMileageReminder(vehicle.ID, month); err != nil {
			failures = append(failures, vehicle.RegistrationNumber+": "+err.Error())
			continue
		}
		sent++
	}
	if len(failures) > 0 {
		return sent, errors.New(strings.Join(failures, "; "))
	}
	return sent, nil
}

func StartMileageReminderScheduler() {
	go func() {
		run := func() {
			sent, err := SendMonthlyMileageReminders(time.Now())
			if err != nil && !errors.Is(err, ErrBotNotConfigured) {
				log.Printf("mileage reminder check failed: %v", err)
			}
			if sent > 0 {
				log.Printf("sent %d monthly mileage reminders", sent)
			}
		}
		run()
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			run()
		}
	}()
}
