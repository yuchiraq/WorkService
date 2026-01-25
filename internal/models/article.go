package models

// Article represents a blog post or article
type Article struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	UserID  int    `json:"user_id"`
}
