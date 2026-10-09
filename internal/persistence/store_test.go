package persistence

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSQLiteLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "данные with spaces.db")
	if _, err := OpenSQLite(path, false); !os.IsNotExist(err) {
		t.Fatalf("opening missing db: %v", err)
	}
	db, err := OpenSQLite(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = OpenSQLite(path, false); err == nil {
		t.Fatal("second writer accepted")
	}
	if err = db.Import(map[string][]byte{"users.json": []byte(`[{"id":"u","value":9007199254740993}]`), "inventory.json": []byte(`{"items":[],"history":[]}`)}); err != nil {
		t.Fatal(err)
	}
	if err = db.Import(map[string][]byte{"objects.json": []byte(`[]`)}); err == nil {
		t.Fatal("second import accepted")
	}
	if err = db.Write("../escape", []byte(`[]`)); err == nil {
		t.Fatal("unknown document accepted")
	}
	if err = db.Write("users.json", []byte(`broken`)); err == nil {
		t.Fatal("invalid JSON accepted")
	}
	want, err := db.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err = db.IntegrityCheck(); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenSQLite(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for name, data := range want {
		got, err := reopened.Read(name)
		if err != nil || string(got) != string(data) {
			t.Fatalf("roundtrip %s: %v", name, err)
		}
	}
	if _, err = reopened.Read("workers.json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestSQLiteImportRollback(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "data.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Import(map[string][]byte{"users.json": []byte(`[]`), "objects.json": []byte(`invalid`)}); err == nil {
		t.Fatal("corrupt import accepted")
	}
	snapshot, err := db.Snapshot()
	if err != nil || len(snapshot) != 0 {
		t.Fatal("partial import", snapshot, err)
	}
}

func TestConcurrentSnapshot(t *testing.T) {
	for _, mode := range []string{"json", "sqlite"} {
		t.Run(mode, func(t *testing.T) {
			var db *Store
			var err error
			if mode == "json" {
				db = OpenJSON(t.TempDir())
			} else {
				db, err = OpenSQLite(filepath.Join(t.TempDir(), "data.db"), true)
			}
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var wg sync.WaitGroup
			for i := 0; i < 12; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for n := 0; n < 12; n++ {
						if e := db.Write("users.json", []byte(`[{"id":"u"}]`)); e != nil {
							t.Error(e)
							return
						}
						data, e := db.Snapshot()
						if e != nil || string(data["users.json"]) != `[{"id":"u"}]` {
							t.Error("invalid concurrent snapshot", e)
							return
						}
					}
				}()
			}
			wg.Wait()
		})
	}
}
