// storage prepares and verifies a reversible migration; run with the server stopped.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"project/internal/persistence"
	"project/internal/storage"
)

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: storage check|import|verify|export [--source storage] [--db storage/workservice.db] [--photos directory] [--cloud directory] [--out new-directory]")
	}
	command := args[0]
	if command != "check" && command != "import" && command != "verify" && command != "export" {
		return errors.New("unknown command")
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	source := flags.String("source", "storage", "source JSON directory; server must be stopped")
	dbPath := flags.String("db", "storage/workservice.db", "SQLite file")
	photoDir := flags.String("photos", "", "photo directory (default: source/object_photos)")
	cloudDir := flags.String("cloud", "", "private cloud directory (default: source/cloud)")
	out := flags.String("out", "", "new export directory")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	unlock, err := persistence.LockDirectory(*source)
	if err != nil {
		return err
	}
	defer unlock()
	if *photoDir == "" {
		*photoDir = filepath.Join(*source, "object_photos")
	}
	if *cloudDir == "" {
		*cloudDir = filepath.Join(*source, "cloud")
	}
	if command == "export" {
		if *out == "" {
			return errors.New("export requires --out pointing to a new directory")
		}
		db, err := persistence.OpenSQLite(*dbPath, false)
		if err != nil {
			return err
		}
		defer db.Close()
		if err = db.IntegrityCheck(); err != nil {
			return err
		}
		docs, err := db.Snapshot()
		if err != nil {
			return err
		}
		if _, err = storage.ValidateDocuments(docs, *photoDir, *cloudDir); err != nil {
			return err
		}
		return storage.ExportDocuments(docs, *photoDir, *out, *cloudDir)
	}
	documents, report, err := storage.ReadMigrationSource(*source, *photoDir, *cloudDir)
	if err != nil {
		return err
	}
	encoded, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(encoded))
	if command == "check" {
		return nil
	}
	db, err := persistence.OpenSQLite(*dbPath, command == "import")
	if err != nil {
		return err
	}
	defer db.Close()
	if command == "import" {
		if err = db.Import(documents); err != nil {
			return err
		}
	}
	if err = db.IntegrityCheck(); err != nil {
		return err
	}
	actual, err := db.Snapshot()
	if err != nil {
		return err
	}
	return storage.CompareDocuments(documents, actual)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "storage:", err)
		os.Exit(1)
	}
	fmt.Println("OK")
}
