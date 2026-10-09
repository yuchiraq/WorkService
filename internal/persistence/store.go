// Package persistence provides the reversible first stage of the SQLite migration.
// Domain aggregates retain their JSON representation until repositories move to SQL.
package persistence

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofrs/flock"
	_ "modernc.org/sqlite"
)

var Names = []string{"users.json", "workers.json", "objects.json", "timesheets.json", "improvements.json", "app_settings.json", "telegram_contacts.json", "vehicles.json", "inventory.json", "object_photos.json", "cloud.json"}

type Store struct {
	mu   sync.Mutex
	root string
	db   *sql.DB
	lock *flock.Flock
}

func OpenJSON(root string) *Store { return &Store{root: root} }

func LockDirectory(root string) (func() error, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("storage path is not a directory")
	}
	lock := flock.New(filepath.Join(root, ".workservice.lock"))
	ok, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("storage is in use; stop the server first")
	}
	return lock.Unlock, nil
}

// OpenSQLite never creates a database accidentally when opening a live deployment.
// The OS lock is held for the lifetime of the in-memory domain cache.
func OpenSQLite(path string, create bool) (*Store, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if create {
		if err = os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
			return nil, err
		}
	} else if _, err = os.Stat(absolute); err != nil {
		return nil, err
	}
	lock := flock.New(absolute + ".lock")
	locked, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, errors.New("database is in use; stop the server before migration or export")
	}
	if create {
		file, e := os.OpenFile(absolute, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			_ = lock.Unlock()
			return nil, e
		}
		_ = file.Close()
	}
	uriPath := filepath.ToSlash(absolute)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath}
	q := u.Query()
	q.Set("mode", "rw")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(FULL)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		_ = lock.Unlock()
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, lock: lock}
	fail := func(err error) (*Store, error) { _ = s.Close(); return nil, err }
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fail(err)
	}
	if create {
		_, err = db.Exec(`BEGIN IMMEDIATE;
CREATE TABLE documents (name TEXT PRIMARY KEY, payload TEXT NOT NULL CHECK(json_valid(payload)), revision INTEGER NOT NULL DEFAULT 1);
PRAGMA user_version = 1;
COMMIT;`)
		if err != nil {
			return fail(err)
		}
	} else if version != 1 {
		return fail(fmt.Errorf("unsupported database version %d", version))
	}
	if _, err = db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fail(err)
	}
	return s, nil
}

func (s *Store) Close() error {
	var err error
	if s.db != nil {
		err = s.db.Close()
	}
	if s.lock != nil {
		err = errors.Join(err, s.lock.Unlock())
	}
	return err
}

func validName(name string) bool {
	for _, n := range Names {
		if n == name {
			return true
		}
	}
	return false
}

func (s *Store) read(name string) ([]byte, error) {
	if !validName(name) {
		return nil, fmt.Errorf("unknown document %q", name)
	}
	if s.db == nil {
		return os.ReadFile(filepath.Join(s.root, name))
	}
	var data []byte
	err := s.db.QueryRow("SELECT payload FROM documents WHERE name = ?", name).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, os.ErrNotExist
	}
	return data, err
}

func (s *Store) Read(name string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(name)
}

func AtomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".data-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

func (s *Store) Write(name string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validName(name) || !json.Valid(data) {
		return errors.New("invalid document")
	}
	if s.db == nil {
		return AtomicWrite(filepath.Join(s.root, name), data)
	}
	_, err := s.db.Exec(`INSERT INTO documents(name,payload) VALUES(?,?)
ON CONFLICT(name) DO UPDATE SET payload=excluded.payload, revision=documents.revision+1`, name, string(data))
	return err
}

// Snapshot returns committed documents at one point in time; caller owns the bytes.
func (s *Store) Snapshot() (map[string][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make(map[string][]byte)
	for _, name := range Names {
		data, err := s.read(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		result[name] = data
	}
	return result, nil
}

func (s *Store) Import(documents map[string][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return errors.New("import requires SQLite")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRow("SELECT count(*) FROM documents").Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return errors.New("destination database is not empty")
	}
	for name, data := range documents {
		if !validName(name) || !json.Valid(data) {
			return fmt.Errorf("invalid document %s", name)
		}
		if _, err = tx.Exec("INSERT INTO documents(name,payload) VALUES(?,?)", name, string(data)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) IntegrityCheck() error {
	if s.db == nil {
		return nil
	}
	var result string
	if err := s.db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return errors.New(result)
	}
	return nil
}
