package models

// AppSettings stores configurable system integrations.
type AppSettings struct {
	TelegramBotToken     string `json:"telegramBotToken,omitempty"`
	TelegramBotUsername  string `json:"telegramBotUsername,omitempty"`
	TelegramSiteURL      string `json:"telegramSiteUrl,omitempty"`
	TelegramUpdateOffset int    `json:"telegramUpdateOffset,omitempty"`
}
