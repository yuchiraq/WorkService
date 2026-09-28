package router

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"project/internal/models"
	"project/internal/storage"
)

func TestSiteIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(old, "../.."))
	dir := t.TempDir()
	if err = os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err = os.MkdirAll("storage", 0755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, v any) {
		t.Helper()
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile("storage/"+name+".json", b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	write("users", []models.User{{ID: "admin", Username: "admin", Password: "testpass", Name: "Администратор", Status: "admin"}, {ID: "user", Username: "worker", Password: "testpass", Name: "Иванов Иван", Status: "user"}, {ID: "other", Username: "other", Password: "testpass", Name: "Петров Петр", Status: "user"}})
	write("workers", []models.Worker{{ID: "own", UserID: "user", Name: "Иванов Иван", Position: "Монтажник", HourlyRate: 15, BirthDate: "1990-04-12"}, {ID: "other", UserID: "other", Name: "Петров Петр", Position: "Прораб"}, {ID: "fired", Name: "Сидоров Сергей", IsFired: true}, {ID: "idle", Name: "Без назначений"}})
	write("objects", []models.Object{{ID: "object", Name: "Жилой комплекс на улице Центральной", Address: "Минск, Центральная 15", Status: "in_progress"}})
	write("timesheets", []models.TimesheetEntry{
		{ID: "own-entry", Date: "2026-09-21", StartTime: "08:00", EndTime: "17:00", LunchBreakMinutes: 60, WorkerIDs: []string{"own"}, ObjectIDs: []string{"object"}, CreatedByID: "user", CreatedByName: "Иванов Иван"},
		{ID: "private-entry", Date: "2026-09-22", StartTime: "08:00", EndTime: "17:00", LunchBreakMinutes: 60, WorkerIDs: []string{"other"}, ObjectIDs: []string{"object"}, CreatedByID: "other", Notes: "PRIVATE_OTHER_WORK"},
		{ID: "fired-entry", Date: "2026-09-01", StartTime: "08:00", EndTime: "17:00", LunchBreakMinutes: 60, WorkerIDs: []string{"fired"}},
		{ID: "absence", Date: "2026-09-02", PeriodEnd: "2026-09-06", UserMark: "Б", WorkerIDs: []string{"own"}, CreatedByID: "admin"},
	})
	for _, load := range []func() error{storage.LoadUsers, storage.LoadWorkers, storage.LoadObjects, storage.LoadTimesheets, storage.LoadImprovements, storage.LoadAppSettings, storage.LoadTelegramContacts, storage.LoadVehicles} {
		if err = load(); err != nil {
			t.Fatal(err)
		}
	}
	vehicle, err := storage.CreateVehicle(models.Vehicle{Name: "Ford Transit", RegistrationNumber: "1234 AB-7", AssignedUserID: "user", CreatedByUserID: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = storage.CreateVehicleRecord(models.VehicleRecord{VehicleID: vehicle.ID, Type: "fuel", Date: "2026-09-21", Mileage: 123400, Liters: 45, Amount: 110, CreatedByID: "user", CreatedByName: "Иванов Иван"})
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("WORKSERVICE_UI_PREVIEW") == "1" {
		if err = os.CopyFS("web", os.DirFS(filepath.Join(root, "web"))); err != nil {
			t.Fatal(err)
		}
		_ = os.Mkdir("gallery", 0755)
		img := image.NewRGBA(image.Rect(0, 0, 600, 400))
		for y := 0; y < 400; y++ {
			for x := 0; x < 600; x++ {
				img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 130, 255})
			}
		}
		f, _ := os.Create("gallery/test.jpg")
		_ = jpeg.Encode(f, img, nil)
		_ = f.Close()
		settings, _ := storage.GetAppSettings()
		settings.GalleryDirectory = filepath.Join(dir, "gallery")
		_ = storage.UpdateAppSettings(settings)
		r := gin.New()
		SetupRouter(r)
		t.Log("preview vehicle", vehicle.ID)
		if err = http.ListenAndServe("127.0.0.1:8100", r); err != nil {
			t.Fatal(err)
		}
		return
	}
	r := gin.New()
	SetupRouter(r)
	request := func(method, path string, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		var body io.Reader
		if form != nil {
			body = strings.NewReader(form.Encode())
		}
		req := httptest.NewRequest(method, path, body)
		if form != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	login := func(name string) *http.Cookie {
		t.Helper()
		w := request("POST", "/login", url.Values{"username": {name}, "password": {"testpass"}}, nil)
		if w.Code != 302 {
			t.Fatal(w.Code)
		}
		return w.Result().Cookies()[0]
	}
	admin, user := login("admin"), login("worker")
	for _, path := range []string{"/worker/other", "/workers", "/workers/new", "/workers/edit/own", "/users", "/settings", "/gallery", "/schedule/edit/private-entry", "/schedule/edit/absence"} {
		w := request("GET", path, nil, user)
		if w.Code != 403 {
			t.Errorf("%s: want 403 got %d", path, w.Code)
		}
	}
	for _, path := range []string{"/worker/own", "/profile", "/schedule", "/timesheets", "/object/object"} {
		w := request("GET", path, nil, user)
		if w.Code != 200 {
			t.Errorf("%s: %d", path, w.Code)
		}
		if strings.Contains(w.Body.String(), "PRIVATE_OTHER_WORK") {
			t.Errorf("private data leaked: %s", path)
		}
	}
	profile := request("GET", "/profile", nil, user).Body.String()
	if !strings.Contains(profile, `value="Монтажник"`) || !strings.Contains(profile, `value="15.00"`) || !strings.Contains(profile, `value="1990-04-12"`) {
		t.Fatal("profile values missing")
	}
	csrf := regexp.MustCompile(`name="_csrf_token" value="([^"]+)"`).FindStringSubmatch(profile)[1]
	for _, path := range []string{"/schedule/edit/private-entry", "/schedule/delete/private-entry", "/schedule/edit/absence", "/workers/edit/own", "/users/new", "/gallery/delete"} {
		w := request("POST", path, url.Values{"_csrf_token": {csrf}}, user)
		if w.Code != 403 {
			t.Errorf("POST %s: %d", path, w.Code)
		}
	}
	if w := request("POST", "/settings/backup", nil, admin); w.Code != 403 {
		t.Error("admin POST without CSRF accepted")
	}
	w := request("POST", "/profile", url.Values{"_csrf_token": {csrf}, "username": {"worker"}, "name": {"Иванов Иван"}, "hourly_rate": {"999"}, "position": {"Директор"}}, user)
	if w.Code != 302 {
		t.Fatal(w.Body.String())
	}
	own, _ := storage.GetWorkerByID("own")
	if own.HourlyRate != 15 || own.Position != "Монтажник" {
		t.Fatal("worker changed privileged fields")
	}
	for _, month := range []string{"2026-09", "2026-08"} {
		html := request("GET", "/timesheets?month="+month, nil, admin).Body.String()
		if strings.Contains(html, "Без назначений") {
			t.Fatal("idle worker in timesheet")
		}
		if month == "2026-09" && !strings.Contains(html, "Сидоров") {
			t.Fatal("fired worker missing")
		}
		if month == "2026-08" && strings.Contains(html, "Сидоров") {
			t.Fatal("worker leaked into empty month")
		}
	}
	export := request("GET", "/timesheets/export?month=2026-09", nil, admin)
	book, err := excelize.OpenReader(bytes.NewReader(export.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	rows, _ := book.GetRows("Табель")
	all := ""
	for _, row := range rows {
		all += strings.Join(row, "|")
	}
	if !strings.Contains(all, "Сидоров Сергей") || strings.Contains(all, "Без назначений") {
		t.Fatal("export worker selection differs")
	}
	schedule := request("GET", "/schedule?month=2026-09", nil, admin).Body.String()
	if strings.Count(schedule, "schedule/edit/absence?") != 4 {
		t.Fatal("absence should have two boundary actions")
	}
	post := url.Values{"_csrf_token": {csrf}, "date": {"2026-09-27"}, "period_end": {"2026-10-02"}, "special_mark": {"ПР"}, "worker_ids": {"other"}}
	w = request("POST", "/schedule/new", post, user)
	if w.Code != 302 {
		t.Fatal(w.Body.String())
	}
	entries, _ := storage.GetTimesheets()
	var created models.TimesheetEntry
	for _, e := range entries {
		if e.UserMark == "ПР" {
			created = e
		}
	}
	if created.PeriodEnd != "2026-10-02" || len(created.WorkerIDs) != 1 || created.WorkerIDs[0] != "own" {
		t.Fatalf("absence scope: %+v", created)
	}
	post.Set("period_end", "2026-10-04")
	w = request("POST", "/schedule/edit/"+created.ID, post, user)
	if w.Code != 302 {
		t.Fatal(w.Body.String())
	}
	created, _ = storage.GetTimesheetByID(created.ID)
	if created.PeriodEnd != "2026-10-04" {
		t.Fatal("period edit not saved")
	}
	w = request("POST", "/schedule/delete/"+created.ID, url.Values{"_csrf_token": {csrf}}, user)
	if w.Code != 302 {
		t.Fatal(w.Code)
	}
	page := request("GET", "/vehicles/"+vehicle.ID, nil, user).Body.String()
	if !strings.Contains(page, `value="fuel" selected`) {
		t.Fatal("fuel not default")
	}
	request("GET", "/logout", nil, user)
	if w = request("GET", "/worker/own", nil, user); w.Code != 302 {
		t.Fatal("logged-out session accepted")
	}
}
