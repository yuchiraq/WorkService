package models

// User represents a user in the system
// Note: This is a simplified model. In a real-world application, 
// you would hash passwords and handle sensitive data more securely.
// We are using plain text for demonstration purposes only.
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"` // In a real app, this should be a hashed password
    Name     string `json:"name"`      // User's full name for display
}
