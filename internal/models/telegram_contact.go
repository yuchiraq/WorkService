package models

// TelegramContactLink stores a phone-to-chat binding collected from the bot.
type TelegramContactLink struct {
	Phone     string `json:"phone"`
	ChatID    int64  `json:"chatId"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}
