package router

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"project/internal/models"
	"project/internal/persistence"
	"project/internal/storage"
)

func TestSiteIntegration(t *testing.T) {
	for _, mode := range []string{"json", "sqlite"} {
		t.Run(mode, func(t *testing.T) { testSiteIntegration(t, mode) })
	}
}

func testSiteIntegration(t *testing.T, mode string) {
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
	write("users", []models.User{{ID: "admin", Username: "admin", Password: "testpass", Name: "Администратор", Status: "admin"}, {ID: "user", Username: "worker", Password: "testpass", Name: "Иванов Иван", Status: "user"}, {ID: "other", Username: "other", Password: "testpass", Name: "Петров Петр", Status: "user"}, {ID: "admin2", Username: "admin2", Password: "testpass", Name: "Второй администратор", Status: "admin"}})
	write("workers", []models.Worker{{ID: "own", UserID: "user", Name: "Иванов Иван", Position: "Монтажник", HourlyRate: 15, BirthDate: "1990-04-12"}, {ID: "other", UserID: "other", Name: "Петров Петр", Position: "Прораб"}, {ID: "fired", Name: "Сидоров Сергей", IsFired: true}, {ID: "idle", Name: "Без назначений"}})
	write("objects", []models.Object{{ID: "object", Name: "Жилой комплекс на улице Центральной", Address: "Минск, Центральная 15", Status: "in_progress"}})
	write("timesheets", []models.TimesheetEntry{
		{ID: "own-entry", Date: "2026-09-21", StartTime: "08:00", EndTime: "17:00", LunchBreakMinutes: 60, WorkerIDs: []string{"own"}, ObjectIDs: []string{"object"}, CreatedByID: "user", CreatedByName: "Иванов Иван"},
		{ID: "private-entry", Date: "2026-09-22", StartTime: "08:00", EndTime: "17:00", LunchBreakMinutes: 60, WorkerIDs: []string{"other"}, ObjectIDs: []string{"object"}, CreatedByID: "other", Notes: "PRIVATE_OTHER_WORK"},
		{ID: "fired-entry", Date: "2026-09-01", StartTime: "08:00", EndTime: "17:00", LunchBreakMinutes: 60, WorkerIDs: []string{"fired"}},
		{ID: "absence", Date: "2026-09-02", PeriodEnd: "2026-09-06", UserMark: "Б", WorkerIDs: []string{"own"}, CreatedByID: "admin"},
	})
	t.Setenv("WORKSERVICE_STORAGE", mode)
	t.Setenv("WORKSERVICE_SQLITE_PATH", filepath.Join(dir, "storage", "workservice.db"))
	t.Setenv("WORKSERVICE_PHOTO_DIR", filepath.Join(dir, "storage", "object_photos"))
	t.Setenv("WORKSERVICE_CLOUD_DIR", filepath.Join(dir, "storage", "cloud"))
	t.Setenv("WORKSERVICE_CLOUD_QUOTA_MB", "10")
	t.Setenv("WORKSERVICE_CLOUD_MAX_FILE_MB", "2")
	if mode == "sqlite" {
		db, e := persistence.OpenSQLite(os.Getenv("WORKSERVICE_SQLITE_PATH"), true)
		if e != nil {
			t.Fatal(e)
		}
		documents, e := persistence.OpenJSON("storage").Snapshot()
		if e != nil {
			t.Fatal(e)
		}
		if e = db.Import(documents); e != nil {
			t.Fatal(e)
		}
		if e = db.Close(); e != nil {
			t.Fatal(e)
		}
	}
	closeStorage, err := storage.ConfigureFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := closeStorage(); err != nil {
			t.Error(err)
		}
	})
	for _, load := range []func() error{storage.LoadUsers, storage.LoadWorkers, storage.LoadObjects, storage.LoadTimesheets, storage.LoadImprovements, storage.LoadAppSettings, storage.LoadTelegramContacts, storage.LoadVehicles, storage.LoadInventory, storage.LoadObjectPhotos, storage.LoadCloud} {
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
		port := os.Getenv("WORKSERVICE_UI_PORT")
		if port == "" {
			port = "8099"
		}
		if number, e := strconv.Atoi(port); e != nil || number < 1 || number > 65535 {
			t.Fatal("WORKSERVICE_UI_PORT must be a port number from 1 to 65535")
		}
		t.Log("isolated preview: http://127.0.0.1:" + port)
		if err = http.ListenAndServe("127.0.0.1:"+port, r); err != nil {
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
	t.Run("quick actions and object tabs", func(t *testing.T) {
		workerPage := request("GET", "/object/object", nil, user).Body.String()
		for _, expected := range []string{"data-quick-actions", "/vehicles/" + vehicle.ID + "/records/new", "data-object-tabs", `href="#object-photos"`, `href="#object-inventory"`, `href="#object-assignments"`} {
			if !strings.Contains(workerPage, expected) {
				t.Errorf("worker object page missing %s", expected)
			}
		}
		adminPage := request("GET", "/objects", nil, admin).Body.String()
		if strings.Contains(adminPage, "data-quick-actions") || !strings.Contains(adminPage, "data-fluid-navigation") {
			t.Fatal("admin without owned vehicles must not see a fleet dial; list tabs must be present")
		}
		vehiclePage := request("GET", "/vehicles/"+vehicle.ID, nil, user).Body.String()
		if !strings.Contains(vehiclePage, `id="vehicle-record-form"`) {
			t.Fatal("vehicle record deep link missing")
		}
	})
	t.Run("personal cloud", func(t *testing.T) { testPersonalCloud(t, r, admin, login("admin2"), user) })
	t.Run("cloud sharing", func(t *testing.T) { testCloudSharing(t, r, admin, login("admin2"), user) })
	t.Run("quick vehicle records", func(t *testing.T) { testQuickVehicleRecords(t, r, vehicle.ID, admin, user, login("other")) })
	t.Run("assignment privacy", func(t *testing.T) { testAssignmentPrivacy(t, r, admin, user) })
	for _, path := range []string{"/worker/other", "/workers", "/workers/new", "/workers/edit/own", "/objects/new", "/objects/edit/object", "/users", "/settings", "/gallery", "/schedule/edit/private-entry", "/schedule/edit/absence"} {
		w := request("GET", path, nil, user)
		if w.Code != 403 {
			t.Errorf("%s: want 403 got %d", path, w.Code)
		}
	}
	for _, path := range []string{"/worker/own", "/profile", "/schedule", "/timesheets", "/objects", "/object/object"} {
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
	t.Run("object photos", func(t *testing.T) {
		var source bytes.Buffer
		_ = jpeg.Encode(&source, image.NewRGBA(image.Rect(0, 0, 80, 40)), nil)
		upload := func(path, token string, data []byte) *httptest.ResponseRecorder {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, e := writer.CreateFormFile("photo", "phone.jpg")
			if e != nil {
				t.Fatal(e)
			}
			_, _ = part.Write(data)
			_ = writer.Close()
			req := httptest.NewRequest("POST", path, &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("X-CSRF-Token", token)
			req.AddCookie(user)
			response := httptest.NewRecorder()
			r.ServeHTTP(response, req)
			return response
		}
		if w := upload("/object/object/photos", "", source.Bytes()); w.Code != 403 {
			t.Fatal("upload without CSRF", w.Code)
		}
		if w := upload("/object/object/photos", csrf, []byte("invalid")); w.Code != 400 {
			t.Fatal("invalid image accepted", w.Code)
		}
		if w := upload("/object/object/photos", csrf, source.Bytes()); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		photos := storage.GetObjectPhotos("object")
		if len(photos) != 1 {
			t.Fatal("photo missing")
		}
		photo := photos[0]
		if w := upload("/object/object/photos", csrf, source.Bytes()); w.Code != 200 || !strings.Contains(w.Body.String(), `"duplicate":true`) {
			t.Fatal("duplicate photo", w.Body.String())
		}
		private, e := storage.CreateObject(models.Object{Name: "Private photo object", Address: "Minsk", ResponsibleUserID: "admin"})
		if e != nil {
			t.Fatal(e)
		}
		for _, path := range []string{"/object/" + private.ID + "/photos", "/object/" + private.ID + "/photos/" + photo.ID + "/image"} {
			if w := request("GET", path, nil, user); w.Code != 403 {
				t.Fatal("private photo access", w.Code)
			}
		}
		if w := upload("/object/"+private.ID+"/photos", csrf, source.Bytes()); w.Code != 403 {
			t.Fatal("private upload", w.Code)
		}
		path := "/object/object/photos/" + photo.ID
		for _, suffix := range []string{"/image", "/image?thumb=1", "/image?download=1"} {
			w := request("GET", path+suffix, nil, user)
			if w.Code != 200 || w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal(w.Code, w.Header())
			}
			if _, _, e = image.DecodeConfig(bytes.NewReader(w.Body.Bytes())); e != nil {
				t.Fatal(e)
			}
		}
		if w := request("GET", path+"/image", nil, nil); w.Code == 200 {
			t.Fatal("anonymous photo access")
		}
		other := login("other")
		token := regexp.MustCompile(`name="_csrf_token" value="([^"]+)"`).FindStringSubmatch(request("GET", "/profile", nil, other).Body.String())[1]
		if w := request("POST", path+"/delete", url.Values{"_csrf_token": {token}, "version": {"1"}}, other); w.Code != 403 {
			t.Fatal("another worker deleted photo", w.Code)
		}
		if w := request("POST", path+"/caption", url.Values{"_csrf_token": {csrf}, "version": {"1"}, "caption": {"</script><script>alert(1)</script>"}}, user); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if strings.Contains(request("GET", "/object/object", nil, user).Body.String(), "</script><script>alert(1)</script>") {
			t.Fatal("unsafe photo caption")
		}
		if w := request("POST", path+"/delete", url.Values{"_csrf_token": {csrf}, "version": {"1"}}, user); w.Code != 409 {
			t.Fatal("stale photo deletion", w.Code)
		}
		worker, _ := storage.GetWorkerByID("own")
		worker.IsFired = true
		_ = storage.UpdateWorker(worker)
		if w := request("GET", path+"/image", nil, user); w.Code != 403 {
			t.Fatal("fired worker retained photo access")
		}
		worker.IsFired = false
		_ = storage.UpdateWorker(worker)
		if e := storage.CreateDataBackup(filepath.Join(dir, "photo-backup")); e != nil {
			t.Fatal(e)
		}
		if _, _, e := storage.ReadMigrationSource(filepath.Join(dir, "photo-backup"), filepath.Join(dir, "photo-backup/object_photos")); e != nil {
			t.Fatal(e)
		}
		if w := request("POST", path+"/delete", url.Values{"_csrf_token": {csrf}, "version": {"2"}}, user); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if w := request("GET", path+"/image", nil, user); w.Code != 404 {
			t.Fatal("deleted photo still available")
		}
	})
	t.Run("object inventory", func(t *testing.T) {
		privateObject, err := storage.CreateObject(models.Object{Name: "Чужой объект", Address: "Минск", ResponsibleUserID: "admin"})
		if err != nil {
			t.Fatal(err)
		}
		responsibleObject, err := storage.CreateObject(models.Object{Name: "Ответственный", Address: "Минск", ResponsibleUserID: "user"})
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"/object/" + privateObject.ID, "/object/" + privateObject.ID + "/inventory"} {
			if w := request("GET", path, nil, user); w.Code != 403 {
				t.Fatalf("private object leaked: %s %d", path, w.Code)
			}
		}
		objectList := request("GET", "/objects", nil, user).Body.String()
		if strings.Contains(objectList, privateObject.ID) || strings.Contains(objectList, `/objects/new`) || strings.Contains(objectList, `/objects/edit/`) || !strings.Contains(objectList, responsibleObject.ID) || !strings.Contains(objectList, `/object/object#object-inventory`) {
			t.Fatal("worker objects list is not correctly scoped")
		}
		for _, path := range []string{"/object/object/inventory", "/object/" + responsibleObject.ID, "/object/" + responsibleObject.ID + "/inventory"} {
			if w := request("GET", path, nil, user); w.Code != 200 {
				t.Fatalf("assigned/responsible access: %s %d", path, w.Code)
			}
		}
		if w := request("POST", "/object/object/inventory", nil, user); w.Code != 403 {
			t.Fatal("missing CSRF accepted")
		}
		values := url.Values{"_csrf_token": {csrf}, "name": {"Дрель </script><script>alert(1)</script>"}, "unit": {"pcs"}, "quantity": {"2,5"}, "note": {"<img src=x onerror=alert(1)>"}}
		values.Set("material", "Нержавейка")
		if w := request("POST", "/object/"+privateObject.ID+"/inventory", values, user); w.Code != 403 {
			t.Fatal("private mutation accepted")
		}
		if w := request("POST", "/object/object/inventory", values, user); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		state := storage.GetInventory("object")
		if len(state.Items) != 1 || state.Items[0].Quantity != 2500 || state.Items[0].Material != "Нержавейка" || state.Catalog[0].Material != "Нержавейка" || state.History[0].ActorID != "user" {
			t.Fatal("inventory not persisted", state)
		}
		item := state.Items[0]
		page := request("GET", "/object/object", nil, user).Body.String()
		if strings.Contains(page, "</script><script>alert(1)</script>") || strings.Contains(page, "<img src=x onerror=alert(1)>") || !strings.Contains(page, `id="inventory-data"`) {
			t.Fatal("unsafe or missing inventory state")
		}
		itemPath := "/object/object/inventory/" + item.ID
		badValues := url.Values{"_csrf_token": {csrf}, "quantity": {"-1"}, "version": {"1"}}
		if w := request("POST", itemPath+"/quantity", badValues, user); w.Code != 400 || !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
			t.Fatal("invalid amount not rejected as JSON", w.Code)
		}
		badValues.Set("quantity", "3")
		if w := request("POST", "/object/"+responsibleObject.ID+"/inventory/"+item.ID+"/quantity", badValues, user); w.Code != 404 {
			t.Fatal("cross-object item accepted", w.Code)
		}
		if w := request("POST", itemPath+"/quantity", badValues, user); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if w := request("POST", itemPath+"/delete", badValues, user); w.Code != 409 {
			t.Fatal("stale deletion accepted", w.Code)
		}
		worker, _ := storage.GetWorkerByID("own")
		worker.IsFired = true
		if err := storage.UpdateWorker(worker); err != nil {
			t.Fatal(err)
		}
		if w := request("GET", "/object/object/inventory", nil, user); w.Code != 403 {
			t.Fatal("fired worker retained inventory access")
		}
		if w := request("POST", itemPath+"/adjust", url.Values{"_csrf_token": {csrf}, "delta": {"1000"}}, user); w.Code != 403 {
			t.Fatal("fired worker can edit inventory")
		}
		worker.IsFired = false
		if err := storage.UpdateWorker(worker); err != nil {
			t.Fatal(err)
		}
		other := login("other")
		otherPage := request("GET", "/profile", nil, other).Body.String()
		otherCSRF := regexp.MustCompile(`name="_csrf_token" value="([^"]+)"`).FindStringSubmatch(otherPage)[1]
		if w := request("POST", itemPath+"/adjust", url.Values{"_csrf_token": {otherCSRF}, "delta": {"1000"}}, other); w.Code != 200 {
			t.Fatal("another assigned worker cannot edit", w.Body.String())
		}
		adminPage := request("GET", "/profile", nil, admin).Body.String()
		adminCSRF := regexp.MustCompile(`name="_csrf_token" value="([^"]+)"`).FindStringSubmatch(adminPage)[1]
		if w := request("POST", itemPath+"/delete", url.Values{"_csrf_token": {adminCSRF}, "version": {"3"}}, admin); w.Code != 200 {
			t.Fatal("admin cannot delete", w.Body.String())
		}
		if state := storage.GetInventory("object"); len(state.Items) != 0 || len(state.Catalog) != 1 {
			t.Fatal("catalog lost after deletion")
		}
	})
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
	t.Run("availability privacy and date changes", func(t *testing.T) {
		w := request("GET", "/schedule/availability?date=2026-09-22", nil, user)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"other":true`) || strings.Contains(w.Body.String(), "PRIVATE_OTHER_WORK") || strings.Contains(w.Body.String(), "08:00") {
			t.Fatal("availability response", w.Code, w.Body.String())
		}
		w = request("GET", "/schedule/availability?date=2026-09-21&exclude_id=own-entry", nil, user)
		if w.Code != 200 || strings.Contains(w.Body.String(), `"own":true`) {
			t.Fatal("current entry not excluded", w.Body.String())
		}
		if w = request("GET", "/schedule/availability?date=2026-09-22&exclude_id=private-entry", nil, user); w.Code != 403 {
			t.Fatal("can exclude another worker's private entry")
		}
		if w = request("GET", "/schedule/availability?date=invalid", nil, user); w.Code != 400 {
			t.Fatal("invalid date accepted")
		}
		if w = request("GET", "/schedule/availability?date=2026-09-03", nil, user); !strings.Contains(w.Body.String(), `"own":true`) {
			t.Fatal("period not marked busy")
		}
	})
	t.Run("object status inventory warning", func(t *testing.T) {
		adminPage := request("GET", "/profile", nil, admin).Body.String()
		token := regexp.MustCompile(`name="_csrf_token" value="([^"]+)"`).FindStringSubmatch(adminPage)[1]
		object, err := storage.CreateObject(models.Object{Name: "Объект с остатком", Address: "Минск", ResponsibleUserID: "admin"})
		if err != nil {
			t.Fatal(err)
		}
		if err = storage.ChangeInventory(object.ID, "admin", "Администратор", storage.InventoryMutation{Action: "add", Name: "Лестница", Unit: "pcs", Quantity: 1000}); err != nil {
			t.Fatal(err)
		}
		values := url.Values{"_csrf_token": {token}, "name": {object.Name}, "address": {object.Address}, "responsible_user_id": {"admin"}, "status": {"paused"}}
		path := "/objects/edit/" + object.ID
		if w := request("POST", path, values, admin); w.Code != 409 {
			t.Fatal("pause without confirmation", w.Code, w.Body.String())
		}
		if got, _ := storage.GetObjectByID(object.ID); got.Status != "in_progress" {
			t.Fatal("warning already changed status")
		}
		values.Set("inventory_confirmed_status", "paused")
		if w := request("POST", path, values, admin); w.Code != 302 {
			t.Fatal(w.Body.String())
		}
		values.Set("status", "completed")
		if w := request("POST", path, values, admin); w.Code != 409 {
			t.Fatal("confirmation for another status accepted")
		}
		values.Set("inventory_confirmed_status", "completed")
		if w := request("POST", path, values, admin); w.Code != 302 {
			t.Fatal(w.Body.String())
		}
		item := storage.GetInventory(object.ID).Items[0]
		if item.Quantity != 1000 {
			t.Fatal("inventory silently removed")
		}
		if err := storage.ChangeInventory(object.ID, "admin", "", storage.InventoryMutation{Action: "quantity", ItemID: item.ID, Version: item.Version, Quantity: 0}); err != nil {
			t.Fatal(err)
		}
		values.Set("status", "paused")
		values.Del("inventory_confirmed_status")
		if w := request("POST", path, values, admin); w.Code != 302 {
			t.Fatal("zero stock requires confirmation", w.Body.String())
		}
	})
	t.Run("vehicle ownership controls and conflicts", func(t *testing.T) {
		adminPage := request("GET", "/profile", nil, admin).Body.String()
		token := regexp.MustCompile(`name="_csrf_token" value="([^"]+)"`).FindStringSubmatch(adminPage)[1]
		path := "/vehicles/" + vehicle.ID
		page := request("GET", path, nil, admin).Body.String()
		if strings.Contains(page, `name="assigned_user_id"`) || !strings.Contains(page, "Снять привязку") {
			t.Fatal("owner chooser shown for assigned vehicle")
		}
		request("POST", path+"/assign", url.Values{"_csrf_token": {token}, "assigned_user_id": {"other"}}, admin)
		if got, _ := storage.GetVehicleByID(vehicle.ID); got.AssignedUserID != "user" {
			t.Fatal("direct reassignment accepted")
		}
		request("POST", path+"/unassign", url.Values{"_csrf_token": {token}, "expected_owner_id": {"user"}}, admin)
		page = request("GET", path, nil, admin).Body.String()
		if !strings.Contains(page, `name="assigned_user_id"`) || strings.Contains(page, "Снять привязку") {
			t.Fatal("unassigned controls incorrect")
		}
		request("POST", path+"/assign", url.Values{"_csrf_token": {token}, "assigned_user_id": {"other"}}, admin)
		request("POST", path+"/unassign", url.Values{"_csrf_token": {token}, "expected_owner_id": {"user"}}, admin)
		if got, _ := storage.GetVehicleByID(vehicle.ID); got.AssignedUserID != "other" || len(got.OwnerHistory) != 3 {
			t.Fatal("stale unassign changed history", got)
		}
		request("POST", path+"/unassign", url.Values{"_csrf_token": {csrf}, "expected_owner_id": {"other"}}, user)
		if got, _ := storage.GetVehicleByID(vehicle.ID); got.AssignedUserID != "other" {
			t.Fatal("former owner unassigned current owner")
		}
		pdf := request("GET", path+"/waybill?month="+time.Now().Format("2006-01"), nil, admin)
		if pdf.Code != 200 || !bytes.HasPrefix(pdf.Body.Bytes(), []byte("%PDF-")) {
			t.Fatal("waybill export failed")
		}
		request("POST", path+"/unassign", url.Values{"_csrf_token": {token}, "expected_owner_id": {"other"}}, admin)
		request("POST", path+"/assign", url.Values{"_csrf_token": {token}, "assigned_user_id": {"user"}}, admin)
	})
	page := request("GET", "/vehicles/"+vehicle.ID, nil, user).Body.String()
	if !strings.Contains(page, `value="fuel" selected`) {
		t.Fatal("fuel not default")
	}
	request("GET", "/logout", nil, user)
	if w = request("GET", "/worker/own", nil, user); w.Code != 302 {
		t.Fatal("logged-out session accepted")
	}
}
