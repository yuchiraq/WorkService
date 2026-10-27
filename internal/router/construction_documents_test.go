package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"project/internal/models"
	"project/internal/storage"
)

func testConstructionDocuments(t *testing.T, r *gin.Engine, admin, worker *http.Cookie) {
	t.Helper()
	for _, name := range []string{"serif-regular.ttf", "serif-bold.ttf", "serif-italic.ttf"} {
		req := httptest.NewRequest("GET", "/document-fonts/"+name, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "font/ttf") || !bytes.HasPrefix(w.Body.Bytes(), []byte{0, 1, 0, 0}) {
			t.Fatal("document font", name, w.Code)
		}
	}
	fontRequest := httptest.NewRequest("GET", "/document-fonts/unknown.ttf", nil)
	fontResponse := httptest.NewRecorder()
	r.ServeHTTP(fontResponse, fontRequest)
	if fontResponse.Code != 404 {
		t.Fatal("unknown font")
	}
	call := func(method, path string, body any, cookie *http.Cookie, token string) *httptest.ResponseRecorder {
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		if cookie != nil {
			req.AddCookie(cookie)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	page := call("GET", "/object/object", nil, admin, "").Body.String()
	if !strings.Contains(page, `href="#object-documents"`) {
		t.Fatal("document tab missing")
	}
	token := regexp.MustCompile(`data-csrf="([^"]+)"`).FindStringSubmatch(page)[1]
	base := "/object/object/documents"
	obj, err := storage.GetObjectByID("object")
	if err != nil {
		t.Fatal(err)
	}
	before := obj
	obj.ResponsibleUserID = "admin"
	if err = storage.UpdateObject(obj); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.UpdateObject(before) })
	if w := call("GET", base, nil, worker, ""); w.Code != 403 {
		t.Fatal("worker read", w.Code)
	}
	if w := call("GET", base+"/sources?from=2026-09-01&to=2026-09-30", nil, worker, ""); w.Code != 403 {
		t.Fatal("source privacy", w.Code)
	}
	if strings.Contains(call("GET", "/object/object", nil, worker, "").Body.String(), `id="object-documents"`) {
		t.Fatal("worker tab exposed")
	}
	content := models.ConstructionContent{Kind: "act", Fields: map[string]string{"work_name": "Монтаж <script>alert(1)</script>", "object_name": "Объект", "address": "Минск"}}
	payload := func(id string, version int64, c models.ConstructionContent, reason string) any {
		return map[string]any{"id": id, "version": version, "content": c, "reason": reason}
	}
	requisites := models.ConstructionContent{Kind: "requisites", Fields: map[string]string{"quality_rep": "Прораб Петров Петр", "technical_rep": "Внешний инженер Согласованный С. С."}}
	if res := call("POST", base, payload("", 0, requisites, ""), admin, token); res.Code != 200 {
		t.Fatal("automation defaults", res.Code, res.Body.String())
	}
	if w := call("POST", base, payload("", 0, content, ""), admin, ""); w.Code != 403 {
		t.Fatal("CSRF", w.Code)
	}
	w := call("POST", base, payload("", 0, content, ""), admin, token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var doc models.ConstructionDocument
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Content.Number != "АСР-001" || len(doc.History) != 1 || doc.Version != 1 {
		t.Fatal("new document", doc)
	}
	if w := call("GET", base+"/"+doc.ID, nil, worker, ""); w.Code != 403 {
		t.Fatal("detail privacy", w.Code)
	}
	if w := call("GET", base+"/"+doc.ID+"/pdf", nil, worker, ""); w.Code != 403 {
		t.Fatal("PDF privacy", w.Code)
	}
	other, err := storage.CreateObject(models.Object{Name: "Документы другого объекта", Address: "Минск", ResponsibleUserID: "admin", Status: "in_progress"})
	if err != nil {
		t.Fatal(err)
	}
	otherBase := "/object/" + other.ID + "/documents"
	if w := call("GET", otherBase+"/"+doc.ID, nil, admin, ""); w.Code != 404 {
		t.Fatal("cross object GET", w.Code)
	}
	if w := call("POST", otherBase, payload(doc.ID, 1, doc.Content, "Изменение"), admin, token); w.Code != 404 {
		t.Fatal("cross object POST", w.Code)
	}
	content = doc.Content
	content.Fields["work_name"] = "Уточнённые работы"
	if w := call("POST", base, payload(doc.ID, 1, content, ""), admin, token); w.Code != 422 {
		t.Fatal("reason required", w.Code)
	}
	w = call("POST", base, payload(doc.ID, 1, content, "Уточнение наименования по проекту"), admin, token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("POST", base, payload(doc.ID, 1, content, "Повтор"), admin, token); w.Code != 409 {
		t.Fatal("conflict", w.Code)
	}
	if err = storage.LoadConstructionDocuments(); err != nil {
		t.Fatal(err)
	}
	saved, err := storage.GetConstructionDocument("object", doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Version != 2 || saved.History[0].Content.Fields["work_name"] == content.Fields["work_name"] {
		t.Fatal("history overwritten")
	}
	saved.Content.Fields["work_name"] = "mutated externally"
	again, _ := storage.GetConstructionDocument("object", doc.ID)
	if again.Content.Fields["work_name"] == "mutated externally" {
		t.Fatal("mutable snapshot")
	}
	for _, suffix := range []string{"/pdf", "/pdf?version=1"} {
		w := call("GET", base+"/"+doc.ID+suffix, nil, admin, "")
		if w.Code != 200 || !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF-")) {
			t.Fatal("PDF", w.Code, w.Body.String())
		}
	}
	if w := call("GET", base+"/"+doc.ID+"/pdf?version=100", nil, admin, ""); w.Code != 404 {
		t.Fatal("unknown revision")
	}
	content.Fields["start_date"] = "2026-10-02"
	content.Fields["end_date"] = "2026-10-01"
	if w := call("POST", base, payload(doc.ID, 2, content, "Ошибочная дата"), admin, token); w.Code != 422 {
		t.Fatal("date validation", w.Code)
	}
	w = call("GET", base+"/sources?from=2026-09-01&to=2026-09-30", nil, admin, "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var source struct {
		Rows []models.ConstructionRow `json:"rows"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &source)
	if len(source.Rows) != 2 || source.Rows[0].Reviewed || source.Rows[0].Fields["weather"] != "" || source.Rows[0].Fields["quantity"] != "" {
		t.Fatal("source fabrication", w.Body.String())
	}
	if !strings.Contains(source.Rows[0].Fields["crew"], "Иванов Иван") || !strings.Contains(source.Rows[0].Fields["persons"], "Администратор") || !strings.Contains(source.Rows[0].Fields["persons"], "Прораб Петров Петр") {
		t.Fatal("missing crew or internal responsible", source.Rows[0])
	}
	for _, tc := range []struct {
		cookie *http.Cookie
		csrf   string
	}{{worker, token}, {admin, ""}} {
		if res := call("POST", base+"/assignments", map[string]any{}, tc.cookie, tc.csrf); res.Code != 403 {
			t.Fatal("assignment draft access", res.Code)
		}
	}
	for _, body := range []map[string]any{{"from": "bad"}, {"from": "2026-09-30", "to": "2026-09-01"}, {"excluded": []string{"act:other"}}} {
		if res := call("POST", base+"/assignments", body, admin, token); res.Code != 422 {
			t.Fatal("assignment validation", res.Code, res.Body.String())
		}
	}
	if res := call("POST", base+"/assignments", map[string]any{"from": "2200-01-01"}, admin, token); res.Code != 200 || !strings.Contains(res.Body.String(), `"rows":[]`) {
		t.Fatal("future journal", res.Code, res.Body.String())
	}
	allSources := append(append([]string{}, source.Rows[0].Sources...), source.Rows[1].Sources...)
	if res := call("POST", base+"/assignments", map[string]any{"from": "2020-01-01", "to": "2200-12-31", "excluded": allSources}, admin, token); res.Code != 200 || !strings.Contains(res.Body.String(), `"rows":[]`) {
		t.Fatal("reimport duplicates", res.Code, res.Body.String())
	}
	if res := call("POST", otherBase+"/assignments", map[string]any{}, admin, token); res.Code != 200 || !strings.Contains(res.Body.String(), `"rows":[]`) {
		t.Fatal("cross object assignments", res.Code, res.Body.String())
	}
	addedEntry, err := storage.CreateTimesheet(models.TimesheetEntry{Date: "2026-09-21", StartTime: "08:00", EndTime: "17:00", WorkerIDs: []string{"other"}, ObjectIDs: []string{"object"}, Notes: "NEW_SAME_SHIFT", CreatedByID: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	res := call("POST", base+"/assignments", map[string]any{"from": "2026-09-01", "to": "2026-09-30", "excluded": allSources}, admin, token)
	var fresh struct {
		Rows []models.ConstructionRow `json:"rows"`
	}
	if err = json.Unmarshal(res.Body.Bytes(), &fresh); err != nil || res.Code != 200 || len(fresh.Rows) != 1 {
		t.Fatal("incremental import", res.Code, res.Body.String())
	}
	if fresh.Rows[0].Fields["crew"] != "Прораб Петров Петр" || fresh.Rows[0].Fields["description"] != "NEW_SAME_SHIFT" || len(fresh.Rows[0].Sources) != 1 || fresh.Rows[0].Sources[0] != "assignment:"+addedEntry.ID || fresh.Rows[0].Reviewed {
		t.Fatal("old assignment copied into new shift", fresh.Rows)
	}
	if err = storage.DeleteTimesheet(addedEntry.ID); err != nil {
		t.Fatal(err)
	}
	staffResponse := call("GET", base+"/sources?from=2026-09-01&to=2026-09-30&section=2", nil, admin, "")
	var staff struct {
		Rows []models.ConstructionRow `json:"rows"`
	}
	_ = json.Unmarshal(staffResponse.Body.Bytes(), &staff)
	if len(staff.Rows) != 1 || !strings.Contains(staff.Rows[0].Fields["person"], "Петров Петр") || staff.Rows[0].Fields["qualification"] != "" {
		t.Fatal("staff role inference", staffResponse.Body.String())
	}
	journal := models.ConstructionContent{Kind: "journal", Fields: map[string]string{"object_name": "Объект"}, Rows: source.Rows}
	w = call("POST", base, payload("", 0, journal, ""), admin, token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var j models.ConstructionDocument
	_ = json.Unmarshal(w.Body.Bytes(), &j)
	w = call("GET", base+"/"+j.ID+"/pdf", nil, admin, "")
	if w.Code != 200 || w.Body.Len() < 10000 {
		t.Fatal("journal PDF", w.Code)
	}
	journal.Rows[0].Reviewed = true
	if w := call("POST", base, payload(j.ID, 1, journal, "Проверка"), admin, token); w.Code != 422 {
		t.Fatal("empty reviewed row allowed")
	}
	if err := storage.ValidateConstructionDocuments([]models.ConstructionDocument{again, j}); err != nil {
		t.Fatal("migration validation", err)
	}
	docBytes, _ := json.Marshal([]models.ConstructionDocument{again, j})
	userBytes, _ := json.Marshal([]models.User{{ID: "admin", Username: "admin", Password: "fixture", Status: "admin"}})
	objectBytes, _ := json.Marshal([]models.Object{{ID: "object", Name: "Объект"}})
	report, err := storage.ValidateDocuments(map[string][]byte{"construction_documents.json": docBytes, "users.json": userBytes, "objects.json": objectBytes}, "")
	if err != nil || report.Counts["construction_documents"] != 2 || report.Counts["construction_revisions"] != 3 {
		t.Fatal("migration report", report, err)
	}
	if err = storage.LoadConstructionDocuments(); err != nil {
		t.Fatal(err)
	}
	auto, err := storage.ConstructionAutomation(obj)
	if err != nil {
		t.Fatal(err)
	}
	if auto.Defaults["responsible"] != "Администратор" || len(auto.Suggestions["work_name"]) != 1 || auto.Suggestions["work_name"][0] != "Уточнённые работы" || len(auto.People) == 0 {
		t.Fatal("memory after reload", auto)
	}
	for _, path := range []string{"/cities?q=Минск", "/weather?city=Минск&latitude=53.9&longitude=27.5&from=2026-09-21&to=2026-09-21"} {
		if res := call("GET", base+path, nil, worker, ""); res.Code != 403 {
			t.Fatal("weather access", res.Code)
		}
	}
}
