package storage

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCloudShareScopeAndLifecycle(t *testing.T) {
	cloudFixture(t)
	root := cloudFolder(t, "alice", "", "Shared")
	nested := cloudFolder(t, "alice", root.ID, "Nested")
	file := cloudUpload(t, "alice", nested.ID, "photo.txt", "public bytes")
	private := cloudUpload(t, "alice", "", "private.txt", "private bytes")
	foreign := cloudUpload(t, "bob", "", "foreign.txt", "foreign bytes")
	if err := SetCloudShare("bob", root.ID, root.Version, true, 0); !errors.Is(err, ErrCloudNotFound) {
		t.Fatal(err)
	}
	if err := SetCloudShare("alice", root.ID, 999, true, 0); !errors.Is(err, ErrCloudConflict) {
		t.Fatal(err)
	}
	if err := SetCloudShare("alice", root.ID, root.Version, true, 7); err != nil {
		t.Fatal(err)
	}
	root = cloudCurrent(t, "alice", root.ID)
	if len(root.ShareToken) != 43 || root.ShareExpiresAt == "" {
		t.Fatal("invalid link")
	}
	if err := LoadCloud(); err != nil {
		t.Fatal(err)
	}
	if _, err := GetSharedCloud(root.ShareToken, file.ID); err != nil {
		t.Fatal("nested file", err)
	}
	for _, id := range []string{private.ID, foreign.ID, "../private"} {
		if _, err := GetSharedCloud(root.ShareToken, id); !errors.Is(err, ErrCloudNotFound) {
			t.Fatal("scope escape", id, err)
		}
	}
	view, err := GetSharedCloud(root.ShareToken, nested.ID)
	if err != nil || len(view.Items) != 1 || len(view.Breadcrumbs) != 2 {
		t.Fatal("nested view", view, err)
	}
	var archive bytes.Buffer
	if err = WriteSharedCloudArchive(root.ShareToken, "", &archive); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range z.File {
		if strings.Contains(entry.Name, "private") || strings.Contains(entry.Name, "foreign") {
			t.Fatal("ZIP scope leak", entry.Name)
		}
		if entry.Name == "Shared/Nested/photo.txt" {
			found = true
		}
	}
	if !found {
		t.Fatal("recursive shared ZIP missing file")
	}
	for _, id := range []string{private.ID, foreign.ID, "../private"} {
		if err = WriteSharedCloudArchive(root.ShareToken, id, &bytes.Buffer{}); !errors.Is(err, ErrCloudNotFound) {
			t.Fatal("ZIP scope escape", id, err)
		}
	}
	if err := ChangeCloud("alice", CloudMutation{Action: "move", Items: []CloudRef{{file.ID, file.Version}}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := OpenSharedCloudFile(root.ShareToken, file.ID); !errors.Is(err, ErrCloudNotFound) {
		t.Fatal("moved file still shared", err)
	}
	old := root.ShareToken
	if err := SetCloudShare("alice", root.ID, root.Version, false, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := GetSharedCloud(old, ""); !errors.Is(err, ErrCloudNotFound) {
		t.Fatal("revoked link", err)
	}
	root = cloudCurrent(t, "alice", root.ID)
	if err := SetCloudShare("alice", root.ID, root.Version, true, 0); err != nil {
		t.Fatal(err)
	}
	root = cloudCurrent(t, "alice", root.ID)
	if old == root.ShareToken {
		t.Fatal("revoked token reused")
	}
	cloudChange(t, "alice", "trash", root)
	root = cloudCurrent(t, "alice", root.ID)
	if root.ShareToken != "" {
		t.Fatal("trash retained link")
	}
	cloudChange(t, "alice", "restore", root)
	if _, err := GetSharedCloud(old, ""); !errors.Is(err, ErrCloudNotFound) {
		t.Fatal("restore revived link")
	}
	file = cloudCurrent(t, "alice", file.ID)
	if err := SetCloudShare("alice", file.ID, file.Version, true, 0); err != nil {
		t.Fatal(err)
	}
	file = cloudCurrent(t, "alice", file.ID)
	for i := range cloudItems {
		if cloudItems[i].ID == file.ID {
			cloudItems[i].ShareExpiresAt = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
		}
	}
	if _, err := GetSharedCloud(file.ShareToken, ""); !errors.Is(err, ErrCloudNotFound) {
		t.Fatal("expired link", err)
	}
	if _, err := GetSharedCloud(strings.Repeat("x", 43), ""); !errors.Is(err, ErrCloudNotFound) {
		t.Fatal("invalid token", err)
	}
}
