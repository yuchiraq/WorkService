package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/net/webdav"
	"golang.org/x/text/cases"
	"project/internal/models"
)

// Each DAV request edits a private snapshot. Nothing is published until the
// protocol handler succeeds and Commit verifies that the website has not edited it.
type CloudDAV struct {
	owner          string
	initial, items []models.CloudItem
	staged         map[string]string
	quota, limit   int64
	writable       bool
	dirty          bool
	lookup         map[string]int
	upload         bool
	retired        []string
}

// TrackUploadPlaceholders distinguishes initial PUT/LOCK files from empty files
// uploaded through the website or copied from existing documents.
func (fs *CloudDAV) TrackUploadPlaceholders() { fs.upload = true }

func NewCloudDAV(owner string, writable bool) *CloudDAV {
	s := CloudState(owner)
	return &CloudDAV{owner: owner, initial: s.Items, items: append([]models.CloudItem{}, s.Items...), staged: map[string]string{}, quota: s.QuotaBytes, limit: s.MaxFileBytes, writable: writable}
}

func (fs *CloudDAV) Close() {
	for _, temp := range fs.staged {
		_ = os.Remove(temp)
	}
}

func (fs *CloudDAV) Commit(ctx context.Context) error {
	if !fs.dirty {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	cloudMutex.Lock()
	defer cloudMutex.Unlock()
	current, next := []models.CloudItem{}, []models.CloudItem{}
	for _, item := range cloudItems {
		if item.OwnerID == fs.owner {
			current = append(current, item)
		} else {
			next = append(next, item)
		}
	}
	if !reflect.DeepEqual(current, fs.initial) {
		return ErrCloudConflict
	}
	if err := fs.check(); err != nil {
		return err
	}
	next = append(next, fs.items...)
	if err := ValidateCloudMetadata(next); err != nil {
		return err
	}
	created := []string{}
	rollback := func() {
		for _, p := range created {
			_ = os.Remove(p)
		}
	}
	for id, temp := range fs.staged {
		p, _ := CloudBlobPath(CloudDirectory(), id)
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			rollback()
			return ErrCloudConflict
		}
		if err := os.Rename(temp, p); err != nil {
			rollback()
			return err
		}
		created = append(created, p)
	}
	if err := saveCloud(next); err != nil {
		rollback()
		return err
	}
	fs.staged = map[string]string{}
	// Remove superseded empty blobs only after metadata is durably committed.
	for _, id := range fs.retired {
		p, _ := CloudBlobPath(CloudDirectory(), id)
		_ = os.Remove(p)
	}
	return nil
}

func (fs *CloudDAV) check() error {
	if len(fs.items) > 20000 {
		return ErrCloudLimit
	}
	var total int64
	for _, item := range fs.items {
		total += item.Size
	}
	if total > fs.quota {
		return ErrCloudQuota
	}
	return ValidateCloudMetadata(fs.items)
}

func (fs *CloudDAV) resolve(name string) (models.CloudItem, error) {
	root := models.CloudItem{OwnerID: fs.owner, Name: "/", Kind: "folder", UpdatedAt: "2000-01-01T00:00:00Z"}
	if name == "" || name == "/" {
		return root, nil
	}
	if strings.Contains(name, "\\") || len(name) > 8192 {
		return models.CloudItem{}, os.ErrNotExist
	}
	parts := strings.Split(strings.Trim(name, "/"), "/")
	if len(parts) > 32 {
		return models.CloudItem{}, os.ErrNotExist
	}
	current := root
	if fs.lookup == nil {
		fs.lookup = make(map[string]int, len(fs.items))
		for i, item := range fs.items {
			if item.DeletedAt == "" {
				fs.lookup[item.ParentID+"\x00"+cases.Fold().String(item.Name)] = i
			}
		}
	}
	for _, part := range parts {
		clean, err := cleanCloudName(part)
		if err != nil || clean != part || current.Kind != "folder" {
			return models.CloudItem{}, os.ErrNotExist
		}
		i, found := fs.lookup[current.ID+"\x00"+cases.Fold().String(part)]
		if !found {
			return models.CloudItem{}, os.ErrNotExist
		}
		current = fs.items[i]
	}
	return current, nil
}

func (fs *CloudDAV) parent(name string) (models.CloudItem, string, error) {
	name = strings.TrimSuffix(name, "/")
	base := path.Base(name)
	clean, err := cleanCloudName(base)
	if err != nil || clean != base || name == "/" || name == "" {
		return models.CloudItem{}, "", os.ErrInvalid
	}
	p, err := fs.resolve(path.Dir(name))
	if err != nil || p.Kind != "folder" {
		return models.CloudItem{}, "", os.ErrNotExist
	}
	return p, base, nil
}

func (fs *CloudDAV) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	item, err := fs.resolve(name)
	if err != nil {
		return nil, err
	}
	return davInfo{item}, nil
}

func (fs *CloudDAV) Mkdir(ctx context.Context, name string, _ os.FileMode) error {
	if !fs.writable {
		return os.ErrPermission
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	parent, base, err := fs.parent(name)
	if err != nil {
		return err
	}
	if _, err = fs.resolve(name); err == nil {
		return os.ErrExist
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	fs.items = append(fs.items, models.CloudItem{ID: uuid.NewString(), OwnerID: fs.owner, ParentID: parent.ID, Name: base, Kind: "folder", Version: 1, CreatedAt: now, UpdatedAt: now})
	fs.lookup = nil
	fs.dirty = true
	return fs.check()
}

func (fs *CloudDAV) RemoveAll(ctx context.Context, name string) error {
	if !fs.writable {
		return os.ErrPermission
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	item, err := fs.resolve(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if item.ID == "" {
		return os.ErrPermission
	}
	fs.trash(item.ID)
	return nil
}

func (fs *CloudDAV) trash(root string) {
	fs.lookup = nil
	index := cloudIndex(fs.items)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i, item := range fs.items {
		if item.DeletedAt != "" {
			continue
		}
		for id := item.ID; id != ""; id = fs.items[index[id]].ParentID {
			if id == root {
				fs.items[i].DeletedAt, fs.items[i].TrashRootID = now, root
				fs.items[i].ShareToken, fs.items[i].ShareExpiresAt = "", ""
				fs.items[i].UpdatedAt = now
				fs.items[i].Version++
				break
			}
		}
	}
	fs.dirty = true
}

func (fs *CloudDAV) Rename(ctx context.Context, oldName, newName string) error {
	if !fs.writable {
		return os.ErrPermission
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	source, err := fs.resolve(oldName)
	if err != nil {
		return err
	}
	if source.ID == "" {
		return os.ErrPermission
	}
	parent, base, err := fs.parent(newName)
	if err != nil {
		return err
	}
	index := cloudIndex(fs.items)
	for id := parent.ID; id != ""; id = fs.items[index[id]].ParentID {
		if id == source.ID {
			return ErrCloudMove
		}
	}
	if other, e := fs.resolve(newName); e == nil && other.ID != source.ID {
		return os.ErrExist
	}
	i := index[source.ID]
	fs.items[i].ParentID, fs.items[i].Name = parent.ID, base
	fs.items[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	fs.items[i].Version++
	fs.lookup = nil
	fs.dirty = true
	return fs.check()
}

func (fs *CloudDAV) OpenFile(ctx context.Context, name string, flag int, _ os.FileMode) (webdav.File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	item, err := fs.resolve(name)
	write := flag&(os.O_RDWR|os.O_WRONLY) != 0
	if !write {
		if err != nil {
			return nil, err
		}
		f := &davFile{fs: fs, ctx: ctx, item: item}
		if item.Kind == "folder" {
			for _, child := range fs.items {
				if child.ParentID == item.ID && child.DeletedAt == "" {
					f.children = append(f.children, davInfo{child})
				}
			}
			sort.Slice(f.children, func(i, j int) bool { return f.children[i].Name() < f.children[j].Name() })
		} else if temp := fs.staged[item.ID]; temp != "" {
			f.file, err = os.Open(temp)
		} else {
			f.file, err = OpenCloudBlob(CloudDirectory(), item.ID)
		}
		return f, err
	}
	if !fs.writable {
		return nil, os.ErrPermission
	}
	if err == nil && item.Kind == "folder" {
		return nil, os.ErrPermission
	}
	if flag&os.O_CREATE == 0 && err != nil {
		return nil, err
	}
	if flag&os.O_EXCL != 0 && err == nil {
		return nil, os.ErrExist
	}
	if flag&os.O_TRUNC == 0 && err == nil {
		return nil, os.ErrPermission
	}
	parent, base, err := fs.parent(name)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(CloudDirectory(), 0700); err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp(CloudDirectory(), ".dav-*.tmp")
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	next := models.CloudItem{ID: uuid.NewString(), OwnerID: fs.owner, ParentID: parent.ID, Name: base, Kind: "file", Version: 1, CreatedAt: now, UpdatedAt: now}
	fs.staged[next.ID] = temp.Name()
	var used int64
	for _, existing := range fs.items {
		used += existing.Size
	}
	return &davFile{fs: fs, ctx: ctx, item: next, previous: item.ID, file: temp, writable: true, available: fs.quota - used}, nil
}

type davInfo struct{ item models.CloudItem }

func (i davInfo) Name() string { return i.item.Name }
func (i davInfo) Size() int64  { return i.item.Size }
func (i davInfo) Mode() os.FileMode {
	if i.IsDir() {
		return os.ModeDir | 0700
	}
	return 0600
}
func (i davInfo) ModTime() time.Time {
	t, _ := time.Parse(time.RFC3339Nano, i.item.UpdatedAt)
	return t
}
func (i davInfo) IsDir() bool { return i.item.Kind == "folder" }
func (i davInfo) Sys() any    { return nil }
func (i davInfo) ETag(context.Context) (string, error) {
	return fmt.Sprintf(`"%s-%d"`, i.item.ID, i.item.Version), nil
}

type davFile struct {
	fs               *CloudDAV
	ctx              context.Context
	item             models.CloudItem
	previous         string
	file             *os.File
	children         []os.FileInfo
	writable, closed bool
	written          int64
	available        int64
	failure          error
}

func (f *davFile) Stat() (os.FileInfo, error) { return davInfo{f.item}, nil }
func (f *davFile) Read(p []byte) (int, error) {
	if f.file == nil {
		return 0, os.ErrInvalid
	}
	return f.file.Read(p)
}
func (f *davFile) Seek(n int64, whence int) (int64, error) {
	if f.file == nil {
		return 0, os.ErrInvalid
	}
	return f.file.Seek(n, whence)
}
func (f *davFile) Readdir(count int) ([]os.FileInfo, error) {
	if f.item.Kind != "folder" {
		return nil, os.ErrInvalid
	}
	if len(f.children) == 0 && count > 0 {
		return nil, io.EOF
	}
	if count <= 0 || count > len(f.children) {
		count = len(f.children)
	}
	result := f.children[:count]
	f.children = f.children[count:]
	return result, nil
}
func (f *davFile) Write(p []byte) (int, error) {
	if !f.writable || f.closed {
		return 0, os.ErrPermission
	}
	if err := f.ctx.Err(); err != nil {
		f.failure = err
		return 0, err
	}
	if f.written+int64(len(p)) > f.fs.limit {
		f.failure = ErrCloudSize
		return 0, f.failure
	}
	if f.written+int64(len(p)) > f.available {
		f.failure = ErrCloudQuota
		return 0, f.failure
	}
	n, err := f.file.Write(p)
	f.written += int64(n)
	f.item.Size = f.written
	if err != nil {
		f.failure = err
	}
	return n, err
}
func (f *davFile) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	if f.file == nil {
		return nil
	}
	if !f.writable {
		return f.file.Close()
	}
	defer f.file.Close()
	if f.failure != nil {
		return f.failure
	}
	if err := f.ctx.Err(); err != nil {
		return err
	}
	if _, err := f.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, f.file); err != nil {
		return err
	}
	f.item.SHA256 = hex.EncodeToString(hash.Sum(nil))
	if _, err := f.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	header := make([]byte, 512)
	n, err := f.file.Read(header)
	if err != nil && err != io.EOF {
		return err
	}
	f.item.MIME = http.DetectContentType(header[:n])
	if strings.EqualFold(path.Ext(f.item.Name), ".mov") && quickTimeHeader(header[:n]) {
		f.item.MIME = "video/quicktime"
	}
	if err = f.file.Sync(); err != nil {
		return err
	}
	if err = f.file.Close(); err != nil {
		return err
	}
	f.item.DAVPlaceholder = f.fs.upload && f.previous == "" && f.item.Size == 0
	// A short-lived, untouched DAV empty file is the client's upload reservation,
	// not a previous document version. Real replacements remain recoverable.
	if f.previous != "" {
		index := cloudIndex(f.fs.items)[f.previous]
		previous := f.fs.items[index]
		created, err := time.Parse(time.RFC3339Nano, previous.CreatedAt)
		age := time.Since(created)
		if f.fs.upload && previous.DAVPlaceholder && previous.Size == 0 && previous.Version == 1 &&
			!previous.Starred && previous.ShareToken == "" && err == nil && age >= 0 && age < 10*time.Minute {
			f.fs.items = append(f.fs.items[:index], f.fs.items[index+1:]...)
			f.fs.retired = append(f.fs.retired, previous.ID)
			f.item.CreatedAt = previous.CreatedAt
			f.item.DAVPlaceholder = f.item.Size == 0
		} else {
			f.fs.trash(f.previous)
		}
	}
	f.fs.items = append(f.fs.items, f.item)
	f.fs.lookup = nil
	f.fs.dirty = true
	return f.fs.check()
}
