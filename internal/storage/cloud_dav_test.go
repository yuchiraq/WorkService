package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"project/internal/models"
)

func TestCloudDAVUploadPlaceholders(t *testing.T) {
	for _, scenario := range []string{"complete", "aborted", "persistence failure", "conflict", "expired", "starred", "shared", "website empty", "copied empty", "real replacement", "repeated empty"} {
		t.Run(scenario, func(t *testing.T) {
			dir := cloudFixture(t)
			ctx := context.Background()
			write := func(fs *CloudDAV, body string) {
				t.Helper()
				f, err := fs.OpenFile(ctx, "/upload.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = io.WriteString(f, body); err != nil {
					t.Fatal(err)
				}
				if err = f.Close(); err != nil {
					t.Fatal(err)
				}
			}
			var original models.CloudItem
			if scenario == "website empty" {
				original = cloudUpload(t, "a", "", "upload.txt", "")
			} else {
				fs := NewCloudDAV("a", true)
				if scenario != "copied empty" {
					fs.TrackUploadPlaceholders()
				}
				body := ""
				if scenario == "real replacement" {
					body = "original"
				}
				write(fs, body)
				if err := fs.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				fs.Close()
				original = CloudState("a").Items[0]
			}
			// Exercise persistence across requests/restarts, not just memory state.
			if err := LoadCloud(); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "expired":
				cloudItems[0].CreatedAt = time.Now().Add(-11 * time.Minute).UTC().Format(time.RFC3339Nano)
			case "starred":
				if err := ChangeCloud("a", CloudMutation{Action: "star", Items: []CloudRef{{original.ID, original.Version}}, Starred: true}); err != nil {
					t.Fatal(err)
				}
			case "shared":
				cloudItems[0].ShareToken = strings.Repeat("a", 43)
			}
			if scenario == "repeated empty" {
				fs := NewCloudDAV("a", true)
				fs.TrackUploadPlaceholders()
				write(fs, "")
				if err := fs.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				fs.Close()
				if len(CloudState("a").Items) != 1 || CloudState("a").Items[0].CreatedAt != original.CreatedAt {
					t.Fatal("empty retry created trash or extended reservation")
				}
				original = CloudState("a").Items[0]
			}
			fs := NewCloudDAV("a", true)
			defer fs.Close()
			fs.TrackUploadPlaceholders()
			write(fs, "content")
			expectFailure := scenario == "aborted" || scenario == "persistence failure" || scenario == "conflict"
			if scenario == "conflict" {
				if err := ChangeCloud("a", CloudMutation{Action: "star", Items: []CloudRef{{original.ID, original.Version}}, Starred: true}); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "persistence failure" {
				oldBackend, oldFile := backend, cloudFile
				backend, cloudFile = nil, dir
				t.Cleanup(func() { backend, cloudFile = oldBackend, oldFile })
			}
			if scenario != "aborted" {
				err := fs.Commit(ctx)
				if (err != nil) != expectFailure {
					t.Fatalf("commit: %v", err)
				}
			}
			items := CloudState("a").Items
			p, _ := CloudBlobPath(CloudDirectory(), original.ID)
			_, blobErr := os.Stat(p)
			consumed := scenario == "complete" || scenario == "repeated empty"
			if consumed {
				if len(items) != 1 || items[0].DeletedAt != "" || items[0].DAVPlaceholder || !os.IsNotExist(blobErr) {
					t.Fatal("placeholder leaked")
				}
			} else if expectFailure {
				if len(items) != 1 || items[0].ID != original.ID || blobErr != nil {
					t.Fatal("failed upload lost original")
				}
			} else {
				if len(items) != 2 || cloudCurrent(t, "a", original.ID).DeletedAt == "" || blobErr != nil {
					t.Fatal("real original not recoverable")
				}
			}
		})
	}
}

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
