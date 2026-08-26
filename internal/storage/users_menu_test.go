package storage

import (
	"path/filepath"
	"reflect"
	"testing"

	"project/internal/models"
)

func TestUpdateUserHiddenMenuItems(t *testing.T) {
	oldFile := usersFile
	oldUsers := users
	defer func() {
		usersFile = oldFile
		users = oldUsers
	}()

	usersFile = filepath.Join(t.TempDir(), "users.json")
	users = []models.User{{ID: "user-1", Username: "worker", Password: "hash", Name: "Worker", Status: "user"}}
	if err := UpdateUserHiddenMenuItems("user-1", []string{"timesheets", "schedule", "timesheets", ""}); err != nil {
		t.Fatalf("UpdateUserHiddenMenuItems() error = %v", err)
	}
	want := []string{"schedule", "timesheets"}
	if !reflect.DeepEqual(users[0].HiddenMenuItems, want) {
		t.Fatalf("hidden items = %#v, want %#v", users[0].HiddenMenuItems, want)
	}
}
