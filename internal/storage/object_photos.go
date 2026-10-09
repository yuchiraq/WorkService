package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"project/internal/models"
	"project/internal/persistence"
)

var (
	photosMutex      sync.RWMutex
	objectPhotos     []models.ObjectPhoto
	photosFile       = "storage/object_photos.json"
	ErrPhotoNotFound = errors.New("Фото не найдено")
	ErrPhotoConflict = errors.New("Фото уже изменено. Обновите список")
	ErrPhotoLimit    = errors.New("Достигнут лимит объекта: 500 фото или 2 ГБ")
	ErrPhotoCaption  = errors.New("Подпись должна быть не длиннее 300 символов")
)

func ObjectPhotosDirectory() string {
	if dir := strings.TrimSpace(os.Getenv("WORKSERVICE_PHOTO_DIR")); dir != "" {
		return dir
	}
	return "storage/object_photos"
}

func LoadObjectPhotos() error {
	photosMutex.Lock()
	defer photosMutex.Unlock()
	data, err := readData(photosFile)
	if os.IsNotExist(err) {
		objectPhotos = []models.ObjectPhoto{}
		return nil
	}
	if err != nil {
		return err
	}
	var next []models.ObjectPhoto
	if err = json.Unmarshal(data, &next); err != nil {
		return err
	}
	objectPhotos = next
	return nil
}

func GetObjectPhotos(objectID string) []models.ObjectPhoto {
	photosMutex.RLock()
	defer photosMutex.RUnlock()
	result := []models.ObjectPhoto{}
	for _, photo := range objectPhotos {
		if photo.ObjectID == objectID {
			result = append(result, photo)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].CreatedAt > result[j].CreatedAt })
	return result
}

func GetObjectPhoto(objectID, id string) (models.ObjectPhoto, error) {
	photosMutex.RLock()
	defer photosMutex.RUnlock()
	for _, photo := range objectPhotos {
		if photo.ObjectID == objectID && photo.ID == id {
			return photo, nil
		}
	}
	return models.ObjectPhoto{}, ErrPhotoNotFound
}

func PhotoPath(directory, id string, thumbnail bool) (string, error) {
	if _, err := uuid.Parse(id); err != nil || len(id) != 36 {
		return "", ErrPhotoNotFound
	}
	suffix := ".jpg"
	if thumbnail {
		suffix = "-thumb.jpg"
	}
	return filepath.Join(directory, id+suffix), nil
}

func OpenPhotoFile(directory, id string, thumbnail bool) (*os.File, error) {
	path, err := PhotoPath(directory, id, thumbnail)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("photo is not a regular file")
	}
	return os.Open(path)
}

func savePhotos(next []models.ObjectPhoto) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err = writeData(photosFile, data); err != nil {
		return err
	}
	objectPhotos = next
	return nil
}

func CreateObjectPhoto(photo models.ObjectPhoto, full, thumbnail []byte) (models.ObjectPhoto, bool, error) {
	objectsMutex.RLock()
	defer objectsMutex.RUnlock()
	found := false
	for _, object := range objects {
		if object.ID == photo.ObjectID {
			found = true
			break
		}
	}
	if !found {
		return photo, false, errors.New("object not found")
	}
	photosMutex.Lock()
	defer photosMutex.Unlock()
	if photo.ObjectID == "" || photo.CreatedByID == "" || len(full) == 0 || len(thumbnail) == 0 {
		return photo, false, errors.New("invalid photo")
	}
	hash := sha256.Sum256(full)
	photo.SHA256 = hex.EncodeToString(hash[:])
	count, size := 0, int64(len(full))
	for _, item := range objectPhotos {
		if item.ObjectID != photo.ObjectID {
			continue
		}
		if item.SHA256 == photo.SHA256 {
			return item, true, nil
		}
		count++
		size += item.Size
	}
	if count >= 500 || size > 2<<30 {
		return photo, false, ErrPhotoLimit
	}
	photo.ID, photo.Version = uuid.NewString(), 1
	photo.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	photo.Size = int64(len(full))
	path, _ := PhotoPath(ObjectPhotosDirectory(), photo.ID, false)
	thumbPath, _ := PhotoPath(ObjectPhotosDirectory(), photo.ID, true)
	if err := persistence.AtomicWrite(path, full); err != nil {
		return photo, false, err
	}
	if err := persistence.AtomicWrite(thumbPath, thumbnail); err != nil {
		_ = os.Remove(path)
		return photo, false, err
	}
	next := append(append([]models.ObjectPhoto{}, objectPhotos...), photo)
	if err := savePhotos(next); err != nil {
		_ = os.Remove(path)
		_ = os.Remove(thumbPath)
		return photo, false, err
	}
	return photo, false, nil
}

func UpdateObjectPhoto(objectID, id, caption string, version int64) error {
	photosMutex.Lock()
	defer photosMutex.Unlock()
	caption = strings.TrimSpace(caption)
	if utf8.RuneCountInString(caption) > 300 {
		return ErrPhotoCaption
	}
	next := append([]models.ObjectPhoto{}, objectPhotos...)
	for i := range next {
		if next[i].ObjectID != objectID || next[i].ID != id {
			continue
		}
		if version != next[i].Version {
			return ErrPhotoConflict
		}
		next[i].Caption, next[i].Version = caption, version+1
		return savePhotos(next)
	}
	return ErrPhotoNotFound
}

func DeleteObjectPhoto(objectID, id string, version int64) error {
	photosMutex.Lock()
	defer photosMutex.Unlock()
	for i, photo := range objectPhotos {
		if photo.ObjectID != objectID || photo.ID != id {
			continue
		}
		if version != photo.Version {
			return ErrPhotoConflict
		}
		next := append(append([]models.ObjectPhoto{}, objectPhotos[:i]...), objectPhotos[i+1:]...)
		if err := savePhotos(next); err != nil {
			return err
		}
		// Commit metadata first: an interrupted delete may leave an unused file,
		// but never a visible photo whose file has already been removed.
		for _, thumbnail := range []bool{false, true} {
			path, err := PhotoPath(ObjectPhotosDirectory(), id, thumbnail)
			if err == nil {
				err = os.Remove(path)
			}
			if err != nil && !os.IsNotExist(err) {
				log.Printf("cleanup deleted photo %s: %v", id, err)
			}
		}
		return nil
	}
	return ErrPhotoNotFound
}

func PhotoFilename(photo models.ObjectPhoto) string {
	return fmt.Sprintf("%s.jpg", strings.TrimSuffix(photo.Name, filepath.Ext(photo.Name)))
}
