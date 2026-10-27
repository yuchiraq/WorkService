package storage

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"project/internal/models"
)

func validateCloudShare(item models.CloudItem) error {
	if item.ShareToken == "" {
		if item.ShareExpiresAt != "" {
			return errors.New("cloud share expiry without token")
		}
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(item.ShareToken)
	if err != nil || len(raw) != 32 || len(item.ShareToken) != 43 || item.DeletedAt != "" {
		return errors.New("invalid cloud share token")
	}
	if item.ShareExpiresAt != "" {
		if _, err = time.Parse(time.RFC3339, item.ShareExpiresAt); err != nil {
			return errors.New("invalid cloud share expiry")
		}
	}
	return nil
}

// A link grants read-only access to the current subtree, never to its ancestors.
func SetCloudShare(owner, id string, version int64, enabled bool, days int) error {
	cloudMutex.Lock()
	defer cloudMutex.Unlock()
	if days != 0 && days != 7 && days != 30 {
		return ErrCloudConflict
	}
	next := append([]models.CloudItem{}, cloudItems...)
	for i, item := range next {
		if item.ID != id || item.OwnerID != owner || item.DeletedAt != "" {
			continue
		}
		if item.Version != version {
			return ErrCloudConflict
		}
		next[i].ShareToken, next[i].ShareExpiresAt = "", ""
		if enabled {
			var token [32]byte
			if _, err := rand.Read(token[:]); err != nil {
				return err
			}
			next[i].ShareToken = base64.RawURLEncoding.EncodeToString(token[:])
			if days > 0 {
				next[i].ShareExpiresAt = time.Now().UTC().AddDate(0, 0, days).Format(time.RFC3339)
			}
		}
		next[i].Version++
		next[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		return saveCloud(next)
	}
	return ErrCloudNotFound
}

func sharedCloudScope(token, id string) (models.CloudItem, models.CloudItem, map[string]int, error) {
	if len(token) != 43 {
		return models.CloudItem{}, models.CloudItem{}, nil, ErrCloudNotFound
	}
	var root models.CloudItem
	for _, item := range cloudItems {
		if subtle.ConstantTimeCompare([]byte(item.ShareToken), []byte(token)) == 1 {
			root = item
			break
		}
	}
	if root.ID == "" || root.DeletedAt != "" {
		return root, models.CloudItem{}, nil, ErrCloudNotFound
	}
	if root.ShareExpiresAt != "" {
		expires, err := time.Parse(time.RFC3339, root.ShareExpiresAt)
		if err != nil || !time.Now().Before(expires) {
			return root, models.CloudItem{}, nil, ErrCloudNotFound
		}
	}
	if id == "" {
		id = root.ID
	}
	index := cloudIndex(cloudItems)
	i, ok := index[id]
	if !ok {
		return root, models.CloudItem{}, nil, ErrCloudNotFound
	}
	current := cloudItems[i]
	for at, depth := id, 0; at != "" && depth < 32; depth++ {
		i, ok := index[at]
		if !ok {
			break
		}
		item := cloudItems[i]
		if item.OwnerID != root.OwnerID || item.DeletedAt != "" {
			break
		}
		if at == root.ID {
			return root, current, index, nil
		}
		at = item.ParentID
	}
	return root, models.CloudItem{}, nil, ErrCloudNotFound
}

type SharedCloudItem struct {
	ID, Name, Kind, MIME string
	Size                 int64
}

func sharedCloudItem(item models.CloudItem) SharedCloudItem {
	return SharedCloudItem{ID: item.ID, Name: item.Name, Kind: item.Kind, MIME: item.MIME, Size: item.Size}
}

type SharedCloudView struct {
	OwnerID            string
	Current            SharedCloudItem
	Breadcrumbs, Items []SharedCloudItem
}

func GetSharedCloud(token, id string) (SharedCloudView, error) {
	cloudMutex.RLock()
	defer cloudMutex.RUnlock()
	root, current, index, err := sharedCloudScope(token, id)
	if err != nil {
		return SharedCloudView{}, err
	}
	view := SharedCloudView{OwnerID: root.OwnerID, Current: sharedCloudItem(current)}
	for at := current; ; at = cloudItems[index[at.ParentID]] {
		view.Breadcrumbs = append([]SharedCloudItem{sharedCloudItem(at)}, view.Breadcrumbs...)
		if at.ID == root.ID {
			break
		}
	}
	if current.Kind == "folder" {
		for _, item := range cloudItems {
			if item.ParentID == current.ID && item.OwnerID == root.OwnerID && item.DeletedAt == "" {
				view.Items = append(view.Items, sharedCloudItem(item))
			}
		}
	}
	sort.Slice(view.Items, func(i, j int) bool {
		a, b := view.Items[i], view.Items[j]
		if a.Kind != b.Kind {
			return a.Kind == "folder"
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return view, nil
}

func OpenSharedCloudFile(token, id string) (models.CloudItem, *os.File, error) {
	cloudMutex.RLock()
	defer cloudMutex.RUnlock()
	_, item, _, err := sharedCloudScope(token, id)
	if err != nil || item.Kind != "file" {
		return models.CloudItem{}, nil, ErrCloudNotFound
	}
	file, err := OpenCloudBlob(CloudDirectory(), item.ID)
	return item, file, err
}

func WriteSharedCloudArchive(token, id string, writer io.Writer) error {
	cloudMutex.RLock()
	defer cloudMutex.RUnlock()
	root, current, _, err := sharedCloudScope(token, id)
	if err != nil {
		return err
	}
	return writeCloudArchiveLocked(root.OwnerID, []string{current.ID}, writer)
}
