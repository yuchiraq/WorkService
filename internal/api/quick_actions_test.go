package api

import (
	"strings"
	"testing"

	"project/internal/models"
)

func TestFloatingActions(t *testing.T) {
	vehicles := []models.Vehicle{
		{ID: "foreign", Name: "Foreign vehicle", AssignedUserID: "other"},
		{ID: "free", Name: "Free vehicle"},
		{ID: "own", Name: "Ford", AssignedUserID: "worker"},
	}
	for _, user := range []string{"", "admin", "no-vehicle"} {
		body := renderFloatingActions(user, "objects", vehicles)
		if strings.Contains(body, "data-quick-actions") || !strings.Contains(body, `data-modal-url="/schedule/new"`) {
			t.Fatalf("user %q without owned vehicle must keep the direct assignment action", user)
		}
	}
	body := renderFloatingActions("worker", "objects", vehicles)
	if !strings.Contains(body, "data-quick-actions") || !strings.Contains(body, `href="/vehicles/own#vehicle-record-form"`) || strings.Contains(body, `id="quick-vehicle-picker"`) {
		t.Fatal("one owned car must open the record directly")
	}
	if strings.Contains(body, "/vehicles/foreign") || strings.Contains(body, "/vehicles/free") {
		t.Fatal("unowned vehicles leaked into quick actions")
	}
	vehicles = append(vehicles, models.Vehicle{ID: "another", Name: `Audi <script>alert(1)</script>`, RegistrationNumber: `" onmouseover="alert(1)`, AssignedUserID: "worker"})
	body = renderFloatingActions("worker", "objects", vehicles)
	if !strings.Contains(body, `id="quick-vehicle-picker"`) || !strings.Contains(body, "data-quick-vehicles") {
		t.Fatal("multiple cars need a picker")
	}
	if strings.Contains(body, "<script>") || strings.Contains(body, `" onmouseover="`) {
		t.Fatal("vehicle labels were not escaped")
	}
	if strings.Index(body, "/vehicles/another#") > strings.Index(body, "/vehicles/own#") {
		t.Fatal("vehicle choices must be alphabetical")
	}
	body = renderFloatingActions("worker", "cloud", vehicles)
	if strings.Count(body, `id="cloud-floating-upload"`) != 1 || !strings.Contains(body, "data-quick-upload") {
		t.Fatal("cloud upload action must remain available")
	}
	body = renderFloatingActions("admin", "cloud", vehicles)
	if strings.Contains(body, "data-quick-actions") || !strings.Contains(body, `id="cloud-floating-upload"`) {
		t.Fatal("cloud without owned vehicles must retain its original action")
	}
}
