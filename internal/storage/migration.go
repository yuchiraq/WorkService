package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"project/internal/models"
	"project/internal/persistence"
)

type MigrationReport struct {
	Counts   map[string]int `json:"counts"`
	Warnings []string       `json:"warnings"`
}

// ValidateDocuments never normalizes or rewrites source data, including unknown
// model fields. Historical references to removed entities are reported, not lost.
func ValidateDocuments(documents map[string][]byte, photoDir string, cloudDirs ...string) (MigrationReport, error) {
	report := MigrationReport{Counts: map[string]int{}, Warnings: []string{}}
	var u []models.User
	var w []models.Worker
	var o []models.Object
	var t []models.TimesheetEntry
	var improvements []models.ImprovementItem
	var settings models.AppSettings
	var contacts []models.TelegramContactLink
	var fleet fleetFileData
	var inv inventoryData
	var photos []models.ObjectPhoto
	var cloud []models.CloudItem
	targets := map[string]any{"users.json": &u, "workers.json": &w, "objects.json": &o, "timesheets.json": &t, "improvements.json": &improvements, "app_settings.json": &settings, "telegram_contacts.json": &contacts, "vehicles.json": &fleet, "inventory.json": &inv, "object_photos.json": &photos}
	for name, data := range documents {
		if name == "cloud.json" {
			if err := json.Unmarshal(data, &cloud); err != nil {
				return report, err
			}
			continue
		}
		target, ok := targets[name]
		if !ok {
			return report, fmt.Errorf("unknown document %s", name)
		}
		if err := json.Unmarshal(data, target); err != nil {
			return report, fmt.Errorf("%s: %w", name, err)
		}
	}
	if len(u) == 0 {
		return report, errors.New("users.json is missing or has no users")
	}
	sets := map[string]map[string]bool{}
	for name, values := range map[string]any{"users": u, "workers": w, "objects": o, "timesheets": t, "improvements": improvements, "vehicles": fleet.Vehicles, "vehicle_records": fleet.Records, "inventory_items": inv.Items, "inventory_history": inv.History, "object_photos": photos} {
		set := map[string]bool{}
		v := reflect.ValueOf(values)
		for i := 0; i < v.Len(); i++ {
			id := v.Index(i).FieldByName("ID").String()
			if strings.TrimSpace(id) == "" || set[id] {
				return report, fmt.Errorf("%s: empty or duplicate id at record %d", name, i+1)
			}
			set[id] = true
		}
		sets[name] = set
		report.Counts[name] = v.Len()
	}
	report.Counts["telegram_contacts"], report.Counts["inventory_catalog"] = len(contacts), len(inv.Catalog)
	admins := 0
	usernames := map[string]bool{}
	for _, user := range u {
		if user.Username == "" || user.Password == "" || usernames[user.Username] {
			return report, errors.New("users: empty credentials or duplicate username")
		}
		usernames[user.Username] = true
		if user.Status == "admin" {
			admins++
		}
	}
	if admins == 0 {
		return report, errors.New("users: no administrator; fix the source before import")
	}
	reference := func(kind, id, source string) {
		if id != "" && !sets[kind][id] {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s references missing %s id %s", source, kind, id))
		}
	}
	for _, worker := range w {
		reference("users", worker.UserID, "worker "+worker.ID)
	}
	for _, object := range o {
		reference("users", object.ResponsibleUserID, "object "+object.ID)
	}
	for _, entry := range t {
		for _, id := range entry.WorkerIDs {
			reference("workers", id, "assignment "+entry.ID)
		}
		for _, id := range entry.ObjectIDs {
			reference("objects", id, "assignment "+entry.ID)
		}
	}
	for _, vehicle := range fleet.Vehicles {
		reference("users", vehicle.AssignedUserID, "vehicle "+vehicle.ID)
		report.Counts["vehicle_owner_changes"] += len(vehicle.OwnerHistory)
	}
	for _, record := range fleet.Records {
		reference("vehicles", record.VehicleID, "vehicle record "+record.ID)
	}
	inventoryKeys := map[string]bool{}
	materials := map[string]bool{}
	for _, item := range inv.Items {
		reference("objects", item.ObjectID, "inventory "+item.ID)
		if strings.TrimSpace(item.Name) == "" || utf8.RuneCountInString(item.Name) > 120 || utf8.RuneCountInString(item.Note) > 500 || utf8.RuneCountInString(item.Material) > 60 || !validInventoryUnit(item.Unit) || item.Quantity < 0 || item.Quantity > MaxInventoryQuantity || item.Version < 1 {
			return report, fmt.Errorf("inventory %s: invalid name, material, quantity, unit or version", item.ID)
		}
		key := item.ObjectID + "\x00" + strings.ToLower(strings.Join(strings.Fields(item.Name), " ")) + "\x00" + item.Unit
		if inventoryKeys[key] {
			return report, fmt.Errorf("inventory %s: duplicate name and unit on object %s", item.ID, item.ObjectID)
		}
		inventoryKeys[key] = true
		if material := InventoryMaterial(item.Name, item.Material); material != "" {
			materials[strings.ToLower(material)] = true
		}
	}
	report.Counts["inventory_materials"] = len(materials)
	catalogKeys := map[string]bool{}
	for _, entry := range inv.Catalog {
		key := strings.ToLower(strings.Join(strings.Fields(entry.Name), " ")) + "\x00" + entry.Unit
		if strings.TrimSpace(entry.Name) == "" || utf8.RuneCountInString(entry.Name) > 120 || utf8.RuneCountInString(entry.Material) > 60 || !validInventoryUnit(entry.Unit) || catalogKeys[key] {
			return report, fmt.Errorf("inventory catalog: invalid or duplicate name %q", entry.Name)
		}
		catalogKeys[key] = true
	}
	for _, photo := range photos {
		if !sets["objects"][photo.ObjectID] {
			return report, fmt.Errorf("photo %s belongs to missing object", photo.ID)
		}
		for _, thumbnail := range []bool{false, true} {
			file, err := OpenPhotoFile(photoDir, photo.ID, thumbnail)
			if err != nil {
				return report, fmt.Errorf("photo %s thumbnail=%v: %w", photo.ID, thumbnail, err)
			}
			hash := sha256.New()
			size, err := io.Copy(hash, file)
			_ = file.Close()
			if err != nil {
				return report, err
			}
			if size == 0 {
				return report, fmt.Errorf("photo %s is empty", photo.ID)
			}
			if !thumbnail && (size != photo.Size || hex.EncodeToString(hash.Sum(nil)) != photo.SHA256) {
				return report, fmt.Errorf("photo %s checksum or size mismatch", photo.ID)
			}
		}
	}
	if err := ValidateCloudMetadata(cloud); err != nil {
		return report, err
	}
	cloudDir := CloudDirectory()
	if len(cloudDirs) > 0 {
		cloudDir = cloudDirs[0]
	}
	report.Counts["cloud_items"] = len(cloud)
	for _, item := range cloud {
		reference("users", item.OwnerID, "cloud item "+item.ID)
		if item.Kind != "file" {
			continue
		}
		file, err := OpenCloudBlob(cloudDir, item.ID)
		if err != nil {
			return report, fmt.Errorf("cloud file %s: %w", item.ID, err)
		}
		hash := sha256.New()
		size, err := io.Copy(hash, file)
		_ = file.Close()
		if err != nil {
			return report, err
		}
		if size != item.Size || hex.EncodeToString(hash.Sum(nil)) != item.SHA256 {
			return report, fmt.Errorf("cloud file %s checksum mismatch", item.ID)
		}
	}
	sort.Strings(report.Warnings)
	return report, nil
}

func ReadMigrationSource(directory, photoDir string, cloudDirs ...string) (map[string][]byte, MigrationReport, error) {
	documents, err := persistence.OpenJSON(directory).Snapshot()
	if err != nil {
		return nil, MigrationReport{}, err
	}
	cloudDir := filepath.Join(directory, "cloud")
	if len(cloudDirs) > 0 {
		cloudDir = cloudDirs[0]
	}
	report, err := ValidateDocuments(documents, photoDir, cloudDir)
	return documents, report, err
}

func CompareDocuments(left, right map[string][]byte) error {
	if len(left) != len(right) {
		return errors.New("document count differs")
	}
	for name, data := range left {
		if !bytes.Equal(data, right[name]) {
			return fmt.Errorf("document differs: %s", name)
		}
	}
	return nil
}

// ExportDocuments publishes a new complete directory; existing destinations are
// never overwritten. Metadata and referenced photos travel together for rollback.
func ExportDocuments(documents map[string][]byte, photoDir, destination string, cloudDirs ...string) error {
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		return errors.New("export destination already exists or is inaccessible")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(destination), ".workservice-export-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	hashes := map[string]string{}
	for name, data := range documents {
		if filepath.Base(name) != name || strings.ContainsAny(name, `/\`) {
			return errors.New("invalid document name")
		}
		if err = persistence.AtomicWrite(filepath.Join(stage, name), data); err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		hashes[name] = hex.EncodeToString(sum[:])
	}
	var photos []models.ObjectPhoto
	if data, ok := documents["object_photos.json"]; ok {
		if err = json.Unmarshal(data, &photos); err != nil {
			return err
		}
	}
	for _, photo := range photos {
		for _, thumbnail := range []bool{false, true} {
			source, e := OpenPhotoFile(photoDir, photo.ID, thumbnail)
			if e != nil {
				return e
			}
			name, _ := PhotoPath("object_photos", photo.ID, thumbnail)
			if e = os.MkdirAll(filepath.Join(stage, "object_photos"), 0700); e != nil {
				_ = source.Close()
				return e
			}
			target, e := os.OpenFile(filepath.Join(stage, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if e != nil {
				_ = source.Close()
				return e
			}
			hash := sha256.New()
			_, e = io.Copy(io.MultiWriter(target, hash), source)
			if e == nil {
				e = target.Sync()
			}
			e = errors.Join(e, target.Close(), source.Close())
			if e != nil {
				return e
			}
			hashes[filepath.ToSlash(name)] = hex.EncodeToString(hash.Sum(nil))
		}
	}
	cloudDir := CloudDirectory()
	if len(cloudDirs) > 0 {
		cloudDir = cloudDirs[0]
	}
	var cloud []models.CloudItem
	if data, ok := documents["cloud.json"]; ok {
		if err = json.Unmarshal(data, &cloud); err != nil {
			return err
		}
	}
	if err = ValidateCloudMetadata(cloud); err != nil {
		return err
	}
	for _, item := range cloud {
		if item.Kind != "file" {
			continue
		}
		source, e := OpenCloudBlob(cloudDir, item.ID)
		if e != nil {
			return e
		}
		if e = os.MkdirAll(filepath.Join(stage, "cloud"), 0700); e != nil {
			_ = source.Close()
			return e
		}
		relative, _ := CloudBlobPath("cloud", item.ID)
		target, e := os.OpenFile(filepath.Join(stage, relative), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			_ = source.Close()
			return e
		}
		hash := sha256.New()
		size, e := io.Copy(io.MultiWriter(target, hash), source)
		if e == nil {
			e = target.Sync()
		}
		e = errors.Join(e, target.Close(), source.Close())
		if e != nil {
			return e
		}
		checksum := hex.EncodeToString(hash.Sum(nil))
		if checksum != item.SHA256 || size != item.Size {
			return fmt.Errorf("cloud file %s checksum mismatch", item.ID)
		}
		hashes[filepath.ToSlash(relative)] = checksum
	}
	manifest, err := json.MarshalIndent(struct {
		Format    int               `json:"format"`
		CreatedAt string            `json:"createdAt"`
		SHA256    map[string]string `json:"sha256"`
	}{1, time.Now().UTC().Format(time.RFC3339), hashes}, "", "  ")
	if err != nil {
		return err
	}
	if err = persistence.AtomicWrite(filepath.Join(stage, "backup-manifest.json"), manifest); err != nil {
		return err
	}
	return os.Rename(stage, destination)
}

func CreateDataBackup(destination string) error {
	cloudMutex.RLock()
	defer cloudMutex.RUnlock()
	photosMutex.RLock()
	defer photosMutex.RUnlock()
	store := backend
	if store == nil {
		store = persistence.OpenJSON("storage")
	}
	documents, err := store.Snapshot()
	if err != nil {
		return err
	}
	return ExportDocuments(documents, ObjectPhotosDirectory(), destination)
}
