package router

import (
	"encoding/json"
	"errors"
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

type brokenDAVBody struct{ sent bool }

func (b *brokenDAVBody) Read(p []byte) (int, error) {
	if !b.sent {
		b.sent = true
		return copy(p, "partial"), nil
	}
	return 0, io.ErrUnexpectedEOF
}

func testCloudDAV(t *testing.T, r *gin.Engine, admin, other, worker *http.Cookie) {
	t.Helper()
	request := func(method, target string, body io.Reader, username, password string, headers map[string]string, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "https://appworks.example"+target, body)
		if username != "" {
			req.SetBasicAuth(username, password)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	csrf := func(cookie *http.Cookie) string {
		w := request("GET", "/cloud/connections", nil, "", "", nil, cookie)
		matches := regexp.MustCompile(`data-csrf="([^"]+)"`).FindStringSubmatch(w.Body.String())
		if len(matches) != 2 {
			t.Fatal("connections page", w.Code)
		}
		return matches[1]
	}
	adminCSRF, otherCSRF := csrf(admin), csrf(other)
	createHeaders := map[string]string{"X-CSRF-Token": adminCSRF, "Content-Type": "application/json"}
	for _, test := range []struct {
		cookie *http.Cookie
		token  string
		code   int
	}{{nil, "", 302}, {worker, "", 403}, {admin, "", 403}} {
		w := request("POST", "/cloud/devices", strings.NewReader(`{}`), "", "", map[string]string{"X-CSRF-Token": test.token}, test.cookie)
		if w.Code != test.code {
			t.Fatalf("device management ACL: %d want %d", w.Code, test.code)
		}
	}
	w := request("POST", "/cloud/devices", strings.NewReader(`{"name":"Windows","days":90,"password":"wrong"}`), "", "", createHeaders, admin)
	if w.Code != 403 {
		t.Fatal("current password confirmation", w.Code)
	}
	w = request("POST", "/cloud/devices", strings.NewReader(`{"name":"Windows","days":90,"readOnly":false,"password":"testpass"}`), "", "", createHeaders, admin)
	if w.Code != 201 {
		t.Fatal("create device", w.Code, w.Body.String())
	}
	var created struct {
		Device   storage.CloudDevice `json:"device"`
		Password string              `json:"password"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	key := created.Password
	if len(key) != 83 {
		t.Fatal("wrong secret format")
	}
	if err := storage.LoadCloudDevices(); err != nil {
		t.Fatal(err)
	}
	for _, cookie := range []*http.Cookie{admin, other} {
		w = request("GET", "/cloud/devices", nil, "", "", nil, cookie)
		if w.Code != 200 || strings.Contains(w.Body.String(), key) || strings.Contains(w.Body.String(), "secretHash") || (cookie == other && strings.Contains(w.Body.String(), created.Device.ID)) {
			t.Fatal("device listing isolation", w.Code)
		}
	}
	_, readKey, err := storage.CreateCloudDevice("admin", "read only", true, 30)
	if err != nil {
		t.Fatal(err)
	}
	_, otherKey, err := storage.CreateCloudDevice("admin2", "other", false, 30)
	if err != nil {
		t.Fatal(err)
	}
	for _, password := range []string{"testpass", key + "wrong", ""} {
		w = request("PROPFIND", "/dav/", nil, "admin", password, map[string]string{"Depth": "1"}, admin)
		if w.Code != 401 {
			t.Fatal("must ignore browser session and account password", w.Code)
		}
	}
	for _, target := range []string{"http://appworks.example/dav/", "https://evil.example/dav/"} {
		req := httptest.NewRequest("PROPFIND", target, nil)
		req.SetBasicAuth("admin", key)
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("Depth", "1")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatal("insecure host/proxy accepted", target, w.Code)
		}
	}
	// A known TLS-terminating proxy may forward HTTPS; arbitrary peers may not.
	req := httptest.NewRequest("PROPFIND", "http://appworks.example/dav/", nil)
	req.RemoteAddr = "127.0.0.1:50000"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("Depth", "0")
	req.SetBasicAuth("admin", key)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 207 {
		t.Fatal("trusted proxy", w.Code, w.Body.String())
	}
	if w = request("PROPFIND", "/dav/", nil, "admin", key, nil, nil); w.Code != 403 {
		t.Fatal("unbounded listing", w.Code)
	}
	if w = request("GET", "/dav/", nil, "admin", key, map[string]string{"Origin": "https://evil.example"}, nil); w.Code != 403 {
		t.Fatal("cross-origin access", w.Code)
	}
	for method, content := range map[string]string{"MKCOL": "", "PUT": "blocked", "DELETE": "", "LOCK": ""} {
		if w = request(method, "/dav/readonly", strings.NewReader(content), "admin", readKey, nil, nil); w.Code != 403 {
			t.Fatal("read-only", method, w.Code)
		}
	}
	w = request("MKCOL", "/dav/DAV-test", nil, "admin", key, nil, nil)
	if w.Code != 201 {
		t.Fatal("mkdir", w.Code, w.Body.String())
	}
	filePath := "/dav/DAV-test/report.txt"
	w = request("PUT", filePath, strings.NewReader("original"), "admin", key, nil, nil)
	if w.Code != 201 {
		t.Fatal("put", w.Code, w.Body.String())
	}
	etag := w.Header().Get("ETag")
	w = request("GET", filePath, nil, "admin", readKey, nil, nil)
	if w.Code != 200 || w.Body.String() != "original" || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("read", w.Code, w.Body.String())
	}
	w = request("GET", filePath, nil, "admin", key, map[string]string{"Range": "bytes=0-3"}, nil)
	if w.Code != 206 || w.Body.String() != "orig" {
		t.Fatal("range", w.Code)
	}
	if w = request("GET", filePath, nil, "admin2", otherKey, nil, nil); w.Code != 404 {
		t.Fatal("cross owner", w.Code)
	}
	w = request("PROPFIND", "/dav/DAV-test/", nil, "admin", key, map[string]string{"Depth": "1"}, nil)
	if w.Code != 207 || !strings.Contains(w.Body.String(), "report.txt") || !strings.Contains(w.Body.String(), etag) {
		t.Fatal("listing", w.Code, w.Body.String())
	}
	w = request("PUT", filePath, strings.NewReader("changed"), "admin", key, map[string]string{"If-Match": `"wrong"`}, nil)
	if w.Code != 412 {
		t.Fatal("if-match", w.Code)
	}
	w = request("PUT", filePath, &brokenDAVBody{}, "admin", key, nil, nil)
	if w.Code < 400 {
		t.Fatal("broken upload accepted")
	}
	w = request("GET", filePath, nil, "admin", key, nil, nil)
	if w.Body.String() != "original" {
		t.Fatal("partial file published", w.Body.String())
	}
	w = request("PUT", filePath, strings.NewReader("replacement"), "admin", key, map[string]string{"If-Match": etag}, nil)
	if w.Code != 201 {
		t.Fatal("overwrite", w.Code, w.Body.String())
	}
	trash := false
	for _, item := range storage.CloudState("admin").Items {
		if item.Name == "report.txt" && item.DeletedAt != "" {
			trash = true
		}
	}
	if !trash {
		t.Fatal("original not recoverable")
	}
	w = request("COPY", "/dav/DAV-test", nil, "admin", key, map[string]string{"Destination": "https://appworks.example/dav/DAV-test/nested"}, nil)
	if w.Code != 403 {
		t.Fatal("recursive copy into itself", w.Code)
	}
	w = request("COPY", filePath, nil, "admin", key, map[string]string{"Destination": "https://evil.example/dav/stolen"}, nil)
	if w.Code != 400 {
		t.Fatal("external destination", w.Code)
	}
	w = request("COPY", filePath, nil, "admin", key, map[string]string{"Destination": "https://appworks.example/dav/DAV-test/copy.txt"}, nil)
	if w.Code != 201 {
		t.Fatal("copy", w.Code, w.Body.String())
	}
	w = request("MOVE", "/dav/DAV-test/copy.txt", nil, "admin", key, map[string]string{"Destination": "/dav/DAV-test/moved.txt"}, nil)
	if w.Code != 201 {
		t.Fatal("move", w.Code, w.Body.String())
	}
	lockXML := `<D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype><D:owner>test</D:owner></D:lockinfo>`
	for _, initialMethod := range []string{"PUT", "LOCK"} {
		target := "/dav/DAV-test/new-" + initialMethod + ".txt"
		initialBody := ""
		if initialMethod == "LOCK" {
			initialBody = lockXML
		}
		w = request(initialMethod, target, strings.NewReader(initialBody), "admin", key, nil, nil)
		if w.Code != 201 {
			t.Fatal("upload reservation", initialMethod, w.Code, w.Body.String())
		}
		token := w.Header().Get("Lock-Token")
		headers := map[string]string{}
		if token != "" {
			headers["If"] = "(" + token + ")"
		}
		before := storage.CloudState("admin")
		w = request("PUT", target, &brokenDAVBody{}, "admin", key, headers, nil)
		if w.Code < 400 || len(storage.CloudState("admin").Items) != len(before.Items) {
			t.Fatal("failed placeholder upload published")
		}
		w = request("PUT", target, strings.NewReader("uploaded"), "admin", key, headers, nil)
		if w.Code != 201 {
			t.Fatal("complete reserved upload", w.Code, w.Body.String())
		}
		if len(storage.CloudState("admin").Items) != len(before.Items) {
			t.Fatal("new upload added trash")
		}
		uploadETag := w.Header().Get("ETag")
		w = request("GET", target, nil, "admin", key, nil, nil)
		if w.Code != 200 || w.Body.String() != "uploaded" || w.Header().Get("ETag") != uploadETag {
			t.Fatal("uploaded file or etag mismatch")
		}
		if token != "" {
			w = request("UNLOCK", target, nil, "admin", key, map[string]string{"Lock-Token": token}, nil)
			if w.Code != 204 {
				t.Fatal("unlock new upload", w.Code)
			}
		}
	}
	w = request("LOCK", filePath, strings.NewReader(lockXML), "admin", key, map[string]string{"Content-Type": "application/xml", "Timeout": "Infinite"}, nil)
	if w.Code != 200 {
		t.Fatal("lock", w.Code, w.Body.String())
	}
	lock := w.Header().Get("Lock-Token")
	if w = request("PUT", "/dav/DAV-test/REPORT.TXT", strings.NewReader("bad"), "admin", key, nil, nil); w.Code != 423 {
		t.Fatal("case alias bypassed lock", w.Code)
	}
	w = request("UNLOCK", filePath, nil, "admin", key, map[string]string{"Lock-Token": lock}, nil)
	if w.Code != 204 {
		t.Fatal("unlock", w.Code)
	}
	w = request("DELETE", "/dav/DAV-test", nil, "admin", key, nil, nil)
	if w.Code != 204 {
		t.Fatal("delete", w.Code, w.Body.String())
	}
	if w = request("GET", filePath, nil, "admin", key, nil, nil); w.Code != 404 {
		t.Fatal("deleted remains accessible")
	}
	w = request("POST", "/cloud/devices/"+created.Device.ID+"/revoke", strings.NewReader(`{}`), "", "", map[string]string{"X-CSRF-Token": otherCSRF}, other)
	if w.Code != 404 {
		t.Fatal("cross-owner revoke", w.Code)
	}
	w = request("POST", "/cloud/devices/"+created.Device.ID+"/revoke", strings.NewReader(`{}`), "", "", createHeaders, admin)
	if w.Code != 204 {
		t.Fatal("revoke", w.Code)
	}
	if w = request("PROPFIND", "/dav/", nil, "admin", key, map[string]string{"Depth": "0"}, nil); w.Code != 401 {
		t.Fatal("revoked access", w.Code)
	}
	// Demotion and account-password changes immediately invalidate device access.
	u, _ := storage.GetUserByID("admin2")
	original := u
	u.Status = "user"
	if err = storage.UpdateUser(u); err != nil {
		t.Fatal(err)
	}
	if _, err = storage.AuthenticateCloudDevice("admin2", otherKey); !errors.Is(err, storage.ErrCloudDevice) {
		t.Fatal("demoted device authenticated")
	}
	if err = storage.UpdateUser(original); err != nil {
		t.Fatal(err)
	}
	u = original
	u.Password = "new-password"
	if err = storage.UpdateUser(u); err != nil {
		t.Fatal(err)
	}
	if _, err = storage.AuthenticateCloudDevice("admin2", otherKey); !errors.Is(err, storage.ErrCloudDevice) {
		t.Fatal("old credential survived password change")
	}
	if err = storage.UpdateUser(original); err != nil {
		t.Fatal(err)
	}
	// Existing regular cloud uploads remain visible through DAV, including Unicode.
	item, err := storage.AddCloudFile("admin", "", "Документ.txt", uuid.NewString(), strings.NewReader("site upload"))
	if err != nil {
		t.Fatal(err)
	}
	w = request("GET", "/dav/Документ.txt", nil, "admin", readKey, nil, nil)
	if w.Code != 200 || w.Body.String() != "site upload" {
		t.Fatal("website interoperability", w.Code)
	}
	_ = storage.ChangeCloud("admin", storage.CloudMutation{Action: "trash", Items: []storage.CloudRef{{ID: item.ID, Version: item.Version}}})
}
