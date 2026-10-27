package router

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"project/internal/storage"
)

func testCloudSharing(t *testing.T, r *gin.Engine, admin, other, worker *http.Cookie) {
	request := func(method, path, body, csrf string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	tokenFor := func(cookie *http.Cookie) string {
		w := request("GET", "/cloud", "", "", cookie)
		return regexp.MustCompile(`data-csrf="([^"]+)"`).FindStringSubmatch(w.Body.String())[1]
	}
	csrf, otherCSRF := tokenFor(admin), tokenFor(other)
	item, err := storage.AddCloudFile("admin", "", "Shared <note>.txt", uuid.NewString(), strings.NewReader("shared content"))
	// Names deliberately reject markup punctuation; use a valid name for the actual fixture.
	if err == nil {
		t.Fatal("invalid cloud name accepted")
	}
	item, err = storage.AddCloudFile("admin", "", "Shared note.txt", uuid.NewString(), strings.NewReader("shared content"))
	if err != nil {
		t.Fatal(err)
	}
	path := "/cloud/share/" + item.ID
	body := `{"version":1,"enabled":true,"days":7}`
	for _, test := range []struct {
		cookie *http.Cookie
		csrf   string
		status int
	}{{admin, "", 403}, {other, otherCSRF, 404}, {worker, "", 403}, {nil, "", 302}} {
		if w := request("POST", path, body, test.csrf, test.cookie); w.Code != test.status {
			t.Fatal("share permissions", w.Code, w.Body.String())
		}
	}
	w := request("POST", path, body, csrf, admin)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var state storage.CloudSnapshot
	_ = json.Unmarshal(w.Body.Bytes(), &state)
	for _, entry := range state.Items {
		if entry.ID == item.ID {
			item = entry
		}
	}
	url := "/s/" + item.ShareToken
	w = request("GET", url, "", "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), item.Name) || strings.Contains(w.Body.String(), item.SHA256) || strings.Contains(w.Body.String(), "shareToken") {
		t.Fatal("public page", w.Code, w.Body.String())
	}
	if w.Header().Get("Referrer-Policy") != "no-referrer" || w.Header().Get("X-Robots-Tag") == "" {
		t.Fatal("missing security headers")
	}
	w = request("GET", url+"/file/"+item.ID, "", "", nil)
	if w.Code != 200 || w.Body.String() != "shared content" || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatal("public file", w.Code, w.Body.String())
	}
	if w = request("GET", "/s/invalid", "", "", nil); w.Code != 404 {
		t.Fatal("invalid link", w.Code)
	}
	if w = request("POST", url+"/archive?item=outside", "", "", nil); w.Code != 404 {
		t.Fatal("archive scope escape", w.Code)
	}
	w = request("POST", url+"/archive", "", "", nil)
	var archive struct {
		URL string `json:"url"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &archive); err != nil || w.Code != 200 {
		t.Fatal("shared ZIP creation", w.Code, w.Body.String())
	}
	w = request("GET", archive.URL, "", "", nil)
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/zip" {
		t.Fatal("shared ZIP", w.Code, w.Body.String())
	}
	z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil || len(z.File) != 1 || z.File[0].Name != item.Name {
		t.Fatal("ZIP content", err)
	}
	entry, _ := z.File[0].Open()
	data, _ := io.ReadAll(entry)
	entry.Close()
	if string(data) != "shared content" {
		t.Fatal("ZIP bytes changed")
	}
	w = request("POST", url+"/archive", "", "", nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &archive)
	if w = request("POST", path, `{"version":2,"enabled":false}`, csrf, admin); w.Code != 200 {
		t.Fatal("revoke", w.Code)
	}
	if w = request("GET", url+"/file/"+item.ID, "", "", nil); w.Code != 404 {
		t.Fatal("revoked download", w.Code)
	}
	if w = request("GET", archive.URL, "", "", nil); w.Code != 404 {
		t.Fatal("prepared ZIP survived revocation", w.Code)
	}
	if w = request("POST", url+"/archive", "", "", nil); w.Code != 404 {
		t.Fatal("revoked ZIP creation", w.Code)
	}
}
