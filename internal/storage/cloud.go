package storage

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
	"project/internal/models"
)

var (
	cloudMutex           sync.RWMutex
	cloudItems           []models.CloudItem
	cloudFile                  = "storage/cloud.json"
	cloudQuota           int64 = 10 << 30
	cloudFileLimit       int64 = 256 << 20
	ErrCloudNotFound           = errors.New("Файл или папка недоступны")
	ErrCloudConflict           = errors.New("Данные изменились в другом окне. Обновите список и повторите действие")
	ErrCloudName               = errors.New("Введите имя до 180 символов без / \\ : * ? < > | и кавычек")
	ErrCloudDuplicate          = errors.New("В этой папке уже есть файл или папка с таким именем")
	ErrCloudQuota              = errors.New("В хранилище недостаточно места. Освободите корзину или удалите ненужные файлы")
	ErrCloudSize               = errors.New("Файл превышает допустимый размер")
	ErrCloudMove               = errors.New("Нельзя переместить папку в саму себя или в её вложенную папку")
	ErrCloudLimit              = errors.New("Превышен лимит: 20 000 элементов или 32 уровня папок")
	ErrCloudArchiveLimit       = errors.New("Для одного ZIP выберите не более 2000 элементов общим размером до 2 ГБ")
)

type CloudRef struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
}
type CloudMutation struct {
	Action   string     `json:"action"`
	Items    []CloudRef `json:"items"`
	ParentID string     `json:"parentId"`
	Name     string     `json:"name"`
	Starred  bool       `json:"starred"`
}
type CloudSnapshot struct {
	Items        []models.CloudItem `json:"items"`
	UsedBytes    int64              `json:"usedBytes"`
	TrashBytes   int64              `json:"trashBytes"`
	QuotaBytes   int64              `json:"quotaBytes"`
	MaxFileBytes int64              `json:"maxFileBytes"`
}

func CloudDirectory() string {
	if dir := strings.TrimSpace(os.Getenv("WORKSERVICE_CLOUD_DIR")); dir != "" {
		return dir
	}
	return "storage/cloud"
}

// Resolve existing ancestors too: a new directory can be below a symlink.
func resolveCloudDirectory(path string) (string, error) {
	current, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) || filepath.Dir(current) == current {
			return "", err
		}
		suffix = append(suffix, filepath.Base(current))
		current = filepath.Dir(current)
	}
}

func LoadCloud() error {
	cloudMutex.Lock()
	defer cloudMutex.Unlock()
	quota, limit := int64(10<<30), int64(256<<20)
	for key, target := range map[string]*int64{"WORKSERVICE_CLOUD_QUOTA_MB": &quota, "WORKSERVICE_CLOUD_MAX_FILE_MB": &limit} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil || n < 1 || n > 1_048_576 {
				return fmt.Errorf("%s must be 1..1048576", key)
			}
			*target = n << 20
		}
	}
	if limit > quota {
		return errors.New("cloud file limit exceeds quota")
	}
	root, err := resolveCloudDirectory(CloudDirectory())
	if err != nil {
		return err
	}
	public, err := resolveCloudDirectory("web/static")
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(public, root)
	if err == nil && (rel == "." || filepath.IsLocal(rel)) {
		return errors.New("cloud directory must not be inside web/static")
	}
	data, err := readData(cloudFile)
	var next []models.CloudItem
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if err = json.Unmarshal(data, &next); err != nil {
			return err
		}
	}
	if err = ValidateCloudMetadata(next); err != nil {
		return err
	}
	cloudItems, cloudQuota, cloudFileLimit = next, quota, limit
	return nil
}

func CloudMaxFileBytes() int64 { cloudMutex.RLock(); defer cloudMutex.RUnlock(); return cloudFileLimit }

func CloudState(owner string) CloudSnapshot {
	cloudMutex.RLock()
	defer cloudMutex.RUnlock()
	state := CloudSnapshot{Items: []models.CloudItem{}, QuotaBytes: cloudQuota, MaxFileBytes: cloudFileLimit}
	for _, item := range cloudItems {
		if item.OwnerID == owner {
			state.Items = append(state.Items, item)
			state.UsedBytes += item.Size
			if item.DeletedAt != "" {
				state.TrashBytes += item.Size
			}
		}
	}
	return state
}

func cloudIndex(items []models.CloudItem) map[string]int {
	result := make(map[string]int, len(items))
	for i, item := range items {
		result[item.ID] = i
	}
	return result
}

func cleanCloudName(name string) (string, error) {
	name = norm.NFC.String(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." || strings.HasSuffix(name, ".") || utf8.RuneCountInString(name) > 180 || strings.ContainsAny(name, `/\:*?<>|"`) {
		return "", ErrCloudName
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", ErrCloudName
		}
	}
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
		return "", ErrCloudName
	}
	return name, nil
}

func ValidateCloudMetadata(items []models.CloudItem) error {
	index := cloudIndex(items)
	if len(index) != len(items) {
		return errors.New("duplicate cloud id")
	}
	names := map[string]bool{}
	for _, item := range items {
		if _, err := uuid.Parse(item.ID); err != nil || len(item.ID) != 36 || item.OwnerID == "" || item.Version < 1 {
			return errors.New("invalid cloud identity")
		}
		if name, err := cleanCloudName(item.Name); err != nil || name != item.Name {
			return errors.New("invalid cloud name")
		}
		if item.Kind != "file" && item.Kind != "folder" {
			return errors.New("invalid cloud kind")
		}
		if item.Size < 0 || item.Size > 1<<40 || (item.Kind == "folder" && item.Size != 0) {
			return errors.New("invalid cloud size")
		}
		if item.Kind == "file" {
			hash, err := hex.DecodeString(item.SHA256)
			if err != nil || len(hash) != 32 {
				return errors.New("invalid cloud checksum")
			}
		}
		if item.DeletedAt == "" {
			key := item.OwnerID + "\x00" + item.ParentID + "\x00" + strings.ToLower(item.Name)
			if names[key] {
				return ErrCloudDuplicate
			}
			names[key] = true
			if item.TrashRootID != "" {
				return errors.New("invalid trash root")
			}
		} else {
			i, ok := index[item.TrashRootID]
			if !ok || items[i].OwnerID != item.OwnerID || items[i].DeletedAt == "" || items[i].TrashRootID != items[i].ID {
				return errors.New("invalid trash group")
			}
		}
		parent := item.ParentID
		seen := map[string]bool{item.ID: true}
		for depth := 0; parent != ""; depth++ {
			if depth >= 31 || seen[parent] {
				return ErrCloudLimit
			}
			seen[parent] = true
			i, ok := index[parent]
			if !ok {
				if item.DeletedAt != "" {
					break
				}
				return errors.New("missing cloud parent")
			}
			p := items[i]
			if p.OwnerID != item.OwnerID || p.Kind != "folder" || (item.DeletedAt == "" && p.DeletedAt != "") {
				return errors.New("invalid cloud parent")
			}
			parent = p.ParentID
		}
		if item.DeletedAt != "" && !seen[item.TrashRootID] {
			return errors.New("cloud trash group is not an ancestor")
		}
	}
	return nil
}

func cloudParent(items []models.CloudItem, owner, parent string) error {
	if owner == "" {
		return ErrCloudNotFound
	}
	if parent == "" {
		return nil
	}
	for _, item := range items {
		if item.ID == parent && item.OwnerID == owner && item.Kind == "folder" && item.DeletedAt == "" {
			return nil
		}
	}
	return ErrCloudNotFound
}

func cloudNameExists(items []models.CloudItem, owner, parent, name, except string) bool {
	for _, item := range items {
		if item.ID != except && item.OwnerID == owner && item.ParentID == parent && item.DeletedAt == "" && strings.EqualFold(item.Name, name) {
			return true
		}
	}
	return false
}

func freeCloudName(items []models.CloudItem, owner, parent, name string) string {
	if !cloudNameExists(items, owner, parent, name, "") {
		return name
	}
	ext := filepath.Ext(name)
	if len([]rune(ext)) > 30 {
		ext = ""
	}
	base := strings.TrimSuffix(name, ext)
	if len([]rune(base)) > 130 {
		base = string([]rune(base)[:130])
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, n, ext)
		if !cloudNameExists(items, owner, parent, candidate, "") {
			return candidate
		}
	}
}

func saveCloud(next []models.CloudItem) error {
	if err := ValidateCloudMetadata(next); err != nil {
		return err
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err = writeData(cloudFile, data); err != nil {
		return err
	}
	cloudItems = next
	return nil
}

func CloudBlobPath(directory, id string) (string, error) {
	if _, err := uuid.Parse(id); err != nil || len(id) != 36 {
		return "", ErrCloudNotFound
	}
	return filepath.Join(directory, id+".bin"), nil
}

func OpenCloudBlob(directory, id string) (*os.File, error) {
	path, err := CloudBlobPath(directory, id)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrCloudNotFound
	}
	return os.Open(path)
}

func OpenCloudFile(owner, id string) (models.CloudItem, *os.File, error) {
	cloudMutex.RLock()
	defer cloudMutex.RUnlock()
	for _, item := range cloudItems {
		if item.ID == id && item.OwnerID == owner && item.Kind == "file" && item.DeletedAt == "" {
			file, err := OpenCloudBlob(CloudDirectory(), id)
			return item, file, err
		}
	}
	return models.CloudItem{}, nil, ErrCloudNotFound
}

func AddCloudFile(owner, parent, name, uploadID string, reader io.Reader) (models.CloudItem, error) {
	name, err := cleanCloudName(name)
	if err != nil {
		return models.CloudItem{}, err
	}
	if _, err = uuid.Parse(uploadID); err != nil || len(uploadID) != 36 {
		return models.CloudItem{}, ErrCloudConflict
	}
	cloudMutex.RLock()
	err = cloudParent(cloudItems, owner, parent)
	for _, item := range cloudItems {
		if item.ID == uploadID {
			cloudMutex.RUnlock()
			if item.OwnerID == owner && item.Kind == "file" && item.DeletedAt == "" {
				return item, nil
			}
			return models.CloudItem{}, ErrCloudConflict
		}
	}
	limit := cloudFileLimit
	cloudMutex.RUnlock()
	if err != nil {
		return models.CloudItem{}, err
	}
	if err = os.MkdirAll(CloudDirectory(), 0700); err != nil {
		return models.CloudItem{}, err
	}
	temp, err := os.CreateTemp(CloudDirectory(), ".upload-*.tmp")
	if err != nil {
		return models.CloudItem{}, err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(temp, hash), io.LimitReader(reader, limit+1))
	if err != nil {
		return models.CloudItem{}, err
	}
	if size > limit {
		return models.CloudItem{}, ErrCloudSize
	}
	if _, err = temp.Seek(0, io.SeekStart); err != nil {
		return models.CloudItem{}, err
	}
	header := make([]byte, 512)
	n, err := temp.Read(header)
	if err != nil && err != io.EOF {
		return models.CloudItem{}, err
	}
	mime := http.DetectContentType(header[:n])
	if err = temp.Sync(); err != nil {
		return models.CloudItem{}, err
	}
	if err = temp.Close(); err != nil {
		return models.CloudItem{}, err
	}
	cloudMutex.Lock()
	defer cloudMutex.Unlock()
	if err = cloudParent(cloudItems, owner, parent); err != nil {
		return models.CloudItem{}, err
	}
	used, count := int64(0), 0
	for _, item := range cloudItems {
		if item.ID == uploadID {
			if item.OwnerID == owner && item.Kind == "file" && item.DeletedAt == "" {
				return item, nil
			}
			return models.CloudItem{}, ErrCloudConflict
		}
		if item.OwnerID == owner {
			used += item.Size
			count++
		}
	}
	if used+size > cloudQuota {
		return models.CloudItem{}, ErrCloudQuota
	}
	if count >= 20000 {
		return models.CloudItem{}, ErrCloudLimit
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	item := models.CloudItem{ID: uploadID, OwnerID: owner, ParentID: parent, Name: freeCloudName(cloudItems, owner, parent, name), Kind: "file", MIME: mime, Size: size, SHA256: hex.EncodeToString(hash.Sum(nil)), Version: 1, CreatedAt: now, UpdatedAt: now}
	path, _ := CloudBlobPath(CloudDirectory(), item.ID)
	if _, err = os.Lstat(path); !os.IsNotExist(err) {
		return models.CloudItem{}, ErrCloudConflict
	}
	if err = os.Rename(temp.Name(), path); err != nil {
		return models.CloudItem{}, err
	}
	next := append(append([]models.CloudItem{}, cloudItems...), item)
	if err = saveCloud(next); err != nil {
		_ = os.Remove(path)
		return models.CloudItem{}, err
	}
	return item, nil
}

func ChangeCloud(owner string, mutation CloudMutation) error {
	cloudMutex.Lock()
	defer cloudMutex.Unlock()
	if owner == "" {
		return ErrCloudNotFound
	}
	next := append([]models.CloudItem{}, cloudItems...)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	index := cloudIndex(next)
	if mutation.Action == "folder" {
		if err := cloudParent(next, owner, mutation.ParentID); err != nil {
			return err
		}
		name, err := cleanCloudName(mutation.Name)
		if err != nil {
			return err
		}
		if cloudNameExists(next, owner, mutation.ParentID, name, "") {
			return ErrCloudDuplicate
		}
		count := 0
		for _, item := range next {
			if item.OwnerID == owner {
				count++
			}
		}
		if count >= 20000 {
			return ErrCloudLimit
		}
		next = append(next, models.CloudItem{ID: uuid.NewString(), OwnerID: owner, ParentID: mutation.ParentID, Name: name, Kind: "folder", CreatedAt: now, UpdatedAt: now, Version: 1})
		return saveCloud(next)
	}
	if len(mutation.Items) == 0 || len(mutation.Items) > 500 {
		return ErrCloudConflict
	}
	selected := map[string]bool{}
	trashAction := mutation.Action == "restore" || mutation.Action == "purge"
	for _, ref := range mutation.Items {
		i, ok := index[ref.ID]
		if !ok || next[i].OwnerID != owner {
			return ErrCloudNotFound
		}
		item := next[i]
		if (item.DeletedAt != "") != trashAction || (trashAction && item.TrashRootID != item.ID) {
			return ErrCloudNotFound
		}
		if item.Version != ref.Version {
			return ErrCloudConflict
		}
		selected[item.ID] = true
	}
	// A selected folder already covers its active descendants.
	if mutation.Action == "move" || mutation.Action == "trash" {
		for id := range selected {
			parent := next[index[id]].ParentID
			for parent != "" {
				if selected[parent] {
					delete(selected, id)
					break
				}
				i, ok := index[parent]
				if !ok {
					break
				}
				parent = next[i].ParentID
			}
		}
	}
	roots := make([]string, 0, len(selected))
	for id := range selected {
		roots = append(roots, id)
	}
	sort.Strings(roots)
	bump := func(i int) { next[i].Version++; next[i].UpdatedAt = now }
	purge := map[string]bool{}
	switch mutation.Action {
	case "rename":
		if len(mutation.Items) != 1 {
			return ErrCloudConflict
		}
		i := index[roots[0]]
		name, err := cleanCloudName(mutation.Name)
		if err != nil {
			return err
		}
		if cloudNameExists(next, owner, next[i].ParentID, name, next[i].ID) {
			return ErrCloudDuplicate
		}
		next[i].Name = name
		bump(i)
	case "star":
		for id := range selected {
			i := index[id]
			next[i].Starred = mutation.Starred
			bump(i)
		}
	case "move":
		if err := cloudParent(next, owner, mutation.ParentID); err != nil {
			return err
		}
		for parent := mutation.ParentID; parent != ""; {
			if selected[parent] {
				return ErrCloudMove
			}
			parent = next[index[parent]].ParentID
		}
		for _, id := range roots {
			i := index[id]
			if cloudNameExists(next, owner, mutation.ParentID, next[i].Name, id) {
				return ErrCloudDuplicate
			}
			next[i].ParentID = mutation.ParentID
			bump(i)
		}
	case "trash":
		for i, item := range next {
			if item.OwnerID != owner || item.DeletedAt != "" {
				continue
			}
			for id := item.ID; id != ""; id = next[index[id]].ParentID {
				if selected[id] {
					next[i].DeletedAt = now
					next[i].TrashRootID = id
					bump(i)
					break
				}
			}
		}
	case "restore":
		for _, id := range roots {
			i := index[id]
			if cloudParent(next, owner, next[i].ParentID) != nil {
				next[i].ParentID = ""
			}
			next[i].Name = freeCloudName(next, owner, next[i].ParentID, next[i].Name)
			for j := range next {
				if next[j].OwnerID == owner && next[j].TrashRootID == id {
					next[j].DeletedAt = ""
					next[j].TrashRootID = ""
					bump(j)
				}
			}
		}
	case "purge":
		kept := []models.CloudItem{}
		for _, item := range next {
			if item.OwnerID == owner && selected[item.TrashRootID] {
				if item.Kind == "file" {
					purge[item.ID] = true
				}
				continue
			}
			kept = append(kept, item)
		}
		next = kept
	default:
		return ErrCloudConflict
	}
	if err := saveCloud(next); err != nil {
		return err
	}
	for id := range purge {
		path, _ := CloudBlobPath(CloudDirectory(), id)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Printf("cloud orphan cleanup %s: %v", id, err)
		}
	}
	return nil
}

// Build archives on disk, not over a slow client connection, while pinning metadata.
func WriteCloudArchive(owner string, ids []string, writer io.Writer) error {
	cloudMutex.RLock()
	defer cloudMutex.RUnlock()
	if len(ids) == 0 || len(ids) > 500 {
		return ErrCloudNotFound
	}
	index := cloudIndex(cloudItems)
	selected := map[string]bool{}
	for _, id := range ids {
		i, ok := index[id]
		if !ok || cloudItems[i].OwnerID != owner || cloudItems[i].DeletedAt != "" {
			return ErrCloudNotFound
		}
		selected[id] = true
	}
	for id := range selected {
		for parent := cloudItems[index[id]].ParentID; parent != ""; parent = cloudItems[index[parent]].ParentID {
			if selected[parent] {
				delete(selected, id)
				break
			}
		}
	}
	paths := map[string]string{}
	size := int64(0)
	count := 0
	rootNames := map[string]string{}
	rootIDs := make([]string, 0, len(selected))
	for id := range selected {
		rootIDs = append(rootIDs, id)
	}
	sort.Strings(rootIDs)
	used := map[string]bool{}
	for _, id := range rootIDs {
		name := cloudItems[index[id]].Name
		candidate := name
		for n := 2; used[strings.ToLower(candidate)]; n++ {
			candidate = fmt.Sprintf("%s (%d)", name, n)
		}
		used[strings.ToLower(candidate)] = true
		rootNames[id] = candidate
	}
	for _, item := range cloudItems {
		if item.OwnerID != owner || item.DeletedAt != "" {
			continue
		}
		parts := []string{}
		current := item.ID
		for current != "" {
			if selected[current] {
				parts = append(parts, rootNames[current])
				for l, r := 0, len(parts)-1; l < r; l, r = l+1, r-1 {
					parts[l], parts[r] = parts[r], parts[l]
				}
				paths[item.ID] = strings.Join(parts, "/")
				size += item.Size
				count++
				break
			}
			node := cloudItems[index[current]]
			parts = append(parts, node.Name)
			current = node.ParentID
		}
	}
	if size > 2<<30 || count > 2000 {
		return ErrCloudArchiveLimit
	}
	archive := zip.NewWriter(writer)
	for _, item := range cloudItems {
		name, ok := paths[item.ID]
		if !ok {
			continue
		}
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.SetModTime(time.Now())
		if item.Kind == "folder" {
			header.Name += "/"
			header.SetMode(os.ModeDir | 0755)
		} else {
			header.SetMode(0600)
		}
		entry, err := archive.CreateHeader(header)
		if err != nil {
			_ = archive.Close()
			return err
		}
		if item.Kind == "file" {
			file, err := OpenCloudBlob(CloudDirectory(), item.ID)
			if err != nil {
				_ = archive.Close()
				return err
			}
			_, err = io.Copy(entry, file)
			_ = file.Close()
			if err != nil {
				_ = archive.Close()
				return err
			}
		}
	}
	return archive.Close()
}
