package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

type CloudDevice struct {
	ID         string    `json:"id"`
	OwnerID    string    `json:"ownerId"`
	Name       string    `json:"name"`
	ReadOnly   bool      `json:"readOnly"`
	CreatedAt  time.Time `json:"createdAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
}

type cloudDeviceRecord struct {
	CloudDevice
	SecretHash  string `json:"secretHash"`
	AccountHash string `json:"accountHash"`
}

var cloudDevicesMu sync.Mutex
var cloudDevices []cloudDeviceRecord
var ErrCloudDevice = errors.New("Доступ устройства недействителен")

func deviceHash(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])
}

func validateCloudDevices(records []cloudDeviceRecord) error {
	seen := map[string]bool{}
	for _, d := range records {
		_, idErr := uuid.Parse(d.ID)
		h, err := hex.DecodeString(d.SecretHash)
		a, accountErr := hex.DecodeString(d.AccountHash)
		if idErr != nil || seen[d.ID] || d.OwnerID == "" || d.Name == "" || len(d.Name) > 320 || err != nil || len(h) != 32 || accountErr != nil || len(a) != 32 || !d.ExpiresAt.After(d.CreatedAt) {
			return ErrCloudDevice
		}
		seen[d.ID] = true
	}
	return nil
}

func LoadCloudDevices() error {
	cloudDevicesMu.Lock()
	defer cloudDevicesMu.Unlock()
	data, err := readData("storage/cloud_devices.json")
	var next []cloudDeviceRecord
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if err = json.Unmarshal(data, &next); err != nil {
			return err
		}
	}
	if err = validateCloudDevices(next); err != nil {
		return err
	}
	cloudDevices = next
	return nil
}

func saveCloudDevices(next []cloudDeviceRecord) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err = writeData("storage/cloud_devices.json", data); err != nil {
		return err
	}
	cloudDevices = next
	return nil
}

func ListCloudDevices(owner string) []CloudDevice {
	cloudDevicesMu.Lock()
	defer cloudDevicesMu.Unlock()
	result := []CloudDevice{}
	for _, d := range cloudDevices {
		if d.OwnerID == owner {
			result = append(result, d.CloudDevice)
		}
	}
	return result
}

func CreateCloudDevice(owner, name string, readOnly bool, days int) (CloudDevice, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 80 || strings.IndexFunc(name, unicode.IsControl) >= 0 || (days != 30 && days != 90 && days != 365) {
		return CloudDevice{}, "", ErrCloudName
	}
	u, err := GetUserByID(owner)
	if err != nil || u.Status != "admin" {
		return CloudDevice{}, "", ErrCloudDevice
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return CloudDevice{}, "", err
	}
	now := time.Now().UTC()
	d := CloudDevice{ID: uuid.NewString(), OwnerID: owner, Name: name, ReadOnly: readOnly, CreatedAt: now, ExpiresAt: now.AddDate(0, 0, days)}
	password := "ws_" + d.ID + "." + base64.RawURLEncoding.EncodeToString(secret)
	cloudDevicesMu.Lock()
	defer cloudDevicesMu.Unlock()
	count := 0
	for _, existing := range cloudDevices {
		if existing.OwnerID == owner {
			count++
		}
	}
	if count >= 20 {
		return CloudDevice{}, "", ErrCloudLimit
	}
	next := append(append([]cloudDeviceRecord{}, cloudDevices...), cloudDeviceRecord{d, deviceHash(password), deviceHash(u.Password)})
	if err = saveCloudDevices(next); err != nil {
		return CloudDevice{}, "", err
	}
	return d, password, nil
}

func RevokeCloudDevice(owner, id string) error {
	cloudDevicesMu.Lock()
	defer cloudDevicesMu.Unlock()
	next := []cloudDeviceRecord{}
	found := false
	for _, d := range cloudDevices {
		if d.OwnerID == owner && d.ID == id {
			found = true
			continue
		}
		next = append(next, d)
	}
	if !found {
		return ErrCloudNotFound
	}
	return saveCloudDevices(next)
}

// Only high-entropy generated device passwords are accepted, never account passwords.
func AuthenticateCloudDevice(username, password string) (CloudDevice, error) {
	if len(password) != 83 || !strings.HasPrefix(password, "ws_") || password[39] != '.' {
		return CloudDevice{}, ErrCloudDevice
	}
	u, err := GetUserByUsername(username)
	if err != nil || u.Status != "admin" {
		return CloudDevice{}, ErrCloudDevice
	}
	id, hash, account := password[3:39], deviceHash(password), deviceHash(u.Password)
	cloudDevicesMu.Lock()
	defer cloudDevicesMu.Unlock()
	for i, d := range cloudDevices {
		if d.ID != id || d.OwnerID != u.ID {
			continue
		}
		if !time.Now().Before(d.ExpiresAt) || subtle.ConstantTimeCompare([]byte(hash), []byte(d.SecretHash)) != 1 || subtle.ConstantTimeCompare([]byte(account), []byte(d.AccountHash)) != 1 {
			break
		}
		if time.Since(d.LastUsedAt) > time.Hour {
			next := append([]cloudDeviceRecord{}, cloudDevices...)
			next[i].LastUsedAt = time.Now().UTC()
			if err = saveCloudDevices(next); err != nil {
				return CloudDevice{}, err
			}
			d = next[i]
		}
		return d.CloudDevice, nil
	}
	return CloudDevice{}, ErrCloudDevice
}
