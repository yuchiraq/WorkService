package router

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"project/internal/storage"
)

func testQuickVehicleRecords(t *testing.T, r *gin.Engine, id string, admin, worker, other *http.Cookie) {
	request := func(method, path string, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	path := "/vehicles/" + id
	if w := request("GET", path+"/records/new?modal=1", nil, other); w.Code != 404 {
		t.Fatal("foreign vehicle form", w.Code)
	}
	for _, cookie := range []*http.Cookie{admin, worker} {
		w := request("GET", path+"/records/new?modal=1&return=%2Fobjects", nil, cookie)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "data-vehicle-quick-form") || strings.Contains(w.Body.String(), "side-nav") {
			t.Fatal("quick form", w.Code, w.Body.String())
		}
		csrf := regexp.MustCompile(`name="_csrf_token" value="([^"]+)"`).FindStringSubmatch(w.Body.String())[1]
		form := url.Values{"_csrf_token": {csrf}, "quick": {"1"}, "type": {"fuel"}, "date": {"2026-10-25"}, "liters": {"0"}, "return_to": {"/objects"}}
		if w = request("POST", path+"/records", form, cookie); w.Code != 422 {
			t.Fatal("quick validation", w.Code, w.Body.String())
		}
		form.Set("liters", "12.5")
		form.Set("amount", "NaN")
		if w = request("POST", path+"/records", form, cookie); w.Code != 422 {
			t.Fatal("invalid numeric amount", w.Code)
		}
		form.Set("amount", "32.5")
		if w = request("POST", path+"/records", form, cookie); w.Code != 302 || !strings.HasPrefix(w.Header().Get("Location"), "/objects?success=") {
			t.Fatal("quick save", w.Code, w.Header())
		}
	}
	records, _ := storage.GetVehicleRecords(id)
	if len(records) < 3 {
		t.Fatal("records not saved")
	}
}
