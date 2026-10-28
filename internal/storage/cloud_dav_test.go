package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestCloudDAVTransactions(t *testing.T) {
	dir := cloudFixture(t)
	ctx := context.Background()
	old := cloudUpload(t, "a", "", "original.txt", "original")
	write := func(fs *CloudDAV, name, body string) {
		t.Helper()
		f, err := fs.OpenFile(ctx, name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = io.Copy(f, strings.NewReader(body)); err != nil {
			_ = f.Close()
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("abort keeps original", func(t *testing.T) {
		fs := NewCloudDAV("a", true)
		write(fs, "/original.txt", "never published")
		fs.Close()
		_, f, err := OpenCloudFile("a", old.ID)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		b, _ := io.ReadAll(f)
		if string(b) != "original" || len(CloudState("a").Items) != 1 {
			t.Fatal("aborted write leaked")
		}
	})
	t.Run("website race", func(t *testing.T) {
		fs := NewCloudDAV("a", true)
		defer fs.Close()
		write(fs, "/new.txt", "new")
		if err := ChangeCloud("a", CloudMutation{Action: "star", Items: []CloudRef{{old.ID, old.Version}}, Starred: true}); err != nil {
			t.Fatal(err)
		}
		if err := fs.Commit(ctx); !errors.Is(err, ErrCloudConflict) {
			t.Fatal("concurrent edit overwritten", err)
		}
		if len(CloudState("a").Items) != 1 {
			t.Fatal("partial commit")
		}
	})
	t.Run("failed persistence", func(t *testing.T) {
		fs := NewCloudDAV("a", true)
		defer fs.Close()
		write(fs, "/new.txt", "new")
		originalBackend := backend
		backend = nil
		oldFile := cloudFile
		cloudFile = dir
		t.Cleanup(func() { backend = originalBackend; cloudFile = oldFile })
		before := CloudState("a")
		if err := fs.Commit(ctx); err == nil {
			t.Fatal("write to directory succeeded")
		}
		if !reflect.DeepEqual(before, CloudState("a")) {
			t.Fatal("failed commit changed memory")
		}
	})
	t.Run("commit and reload", func(t *testing.T) {
		fs := NewCloudDAV("a", true)
		defer fs.Close()
		write(fs, "/original.txt", "replacement")
		cloudUpload(t, "b", "", "other.txt", "unrelated")
		if err := fs.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err := LoadCloud(); err != nil {
			t.Fatal(err)
		}
		if len(CloudState("a").Items) != 2 || len(CloudState("b").Items) != 1 {
			t.Fatal("isolation or persistence")
		}
		if cloudCurrent(t, "a", old.ID).DeletedAt == "" {
			t.Fatal("old original lost")
		}
	})
	t.Run("path isolation", func(t *testing.T) {
		fs := NewCloudDAV("a", false)
		defer fs.Close()
		for _, name := range []string{"/../original.txt", "/a/../../original.txt", "/other.txt", "/a\\b", "/./original.txt"} {
			if _, err := fs.Stat(ctx, name); err == nil {
				t.Fatal("invalid path accepted", name)
			}
		}
	})
	t.Run("size quota and cancellation", func(t *testing.T) {
		for _, body := range []string{strings.Repeat("x", 81), strings.Repeat("x", 79)} {
			fs := NewCloudDAV("a", true)
			// The fixture's file limit is 80 bytes; account quota includes the trash.
			fs.limit = 80
			fs.quota = 50
			f, err := fs.OpenFile(ctx, "/large.bin", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.Write([]byte(body))
			if err == nil {
				t.Fatal("limit ignored")
			}
			_ = f.Close()
			fs.Close()
		}
		fs := NewCloudDAV("a", true)
		defer fs.Close()
		write(fs, "/cancel.txt", "canceled")
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if err := fs.Commit(canceled); !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation ignored", err)
		}
	})
}
