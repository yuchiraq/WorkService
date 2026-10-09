package main

import (
	"os"
	"path/filepath"
	"testing"

	"project/internal/persistence"
)

func TestMigrationCommands(t *testing.T) {
	source := t.TempDir()
	db := filepath.Join(t.TempDir(), "data.db")
	data := []byte(`[{"id":"u","username":"admin","password":"preserved","status":"admin"}]`)
	if err := os.WriteFile(filepath.Join(source, "users.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"check", "import", "verify"} {
		if err := run([]string{command, "--source", source, "--db", db}); err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		if command == "check" {
			if _, err := os.Stat(db); !os.IsNotExist(err) {
				t.Fatal("check created database")
			}
		}
	}
	if err := run([]string{"import", "--source", source, "--db", db}); err == nil {
		t.Fatal("overwrote existing database")
	}
	out := filepath.Join(t.TempDir(), "rollback")
	if err := run([]string{"export", "--source", source, "--db", db, "--out", out}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(out, "users.json"))
	if err != nil || string(got) != string(data) {
		t.Fatal("export mismatch", err)
	}
	if err := run([]string{"check", "--source", out}); err != nil {
		t.Fatal(err)
	}
	unlock, err := persistence.LockDirectory(source)
	if err != nil {
		t.Fatal(err)
	}
	if err = run([]string{"check", "--source", source}); err == nil {
		t.Fatal("check accepted running server")
	}
	_ = unlock()
	if err = os.WriteFile(filepath.Join(source, "objects.json"), []byte(`broken`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = run([]string{"import", "--source", source, "--db", db + ".new"}); err == nil {
		t.Fatal("corrupt source accepted")
	}
	if _, err = os.Stat(db + ".new"); !os.IsNotExist(err) {
		t.Fatal("invalid import created database")
	}
}
