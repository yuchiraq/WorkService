package api

import (
	"bytes"
	"fmt"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"project/internal/models"
	"project/internal/security"
	"project/internal/storage"

	"codeberg.org/go-pdf/fpdf"
	"github.com/gin-gonic/gin"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

const vehicleWaybillOrganization = `ЧСУП "АВАЮССТРОЙ"`

type vehicleWaybillRow struct {
	Date        string
	RecordType  string
	Mileage     string
	MileageDiff string
	Liters      string
	Amount      string
	Description string
}

type vehicleWaybillData struct {
	Month          string
	PeriodLabel    string
	VehicleName    string
	Registration   string
	DriverName     string
	OpeningMileage int
	ClosingMileage int
	TotalMileage   int
	TotalLiters    float64
	TotalAmount    float64
	Rows           []vehicleWaybillRow
}

func parseVehicleWaybillMonth(value string) (time.Time, error) {
	month, err := time.Parse("2006-01", strings.TrimSpace(value))
	if err != nil || month.Year() < 2000 || month.Year() > 2100 {
		return time.Time{}, fmt.Errorf("invalid waybill month")
	}
	return month, nil
}

func vehicleWaybillPeriodLabel(month time.Time) string {
	monthNames := [...]string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}
	lastDay := month.AddDate(0, 1, -1).Day()
	return fmt.Sprintf("01-%02d %s %d г.", lastDay, monthNames[month.Month()-1], month.Year())
}

func formatWaybillDecimal(value float64) string {
	if value == 0 {
		return ""
	}
	return strings.ReplaceAll(strconv.FormatFloat(value, 'f', 2, 64), ".", ",")
}

func formatWaybillTotal(value float64) string {
	return strings.ReplaceAll(strconv.FormatFloat(value, 'f', 2, 64), ".", ",")
}

func buildVehicleWaybillData(vehicle models.Vehicle, driverName string, records []models.VehicleRecord, month time.Time) vehicleWaybillData {
	ordered := append([]models.VehicleRecord(nil), records...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Date == ordered[j].Date {
			return ordered[i].CreatedAt < ordered[j].CreatedAt
		}
		return ordered[i].Date < ordered[j].Date
	})

	monthStart := month.Format("2006-01-02")
	monthEnd := month.AddDate(0, 1, 0).Format("2006-01-02")
	previousMileage := 0
	for _, record := range ordered {
		if record.Date >= monthStart {
			break
		}
		if record.Mileage > 0 {
			previousMileage = record.Mileage
		}
	}

	data := vehicleWaybillData{
		Month:          month.Format("2006-01"),
		PeriodLabel:    vehicleWaybillPeriodLabel(month),
		VehicleName:    vehicle.Name,
		Registration:   vehicle.RegistrationNumber,
		DriverName:     strings.TrimSpace(driverName),
		OpeningMileage: previousMileage,
		ClosingMileage: previousMileage,
		Rows:           make([]vehicleWaybillRow, 0),
	}
	lastMileage := previousMileage
	for _, record := range ordered {
		if record.Date < monthStart || record.Date >= monthEnd {
			continue
		}
		row := vehicleWaybillRow{
			Date:        vehicleDateLabel(record.Date),
			RecordType:  vehicleRecordTypeLabel(record.Type),
			Liters:      formatWaybillDecimal(record.Liters),
			Amount:      formatWaybillDecimal(record.Amount),
			Description: strings.TrimSpace(record.Notes),
		}
		if record.Mileage > 0 {
			row.Mileage = strconv.Itoa(record.Mileage)
			if data.OpeningMileage == 0 {
				data.OpeningMileage = record.Mileage
			}
			if lastMileage > 0 && record.Mileage >= lastMileage {
				row.MileageDiff = strconv.Itoa(record.Mileage - lastMileage)
			}
			lastMileage = record.Mileage
			data.ClosingMileage = record.Mileage
		}
		data.TotalLiters += record.Liters
		data.TotalAmount += record.Amount
		data.Rows = append(data.Rows, row)
	}
	if data.ClosingMileage >= data.OpeningMileage {
		data.TotalMileage = data.ClosingMileage - data.OpeningMileage
	}
	if data.DriverName == "" {
		data.DriverName = "Не указан"
	}
	return data
}

func truncateWaybillText(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes-3]) + "..."
}

func buildVehicleWaybillPDF(data vehicleWaybillData, generatedAt time.Time) ([]byte, error) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(true, 15)
	pdf.SetTitle("Путевой лист за "+data.Month, true)
	pdf.SetAuthor(vehicleWaybillOrganization, true)
	pdf.SetCreator("WorkService", true)
	pdf.AddUTF8FontFromBytes("Go", "", goregular.TTF)
	pdf.AddUTF8FontFromBytes("Go", "B", gobold.TTF)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Go", "", 7)
		pdf.SetTextColor(90, 98, 105)
		pdf.CellFormat(0, 5, fmt.Sprintf("Сформировано %s · Страница %d", generatedAt.Format("02.01.2006 15:04"), pdf.PageNo()), "", 0, "R", false, 0, "")
	})

	addDocumentHeader := func() {
		pdf.SetTextColor(28, 37, 45)
		pdf.SetFont("Go", "B", 14)
		pdf.CellFormat(0, 8, "ПУТЕВОЙ ЛИСТ / НАКОПИТЕЛЬНАЯ ВЕДОМОСТЬ", "", 1, "C", false, 0, "")
		pdf.SetFont("Go", "", 9)
		pdf.CellFormat(0, 6, "за период "+data.PeriodLabel, "", 1, "C", false, 0, "")
		pdf.Ln(2)
		pdf.SetFont("Go", "B", 9)
		pdf.CellFormat(42, 6, "Организация:", "", 0, "L", false, 0, "")
		pdf.SetFont("Go", "", 9)
		pdf.CellFormat(95, 6, vehicleWaybillOrganization, "B", 0, "L", false, 0, "")
		pdf.SetFont("Go", "B", 9)
		pdf.CellFormat(32, 6, "Документ №:", "", 0, "L", false, 0, "")
		pdf.SetFont("Go", "", 9)
		pdf.CellFormat(0, 6, data.Registration+"-"+strings.ReplaceAll(data.Month, "-", ""), "B", 1, "L", false, 0, "")
		pdf.SetFont("Go", "B", 9)
		pdf.CellFormat(42, 6, "Автомобиль:", "", 0, "L", false, 0, "")
		pdf.SetFont("Go", "", 9)
		pdf.CellFormat(95, 6, data.VehicleName, "B", 0, "L", false, 0, "")
		pdf.SetFont("Go", "B", 9)
		pdf.CellFormat(32, 6, "Госномер:", "", 0, "L", false, 0, "")
		pdf.SetFont("Go", "", 9)
		pdf.CellFormat(0, 6, data.Registration, "B", 1, "L", false, 0, "")
		pdf.SetFont("Go", "B", 9)
		pdf.CellFormat(42, 6, "Водитель:", "", 0, "L", false, 0, "")
		pdf.SetFont("Go", "", 9)
		pdf.CellFormat(95, 6, data.DriverName, "B", 0, "L", false, 0, "")
		pdf.SetFont("Go", "B", 9)
		pdf.CellFormat(32, 6, "Составлено в:", "", 0, "L", false, 0, "")
		pdf.SetFont("Go", "", 8)
		pdf.CellFormat(0, 6, "Республике Беларусь", "B", 1, "L", false, 0, "")
		pdf.SetFont("Go", "B", 9)
		pdf.CellFormat(42, 6, "Основание:", "", 0, "L", false, 0, "")
		pdf.SetFont("Go", "", 8)
		pdf.CellFormat(0, 6, "учет использования транспортного средства, пробега, топлива и технического обслуживания", "B", 1, "L", false, 0, "")
		pdf.Ln(4)
	}

	columnWidths := []float64{22, 34, 28, 25, 25, 28, 95}
	headings := []string{"Дата", "Операция", "Одометр, км", "Пробег, км", "Топливо, л", "Сумма, руб.", "Описание / маршрут / основание"}
	addTableHeader := func() {
		pdf.SetFillColor(225, 232, 226)
		pdf.SetDrawColor(145, 154, 150)
		pdf.SetTextColor(25, 35, 32)
		pdf.SetFont("Go", "B", 7.5)
		for index, heading := range headings {
			pdf.CellFormat(columnWidths[index], 9, heading, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)
	}

	pdf.AddPage()
	addDocumentHeader()
	addTableHeader()
	rows := data.Rows
	if len(rows) == 0 {
		rows = []vehicleWaybillRow{{Description: "За выбранный месяц записи отсутствуют"}}
	}
	pdf.SetFont("Go", "", 8)
	for _, row := range rows {
		if pdf.GetY() > 183 {
			pdf.AddPage()
			addDocumentHeader()
			addTableHeader()
			pdf.SetFont("Go", "", 8)
		}
		values := []string{row.Date, row.RecordType, row.Mileage, row.MileageDiff, row.Liters, row.Amount, truncateWaybillText(row.Description, 72)}
		alignments := []string{"C", "L", "R", "R", "R", "R", "L"}
		for index, value := range values {
			pdf.CellFormat(columnWidths[index], 8, value, "1", 0, alignments[index], false, 0, "")
		}
		pdf.Ln(-1)
	}

	if pdf.GetY() > 155 {
		pdf.AddPage()
		addDocumentHeader()
	}
	pdf.Ln(5)
	pdf.SetFillColor(242, 245, 243)
	pdf.SetFont("Go", "B", 8)
	summaryLabels := []string{"Одометр на начало", "Одометр на конец", "Пробег за месяц", "Заправлено топлива", "Затраты на топливо"}
	summaryValues := []string{
		strconv.Itoa(data.OpeningMileage) + " км",
		strconv.Itoa(data.ClosingMileage) + " км",
		strconv.Itoa(data.TotalMileage) + " км",
		formatWaybillTotal(data.TotalLiters) + " л",
		formatWaybillTotal(data.TotalAmount) + " руб.",
	}
	boxWidth := 55.4
	for index, label := range summaryLabels {
		pdf.CellFormat(boxWidth, 6, label, "LTR", 0, "C", true, 0, "")
		if index == len(summaryLabels)-1 {
			pdf.Ln(-1)
		}
	}
	pdf.SetFont("Go", "B", 11)
	for index, value := range summaryValues {
		pdf.CellFormat(boxWidth, 9, value, "LBR", 0, "C", true, 0, "")
		if index == len(summaryValues)-1 {
			pdf.Ln(-1)
		}
	}
	pdf.Ln(8)
	pdf.SetFont("Go", "", 8)
	pdf.CellFormat(92, 7, "Водитель: ____________________ / "+data.DriverName+" /", "", 0, "L", false, 0, "")
	pdf.CellFormat(92, 7, "Ответственный за выпуск: ____________________", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 7, "Бухгалтер: ____________________", "", 1, "L", false, 0, "")
	pdf.Ln(4)
	pdf.SetFont("Go", "", 7)
	pdf.SetTextColor(75, 82, 86)
	pdf.MultiCell(0, 4, "Самостоятельно разработанная форма первичного учетного документа с учетом основных реквизитов статьи 10 Закона Республики Беларусь от 12.07.2013 № 57-З «О бухгалтерском учете и отчетности». Для применения форма должна быть утверждена руководителем организации и подписана ответственными лицами.", "", "L", false)

	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func ExportVehicleWaybillPDF(c *gin.Context) {
	vehicle, err := storage.GetVehicleByID(c.Param("id"))
	if err != nil {
		redirectVehicleMessage(c, "/vehicles", "error", "Транспорт не найден.")
		return
	}
	if !canAccessVehicle(c, vehicle) {
		redirectVehicleMessage(c, "/vehicles", "error", "У вас нет доступа к этому транспорту.")
		return
	}
	month, err := parseVehicleWaybillMonth(c.Query("month"))
	if err != nil {
		redirectVehicleMessage(c, "/vehicles/"+vehicle.ID, "error", "Выберите корректный месяц для путевки.")
		return
	}
	records, err := storage.GetVehicleRecords(vehicle.ID)
	if err != nil {
		redirectVehicleMessage(c, "/vehicles/"+vehicle.ID, "error", "Не удалось загрузить записи транспорта.")
		return
	}
	userNames, _, _ := vehicleUserNames()
	driverName := userNames[vehicle.AssignedUserID]
	data := buildVehicleWaybillData(vehicle, driverName, records, month)
	pdfData, err := buildVehicleWaybillPDF(data, time.Now())
	if err != nil {
		redirectVehicleMessage(c, "/vehicles/"+vehicle.ID, "error", "Не удалось сформировать PDF.")
		return
	}

	filename := fmt.Sprintf("waybill-%s-%s.pdf", strings.ReplaceAll(vehicle.RegistrationNumber, " ", "-"), month.Format("2006-01"))
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Header("Cache-Control", "no-store")
	security.LogEvent("vehicle_waybill_exported", fmt.Sprintf("user=%s vehicle=%s month=%s", c.GetString("userName"), vehicle.ID, month.Format("2006-01")))
	c.Data(http.StatusOK, "application/pdf", pdfData)
}
