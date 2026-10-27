package router

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"project/internal/models"
	"project/internal/storage"
)

func testPersonalCloud(t *testing.T, r *gin.Engine, admin, other, worker *http.Cookie) {
	request := func(method, path, body, token string, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", token)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	tokenFor := func(cookie *http.Cookie) string {
		t.Helper()
		w := request("GET", "/cloud", "", "", cookie)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		m := regexp.MustCompile(`data-csrf="([^"]+)"`).FindStringSubmatch(w.Body.String())
		if len(m) < 2 {
			t.Fatal("cloud CSRF missing")
		}
		return m[1]
	}
	token, otherToken := tokenFor(admin), tokenFor(other)
	for _, path := range []string{"/cloud", "/cloud/items", "/cloud/file/id", "/cloud/archive/id"} {
		if w := request("GET", path, "", "", worker); w.Code != 403 {
			t.Fatal("worker access", path, w.Code)
		}
		if w := request("GET", path, "", "", nil); w.Code == 200 {
			t.Fatal("anonymous access", path)
		}
	}
	if w := request("POST", "/cloud/action", `{"action":"folder","name":"No CSRF"}`, "", admin); w.Code != 403 {
		t.Fatal("missing CSRF", w.Code)
	}
	w := request("POST", "/cloud/action", `{"action":"folder","name":"Documents"}`, token, admin)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var state storage.CloudSnapshot
	_ = json.Unmarshal(w.Body.Bytes(), &state)
	folder := state.Items[0]
	upload := func(name, content, id, csrf string) *httptest.ResponseRecorder {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, e := writer.CreateFormFile("file", name)
		if e != nil {
			t.Fatal(e)
		}
		_, _ = part.Write([]byte(content))
		_ = writer.Close()
		req := httptest.NewRequest("POST", "/cloud/upload?parent="+folder.ID, &body)
		req.AddCookie(admin)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-CSRF-Token", csrf)
		req.Header.Set("X-Upload-ID", id)
		response := httptest.NewRecorder()
		r.ServeHTTP(response, req)
		return response
	}
	if w = upload("no.txt", "no", uuid.NewString(), ""); w.Code != 403 {
		t.Fatal("upload CSRF", w.Code)
	}
	id := uuid.NewString()
	w = upload("original.txt", "hello <safe> & original bytes", id, token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = upload("original.txt", "retry must not replace", id, token); w.Code != 200 {
		t.Fatal("retry", w.Code)
	}
	for _, suffix := range []string{"", "?preview=text", "?preview=1", "?preview=video", "?preview=stream"} {
		if w = request("GET", "/cloud/file/"+id+suffix, "", "", other); w.Code != 404 {
			t.Fatal("other admin read", w.Code)
		}
	}
	if w = request("GET", "/cloud/items", "", "", other); strings.Contains(w.Body.String(), id) || strings.Contains(w.Body.String(), folder.ID) {
		t.Fatal("metadata leak")
	}
	for _, action := range []string{"rename", "move", "star", "trash", "restore", "purge"} {
		body, _ := json.Marshal(storage.CloudMutation{Action: action, Name: "stolen", Items: []storage.CloudRef{{ID: id, Version: 1}}})
		if w = request("POST", "/cloud/action", string(body), otherToken, other); w.Code != 404 {
			t.Fatal("other admin action", action, w.Code)
		}
	}
	w = request("GET", "/cloud/file/"+id, "", "", admin)
	if w.Code != 200 || w.Body.String() != "hello <safe> & original bytes" || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment;") || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("download", w.Code, w.Header(), w.Body.String())
	}
	req := httptest.NewRequest("GET", "/cloud/file/"+id, nil)
	req.AddCookie(admin)
	req.Header.Set("Range", "bytes=0-4")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 206 || w.Body.String() != "hello" {
		t.Fatal("range", w.Code, w.Body.String())
	}
	w = request("GET", "/cloud/file/"+id+"?preview=text", "", "", admin)
	if w.Code != 200 || strings.Contains(w.Body.String(), "<safe>") {
		t.Fatal("unsafe text JSON", w.Code, w.Body.String())
	}
	htmlID := uuid.NewString()
	w = upload("unsafe.html", "<!doctype html><script>window.top.hacked=true</script>", htmlID, token)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = request("GET", "/cloud/file/"+htmlID+"?preview=1", "", "", admin); w.Code != 415 {
		t.Fatal("HTML inline", w.Code)
	}
	if w = request("GET", "/cloud/file/"+htmlID, "", "", admin); !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") || w.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatal("unsafe download headers")
	}
	w = request("POST", "/cloud/archive", `{"ids":["`+folder.ID+`"]}`, otherToken, other)
	if w.Code != 404 {
		t.Fatal("archive owner leak", w.Code)
	}
	w = request("POST", "/cloud/archive", `{"ids":["`+folder.ID+`"]}`, token, admin)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var archive struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &archive)
	if w = request("GET", archive.URL, "", "", other); w.Code != 404 {
		t.Fatal("archive token leak", w.Code)
	}
	w = request("GET", archive.URL, "", "", admin)
	z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil || w.Code != 200 {
		t.Fatal("archive", w.Code, err)
	}
	found := false
	for _, f := range z.File {
		if f.Name == "Documents/original.txt" {
			file, _ := f.Open()
			data, _ := io.ReadAll(file)
			_ = file.Close()
			found = string(data) == "hello <safe> & original bytes"
		}
	}
	if !found {
		t.Fatal("archive bytes lost")
	}
	if w = request("GET", archive.URL, "", "", admin); w.Code != 404 {
		t.Fatal("archive token reusable")
	}
	w = request("POST", "/cloud/action", `{"action":"trash","items":[{"id":"`+folder.ID+`","version":1}]}`, token, admin)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = request("GET", "/cloud/file/"+id, "", "", admin); w.Code != 404 {
		t.Fatal("trashed file served")
	}
	if w = request("POST", "/cloud/action", `{"action":"restore","items":[{"id":"`+folder.ID+`","version":1}]}`, token, admin); w.Code != 409 {
		t.Fatal("stale restore", w.Code)
	}
	if w = request("POST", "/cloud/action", `{"action":"purge","items":[{"id":"`+folder.ID+`","version":2}]}`, token, admin); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if len(storage.CloudState("admin").Items) != 0 {
		t.Fatal("cleanup failed")
	}
}

func testAssignmentPrivacy(t *testing.T, r *gin.Engine, admin, user *http.Cookie) {
	get := func(path string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	object, err := storage.GetObjectByID("object")
	if err != nil {
		t.Fatal(err)
	}
	original := object
	object.ResponsibleUserID = "user"
	if err = storage.UpdateObject(object); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.UpdateObject(original) })
	shared, err := storage.CreateTimesheet(models.TimesheetEntry{Date: "2026-09-23", StartTime: "08:00", EndTime: "16:00", WorkerIDs: []string{"own", "other"}, ObjectIDs: []string{"object"}, CreatedByID: "admin", Notes: "VISIBLE_SHARED_ASSIGNMENT"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.DeleteTimesheet(shared.ID) })
	created, err := storage.CreateTimesheet(models.TimesheetEntry{Date: "2026-09-24", StartTime: "10:00", EndTime: "12:00", WorkerIDs: []string{"other"}, ObjectIDs: []string{"object"}, CreatedByID: "user", Notes: "PRIVATE_CREATED_WITHOUT_MEMBERSHIP"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.DeleteTimesheet(created.ID) })
	for _, path := range []string{"/object/object", "/schedule?month=2026-09", "/timesheets?month=2026-09&worker=other", "/worker/own?month=2026-09"} {
		w := get(path, user)
		if w.Code != 200 {
			t.Fatal(path, w.Code)
		}
		for _, private := range []string{"PRIVATE_OTHER_WORK", "PRIVATE_CREATED_WITHOUT_MEMBERSHIP"} {
			if strings.Contains(w.Body.String(), private) {
				t.Fatal("foreign assignment exposed", path, private)
			}
		}
		if !strings.Contains(w.Body.String(), "VISIBLE_SHARED_ASSIGNMENT") {
			t.Fatal("own shared assignment hidden", path)
		}
	}
	for _, path := range []string{"/schedule/edit/private-entry", "/timesheets/edit/private-entry", "/schedule/edit/" + created.ID, "/worker/other"} {
		if w := get(path, user); w.Code != 403 {
			t.Fatal("foreign direct access", path, w.Code)
		}
	}
	w := get("/object/object", user)
	if strings.Contains(w.Body.String(), `href="/worker/other"`) || !strings.Contains(w.Body.String(), "Мои назначения") {
		t.Fatal("object exposes colleague profile links")
	}
	w = get("/object/object", admin)
	if !strings.Contains(w.Body.String(), "PRIVATE_OTHER_WORK") || !strings.Contains(w.Body.String(), "PRIVATE_CREATED_WITHOUT_MEMBERSHIP") {
		t.Fatal("admin scope changed")
	}
	// Being responsible for a site is not permission to see its workers' shifts.
	private, err := storage.CreateObject(models.Object{Name: "Responsible only", Address: "Minsk", ResponsibleUserID: "user"})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := storage.CreateTimesheet(models.TimesheetEntry{Date: "2026-09-22", StartTime: "08:00", EndTime: "17:00", WorkerIDs: []string{"other"}, ObjectIDs: []string{private.ID}, Notes: "RESPONSIBLE_MUST_NOT_SEE"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.DeleteTimesheet(entry.ID); _ = storage.DeleteObject(private.ID) })
	w = get("/object/"+private.ID, user)
	if w.Code != 200 || strings.Contains(w.Body.String(), "RESPONSIBLE_MUST_NOT_SEE") {
		t.Fatal("responsible scope", w.Code)
	}
}
