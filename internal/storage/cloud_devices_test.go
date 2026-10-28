package storage

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"project/internal/models"
)

func TestCloudDeviceSecrets(t *testing.T) {
	cloudFixture(t)
	oldUsers, oldDevices := users, cloudDevices
	t.Cleanup(func() { users, cloudDevices = oldUsers, oldDevices })
	users = []models.User{{ID: "a", Username: "admin", Status: "admin", Password: "account-hash"}}
	cloudDevices = nil
	d, secret, err := CreateCloudDevice("a", "Phone", true, 30)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := readData("storage/cloud_devices.json")
	if err != nil || strings.Contains(string(raw), secret) || strings.Contains(string(raw), "account-hash") {
		t.Fatal("plaintext persisted", err)
	}
	if err = LoadCloudDevices(); err != nil {
		t.Fatal(err)
	}
	if _, err = AuthenticateCloudDevice("admin", secret); err != nil {
		t.Fatal(err)
	}
	cloudDevices[0].ExpiresAt = time.Now().Add(-time.Minute)
	if _, err = AuthenticateCloudDevice("admin", secret); !errors.Is(err, ErrCloudDevice) {
		t.Fatal("expired password accepted")
	}
	cloudDevices[0].ExpiresAt = d.ExpiresAt
	if err = RevokeCloudDevice("other", d.ID); !errors.Is(err, ErrCloudNotFound) {
		t.Fatal("cross owner revoke")
	}
	if err = RevokeCloudDevice("a", d.ID); err != nil {
		t.Fatal(err)
	}
	if err = LoadCloudDevices(); err != nil {
		t.Fatal(err)
	}
	if _, err = AuthenticateCloudDevice("admin", secret); !errors.Is(err, ErrCloudDevice) {
		t.Fatal("revocation lost on reload")
	}
	if _, _, err = CreateCloudDevice("a", "Phone", false, 0); err == nil {
		t.Fatal("unbounded expiry")
	}
	// Migration recognizes the credential document and preserves only its hashes.
	_, _, err = CreateCloudDevice("a", "Laptop", false, 90)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := readData("storage/cloud_devices.json")
	userData, _ := json.Marshal(users)
	report, err := ValidateDocuments(map[string][]byte{"users.json": userData, "cloud_devices.json": data}, "")
	if err != nil || report.Counts["cloud_devices"] != 1 {
		t.Fatal("migration", err, report)
	}
}
