package storage

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"project/internal/models"
)

var (
	telegramContacts      []models.TelegramContactLink
	telegramContactsMutex sync.RWMutex
	telegramContactsFile  = "storage/telegram_contacts.json"
)

func NormalizePhoneNumber(value string) string {
	var digits strings.Builder
	for _, r := range strings.TrimSpace(value) {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	normalized := digits.String()
	if normalized == "" {
		return ""
	}
	if len(normalized) == 11 && normalized[0] == '8' {
		normalized = "7" + normalized[1:]
	}
	if !strings.HasPrefix(normalized, "7") && len(normalized) == 10 {
		normalized = "7" + normalized
	}
	return "+" + normalized
}

func LoadTelegramContacts() error {
	telegramContactsMutex.Lock()
	defer telegramContactsMutex.Unlock()

	telegramContacts = []models.TelegramContactLink{}
	file, err := readData(telegramContactsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(file) == 0 {
		return nil
	}
	if err := json.Unmarshal(file, &telegramContacts); err != nil {
		return err
	}
	for i := range telegramContacts {
		telegramContacts[i].Phone = NormalizePhoneNumber(telegramContacts[i].Phone)
		telegramContacts[i].Username = strings.TrimSpace(strings.TrimPrefix(telegramContacts[i].Username, "@"))
	}
	return nil
}

func saveTelegramContacts() error {
	data, err := json.MarshalIndent(telegramContacts, "", "    ")
	if err != nil {
		return err
	}
	return writeData(telegramContactsFile, data)
}

func GetTelegramContacts() ([]models.TelegramContactLink, error) {
	telegramContactsMutex.RLock()
	defer telegramContactsMutex.RUnlock()

	contactsCopy := make([]models.TelegramContactLink, len(telegramContacts))
	copy(contactsCopy, telegramContacts)
	sort.Slice(contactsCopy, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, contactsCopy[i].UpdatedAt)
		tj, _ := time.Parse(time.RFC3339, contactsCopy[j].UpdatedAt)
		return tj.Before(ti)
	})
	return contactsCopy, nil
}

func UpsertTelegramContact(contact models.TelegramContactLink) error {
	telegramContactsMutex.Lock()
	defer telegramContactsMutex.Unlock()

	contact.Phone = NormalizePhoneNumber(contact.Phone)
	contact.Username = strings.TrimSpace(strings.TrimPrefix(contact.Username, "@"))
	if contact.Phone == "" || contact.ChatID == 0 {
		return errors.New("phone and chat id are required")
	}
	if strings.TrimSpace(contact.UpdatedAt) == "" {
		contact.UpdatedAt = time.Now().Format(time.RFC3339)
	}

	for i := range telegramContacts {
		if telegramContacts[i].Phone == contact.Phone || telegramContacts[i].ChatID == contact.ChatID {
			previous := telegramContacts[i]
			telegramContacts[i] = contact
			if err := saveTelegramContacts(); err != nil {
				telegramContacts[i] = previous
				return err
			}
			return nil
		}
	}

	telegramContacts = append(telegramContacts, contact)
	if err := saveTelegramContacts(); err != nil {
		telegramContacts = telegramContacts[:len(telegramContacts)-1]
		return err
	}
	return nil
}

func FindTelegramContactByPhone(phone string) (models.TelegramContactLink, error) {
	telegramContactsMutex.RLock()
	defer telegramContactsMutex.RUnlock()

	normalized := NormalizePhoneNumber(phone)
	for _, contact := range telegramContacts {
		if contact.Phone == normalized {
			return contact, nil
		}
	}
	return models.TelegramContactLink{}, errors.New("telegram contact not found")
}

func DeleteTelegramContactByPhone(phone string) error {
	telegramContactsMutex.Lock()
	defer telegramContactsMutex.Unlock()

	normalized := NormalizePhoneNumber(phone)
	for i, contact := range telegramContacts {
		if contact.Phone == normalized {
			previous := telegramContacts
			telegramContacts = append(append([]models.TelegramContactLink{}, telegramContacts[:i]...), telegramContacts[i+1:]...)
			if err := saveTelegramContacts(); err != nil {
				telegramContacts = previous
				return err
			}
			return nil
		}
	}
	return errors.New("telegram contact not found")
}
