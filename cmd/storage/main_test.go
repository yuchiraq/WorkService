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
	for _, command := range []string{"check", "import", "verify", "audit"} {
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
	if err = run([]string{"audit", "--source", source, "--db", db}); err != nil {
		t.Fatal("audit incorrectly reads obsolete JSON", err)
	}
	if err = run([]string{"import", "--source", source, "--db", db + ".new"}); err == nil {
		t.Fatal("corrupt source accepted")
	}
	if _, err = os.Stat(db + ".new"); !os.IsNotExist(err) {
		t.Fatal("invalid import created database")
	}
}

func TestAuditInventoryAndLock(t *testing.T) {
	source := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := persistence.OpenSQLite(dbPath, true)
	if err != nil {
		t.Fatal(err)
	}
	docs := map[string][]byte{
		"users.json":     []byte(`[{"id":"u","username":"admin","password":"preserved","status":"admin"}]`),
		"objects.json":   []byte(`[{"id":"o","name":"Test"}]`),
		"inventory.json": []byte(`{"catalog":[{"name":"Нерж 160","unit":"pcs","material":"Нержавейка"}],"items":[{"id":"i","object_id":"o","name":"Нерж 160","material":"Нержавейка","unit":"pcs","quantity":1500,"version":1}],"history":[]}`),
	}
	if err = db.Import(docs); err != nil {
		t.Fatal(err)
	}
	if err = run([]string{"audit", "--source", source, "--db", dbPath}); err == nil {
		t.Fatal("audit accepted an in-use database")
	}
	_ = db.Close()
	if err = run([]string{"audit", "--source", source, "--db", dbPath}); err != nil {
		t.Fatal(err)
	}
	db, err = persistence.OpenSQLite(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Write("inventory.json", []byte(`{"items":[{"id":"bad","object_id":"o","name":"Труба","unit":"pcs","quantity":-1,"version":1}]}`)); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if err = run([]string{"audit", "--source", source, "--db", dbPath}); err == nil {
		t.Fatal("audit accepted invalid inventory")
	}
}
