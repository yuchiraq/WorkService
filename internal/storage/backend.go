package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"project/internal/persistence"
)

// Configured once, before loading domain caches or starting HTTP/scheduler goroutines.
var backend *persistence.Store
var backendName = "JSON"

func ConfigureFromEnv() (close func() error, err error) {
	unlock, err := persistence.LockDirectory("storage")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = unlock()
		}
	}()
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("WORKSERVICE_STORAGE")))
	if mode == "" || mode == "json" {
		backend = persistence.OpenJSON("storage")
		backendName = "JSON"
		return func() error { return errors.Join(backend.Close(), unlock()) }, nil
	}
	if mode != "sqlite" {
		return nil, fmt.Errorf("unknown WORKSERVICE_STORAGE %q", mode)
	}
	path := strings.TrimSpace(os.Getenv("WORKSERVICE_SQLITE_PATH"))
	if path == "" {
		path = "storage/workservice.db"
	}
	db, err := persistence.OpenSQLite(path, false)
	if err != nil {
		return nil, fmt.Errorf("open SQLite (import JSON first): %w", err)
	}
	if _, err = db.Read("users.json"); err != nil {
		_ = db.Close()
		return nil, errors.New("SQLite has no imported users; refusing to start")
	}
	backend, backendName = db, "SQLite"
	return func() error { return errors.Join(db.Close(), unlock()) }, nil
}

func BackendName() string { return backendName }

func readData(path string) ([]byte, error) {
	if backend == nil {
		return os.ReadFile(path)
	}
	return backend.Read(filepath.Base(path))
}

func writeData(path string, data []byte) error {
	if backend == nil {
		return persistence.AtomicWrite(path, data)
	}
	return backend.Write(filepath.Base(path), data)
}
