package telegrambot

import (
	"testing"
	"time"
)

func TestMonthlyMileageReminderDoesNotRunAfterThirdDay(t *testing.T) {
	sent, err := SendMonthlyMileageReminders(time.Date(2026, time.August, 4, 9, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("SendMonthlyMileageReminders() error = %v", err)
	}
	if sent != 0 {
		t.Fatalf("sent = %d, want 0", sent)
	}
}
