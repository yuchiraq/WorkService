package api

import (
	"bytes"
	"html/template"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"project/internal/models"
	"project/internal/storage"
)

func vehicleReturnPath(raw, fallback string) string {
	u, err := url.Parse(raw)
	if err != nil || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.ContainsAny(raw, "\\\r\n") || u.IsAbs() || u.Host != "" || strings.HasPrefix(u.Path, "//") || strings.ContainsAny(u.Path, "\\\r\n") {
		return fallback
	}
	return u.String()
}

func NewVehicleRecordPage(c *gin.Context) {
	vehicle, err := storage.GetVehicleByID(c.Param("id"))
	if err != nil || !canAccessVehicle(c, vehicle) {
		c.JSON(404, gin.H{"error": "Транспорт недоступен. Возможно, привязка была изменена."})
		return
	}
	records, err := storage.GetVehicleRecords(vehicle.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Не удалось загрузить данные автомобиля"})
		return
	}
	mileage := 0
	for _, record := range records {
		if record.Mileage > mileage {
			mileage = record.Mileage
		}
	}
	var out bytes.Buffer
	data := struct {
		Vehicle       models.Vehicle
		Mileage       int
		Today, Return string
		CSRF, Sidebar template.HTML
		Modal         bool
	}{Vehicle: vehicle, Mileage: mileage, Today: time.Now().Format("2006-01-02"), Return: vehicleReturnPath(c.Query("return"), "/vehicles/"+vehicle.ID), CSRF: template.HTML(CSRFHiddenInput(c)), Modal: IsModalRequest(c)}
	if !data.Modal {
		data.Sidebar = template.HTML(RenderSidebar(c, "vehicles"))
	}
	if err = vehicleRecordFormTemplate.Execute(&out, data); err != nil {
		c.JSON(500, gin.H{"error": "Не удалось открыть форму"})
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Data(200, "text/html; charset=utf-8", out.Bytes())
}

var vehicleRecordFormTemplate = template.Must(template.New("vehicle-record-form").Parse(`<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Запись по авто</title><link rel="stylesheet" href="/static/css/style.css?v=20"></head><body>
{{.Sidebar}}<div class="{{if .Modal}}modal-form-layout{{else}}main-content{{end}}"><section class="vehicle-quick-form">
<header><h2>{{.Vehicle.Name}}</h2><p class="text-muted">{{.Vehicle.RegistrationNumber}}</p></header>
<form method="post" action="/vehicles/{{.Vehicle.ID}}/records" class="vehicle-record-form" data-vehicle-quick-form>
{{.CSRF}}<input type="hidden" name="return_to" value="{{.Return}}"><input type="hidden" name="quick" value="1">
<div class="form-group-edit"><label for="quick_record_type">Тип записи</label><select id="quick_record_type" name="type" required><option value="fuel">Заправка</option><option value="mileage">Пробег</option><option value="maintenance">ТО</option></select></div>
<div class="form-group-edit"><label for="quick_record_date">Дата</label><input id="quick_record_date" name="date" type="date" value="{{.Today}}" required></div>
<div class="form-group-edit"><label for="quick_mileage">Пробег, км</label><input id="quick_mileage" name="mileage" type="number" inputmode="numeric" min="0" step="1" value="{{.Mileage}}"></div>
<div class="form-group-edit" data-quick-fuel><label for="quick_liters">Топливо, л</label><input id="quick_liters" name="liters" type="number" inputmode="decimal" min="0.01" step="0.01" required></div>
<div class="form-group-edit" data-quick-fuel><label for="quick_amount">Сумма, руб.</label><input id="quick_amount" name="amount" type="number" inputmode="decimal" min="0" step="0.01"></div>
<div class="form-group-edit vehicle-record-notes"><label for="quick_notes">Примечание</label><textarea id="quick_notes" name="notes" rows="2" maxlength="2000"></textarea></div>
<div class="vehicle-quick-footer"><button type="submit" class="btn btn-primary">Добавить запись</button></div>
</form></section></div><script>(()=>{
const scope=document.currentScript.closest('#app-action-modal-content') || document;
const form=scope.querySelector('[data-vehicle-quick-form]');
const type=form.elements.type, notes=form.elements.notes;
const now=new Date(); form.elements.date.value=[now.getFullYear(),String(now.getMonth()+1).padStart(2,'0'),String(now.getDate()).padStart(2,'0')].join('-');
function sync(){ const fuel=type.value==='fuel'; form.querySelectorAll('[data-quick-fuel]').forEach(field=>{field.hidden=!fuel;field.querySelector('input').disabled=!fuel;});form.elements.liters.required=fuel;form.elements.mileage.required=type.value==='mileage';notes.required=type.value==='maintenance'; }
type.addEventListener('change',sync); sync();
})();</script></body></html>`))
