package api

import (
	"bytes"
	"net/http/httptest"
	"testing"
	"time"

	"project/internal/models"

	"github.com/gin-gonic/gin"
)

func TestCanDeleteVehicleRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("userID", "author")
	context.Set("userStatus", "user")
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	record := models.VehicleRecord{CreatedByID: "author", CreatedAt: now.Add(-23 * time.Hour).Format(time.RFC3339)}
	if !canDeleteVehicleRecord(context, record, now) {
		t.Fatal("author could not delete a recent record")
	}
	context.Set("userID", "other")
	if canDeleteVehicleRecord(context, record, now) {
		t.Fatal("another user could delete the record")
	}
	context.Set("userStatus", "admin")
	if !canDeleteVehicleRecord(context, record, now) {
		t.Fatal("administrator could not delete a recent record")
	}
	record.CreatedAt = now.Add(-25 * time.Hour).Format(time.RFC3339)
	if canDeleteVehicleRecord(context, record, now) {
		t.Fatal("expired record remained deletable")
	}
}

func TestVehicleRecordsLabel(t *testing.T) {
	for count, expected := range map[int]string{1: "1 запись", 2: "2 записи", 5: "5 записей", 11: "11 записей", 21: "21 запись"} {
		if actual := vehicleRecordsLabel(count); actual != expected {
			t.Fatalf("vehicleRecordsLabel(%d) = %q", count, actual)
		}
	}
}

func TestBuildVehicleWaybillData(t *testing.T) {
	month := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.Local)
	records := []models.VehicleRecord{
		{Date: "2026-07-31", Type: "mileage", Mileage: 120000, CreatedAt: "2026-07-31T18:00:00+03:00"},
		{Date: "2026-08-03", Type: "fuel", Mileage: 120150, Liters: 45.5, Amount: 120.25, Notes: "Объект № 1", CreatedAt: "2026-08-03T08:00:00+03:00"},
		{Date: "2026-08-20", Type: "mileage", Mileage: 120700, CreatedAt: "2026-08-20T18:00:00+03:00"},
	}
	data := buildVehicleWaybillData(models.Vehicle{Name: "Ford Transit", RegistrationNumber: "А123ВС-7"}, "Иван Петров", records, month)
	if data.OpeningMileage != 120000 || data.ClosingMileage != 120700 || data.TotalMileage != 700 {
		t.Fatalf("unexpected mileage summary: %#v", data)
	}
	if data.TotalLiters != 45.5 || data.TotalAmount != 120.25 || len(data.Rows) != 2 {
		t.Fatalf("unexpected totals: %#v", data)
	}
}

func TestBuildVehicleWaybillPDF(t *testing.T) {
	data := vehicleWaybillData{
		Month: "2026-08", PeriodLabel: "01-31 августа 2026 г.", VehicleName: "Ford Transit", Registration: "А123ВС-7", DriverName: "Иван Петров",
		OpeningMileage: 120000, ClosingMileage: 120700, TotalMileage: 700, TotalLiters: 45.5, TotalAmount: 120.25,
		Rows: []vehicleWaybillRow{{Date: "03.08.2026", RecordType: "Заправка", Mileage: "120150", MileageDiff: "150", Liters: "45,50", Amount: "120,25", Description: "Работа на объекте"}},
	}
	result, err := buildVehicleWaybillPDF(data, time.Date(2026, time.August, 31, 18, 30, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("buildVehicleWaybillPDF() error = %v", err)
	}
	if len(result) < 1000 || !bytes.HasPrefix(result, []byte("%PDF-")) {
		t.Fatalf("invalid PDF output: %d bytes", len(result))
	}
}
