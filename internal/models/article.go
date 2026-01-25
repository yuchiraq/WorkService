package models

import "gorm.io/gorm"

// Article represents a blog post or article
type Article struct {
	gorm.Model
	Title   string `json:"title"`
	Content string `json:"content"`
	UserID  uint   `json:"user_id"`
	User    User   `json:"user"`
}
