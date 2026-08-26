package api

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"project/internal/models"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

func vehicleRecordTypeLabel(recordType string) string {
	switch recordType {
	case "fuel":
		return "Заправка"
	case "maintenance":
		return "Техническое обслуживание"
	default:
		return "Пробег"
	}
}

func vehicleDateLabel(value string) string {
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return value
	}
	return date.Format("02.01.2006")
}

func vehicleUserNames() (map[string]string, []models.User, error) {
	users, err := storage.GetUsers()
	if err != nil {
		return nil, nil, err
	}
	sort.SliceStable(users, func(i, j int) bool {
		return strings.ToLower(users[i].Name) < strings.ToLower(users[j].Name)
	})
	names := make(map[string]string, len(users))
	for _, user := range users {
		names[user.ID] = user.Name
	}
	return names, users, nil
}

func canAccessVehicle(c *gin.Context, vehicle models.Vehicle) bool {
	return isAdmin(c) || vehicle.AssignedUserID == c.GetString("userID")
}

func VehiclesPage(c *gin.Context) {
	allVehicles, err := storage.GetVehicles()
	if err != nil {
		c.String(http.StatusInternalServerError, "Не удалось загрузить транспорт: %v", err)
		return
	}
	userNames, users, err := vehicleUserNames()
	if err != nil {
		c.String(http.StatusInternalServerError, "Не удалось загрузить пользователей: %v", err)
		return
	}

	userID := c.GetString("userID")
	admin := isAdmin(c)
	var assignedCards, availableCards strings.Builder
	assignedCount := 0
	unassignedCount := 0
	visibleTotal := 0
	needsMileage := 0
	currentMonth := time.Now().Format("2006-01")

	for _, vehicle := range allVehicles {
		if !admin && vehicle.AssignedUserID != "" && vehicle.AssignedUserID != userID {
			continue
		}
		visibleTotal++
		if vehicle.AssignedUserID == "" {
			unassignedCount++
		} else if !storage.HasMileageRecordForMonth(vehicle.ID, currentMonth) {
			needsMileage++
		}

		owner := "Свободно"
		statusClass := "warning"
		if vehicle.AssignedUserID != "" {
			owner = userNames[vehicle.AssignedUserID]
			if owner == "" {
				owner = "Неизвестный пользователь"
			}
			statusClass = "active"
			assignedCount++
		}
		card := fmt.Sprintf(`<article class="vehicle-card"><div class="vehicle-card-head"><div><span class="vehicle-card-kicker">Транспортное средство</span><h3>%s</h3></div><span class="status-badge %s">%s</span></div><div class="vehicle-registration">%s</div><div class="vehicle-card-footer"><span class="text-muted">%s</span>%s</div></article>`,
			template.HTMLEscapeString(vehicle.Name), template.HTMLEscapeString(statusClass), template.HTMLEscapeString(owner), template.HTMLEscapeString(vehicle.RegistrationNumber), template.HTMLEscapeString(owner), vehicleCardAction(c, vehicle))
		if vehicle.AssignedUserID == "" && !admin {
			availableCards.WriteString(card)
		} else {
			assignedCards.WriteString(card)
		}
	}

	if assignedCards.Len() == 0 {
		message := "Транспорт пока не добавлен."
		if !admin {
			message = "За вами пока не закреплен транспорт."
		}
		assignedCards.WriteString(`<div class="empty-state-inline"><strong>` + message + `</strong></div>`)
	}
	availableSection := ""
	if !admin && availableCards.Len() > 0 {
		availableSection = `<section class="vehicle-section"><div class="section-heading"><div><h2>Свободный транспорт</h2><p>Выберите ТС, которым пользуетесь сейчас.</p></div></div><div class="vehicle-grid">` + availableCards.String() + `</div></section>`
	}

	assignOptions := `<option value="">Без владельца</option>`
	if admin {
		for _, user := range users {
			assignOptions += `<option value="` + template.HTMLEscapeString(user.ID) + `">` + template.HTMLEscapeString(user.Name) + `</option>`
		}
	}
	assignmentField := ""
	submitLabel := "Добавить и привязать к себе"
	if admin {
		assignmentField = `<div class="form-group-edit"><label for="assigned_user_id">Владелец</label><select id="assigned_user_id" name="assigned_user_id">` + assignOptions + `</select></div>`
		submitLabel = "Добавить ТС"
	}

	alert := ""
	if needsMileage > 0 {
		alert = `<div class="dashboard-alert-item is-warning vehicle-mileage-alert"><strong>Нужно уточнить пробег</strong><p>Для транспорта без показания за текущий месяц добавьте запись типа «Пробег».</p></div>`
	}
	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover"><title>Авто</title><link rel="stylesheet" href="/static/css/style.css?v=9"></head><body>
{{SIDEBAR_HTML}}<main class="main-content vehicles-page"><div class="page-header"><div><h1>Авто</h1><p class="text-muted">Учет заправок, пробега и технического обслуживания.</p></div></div>
{{ALERT}}<div class="vehicle-summary"><div class="metric"><div class="label">Всего ТС</div><div class="value">{{TOTAL}}</div></div><div class="metric"><div class="label">Закреплено</div><div class="value">{{ASSIGNED}}</div></div><div class="metric"><div class="label">Свободно</div><div class="value">{{FREE}}</div></div></div>
<section class="vehicle-section"><div class="section-heading"><div><h2>{{LIST_TITLE}}</h2><p>{{LIST_SUBTITLE}}</p></div></div><div class="vehicle-grid">{{ASSIGNED_CARDS}}</div></section>
{{AVAILABLE_SECTION}}
<section class="card vehicle-create-panel"><div class="section-heading"><div><h2>Добавить транспорт</h2><p>Укажите понятное название и государственный номер.</p></div></div><form method="POST" action="/vehicles/new" class="vehicle-create-form">{{CSRF_FIELD}}<div class="form-group-edit"><label for="vehicle_name">Марка и модель</label><input id="vehicle_name" name="name" required placeholder="Например, Ford Transit"></div><div class="form-group-edit"><label for="registration_number">Госномер</label><input id="registration_number" name="registration_number" required placeholder="А123ВС-7"></div>{{ASSIGNMENT_FIELD}}<button class="btn btn-primary" type="submit">{{SUBMIT_LABEL}}</button></form></section>
</main></body></html>`
	listTitle := "Весь транспорт"
	listSubtitle := "Откройте ТС, чтобы добавить запись или изменить владельца."
	if !admin {
		listTitle = "Мой транспорт"
		listSubtitle = "Здесь отображаются только закрепленные за вами ТС."
	}
	replacements := map[string]string{
		"{{SIDEBAR_HTML}}": RenderSidebar(c, "vehicles"), "{{ALERT}}": alert,
		"{{TOTAL}}": strconv.Itoa(visibleTotal), "{{ASSIGNED}}": strconv.Itoa(assignedCount), "{{FREE}}": strconv.Itoa(unassignedCount),
		"{{LIST_TITLE}}": listTitle, "{{LIST_SUBTITLE}}": listSubtitle, "{{ASSIGNED_CARDS}}": assignedCards.String(), "{{AVAILABLE_SECTION}}": availableSection,
		"{{CSRF_FIELD}}": CSRFHiddenInput(c), "{{ASSIGNMENT_FIELD}}": assignmentField, "{{SUBMIT_LABEL}}": submitLabel,
	}
	for token, value := range replacements {
		page = strings.ReplaceAll(page, token, value)
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}

func vehicleCardAction(c *gin.Context, vehicle models.Vehicle) string {
	if vehicle.AssignedUserID == "" && !isAdmin(c) {
		return `<form method="POST" action="/vehicles/` + template.HTMLEscapeString(vehicle.ID) + `/assign-self">` + CSRFHiddenInput(c) + `<button class="btn btn-primary btn-compact" type="submit">Привязать к себе</button></form>`
	}
	return `<a class="btn btn-secondary btn-compact" href="/vehicles/` + template.HTMLEscapeString(vehicle.ID) + `">Открыть</a>`
}

func CreateVehicle(c *gin.Context) {
	userID := c.GetString("userID")
	assignedUserID := userID
	if isAdmin(c) {
		assignedUserID = strings.TrimSpace(c.PostForm("assigned_user_id"))
		if assignedUserID != "" {
			if _, err := storage.GetUserByID(assignedUserID); err != nil {
				c.String(http.StatusBadRequest, "Выбранный пользователь не найден")
				return
			}
		}
	}
	_, err := storage.CreateVehicle(models.Vehicle{
		Name: c.PostForm("name"), RegistrationNumber: c.PostForm("registration_number"), AssignedUserID: assignedUserID,
		CreatedByUserID: userID, CreatedByName: c.GetString("userName"),
	})
	if err != nil {
		c.String(http.StatusBadRequest, "Не удалось добавить транспорт: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/vehicles")
}

func AssignVehicleToSelf(c *gin.Context) {
	vehicle, err := storage.GetVehicleByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Транспорт не найден")
		return
	}
	userID := c.GetString("userID")
	if vehicle.AssignedUserID != "" && vehicle.AssignedUserID != userID {
		c.String(http.StatusConflict, "Транспорт уже закреплен за другим пользователем")
		return
	}
	if err := storage.AssignVehicle(vehicle.ID, userID); err != nil {
		c.String(http.StatusInternalServerError, "Не удалось привязать транспорт: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/vehicles/"+vehicle.ID)
}

func AssignVehicle(c *gin.Context) {
	if !isAdmin(c) {
		c.String(http.StatusForbidden, "Доступ запрещен")
		return
	}
	userID := strings.TrimSpace(c.PostForm("assigned_user_id"))
	if userID != "" {
		if _, err := storage.GetUserByID(userID); err != nil {
			c.String(http.StatusBadRequest, "Пользователь не найден")
			return
		}
	}
	if err := storage.AssignVehicle(c.Param("id"), userID); err != nil {
		c.String(http.StatusBadRequest, "Не удалось изменить владельца: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/vehicles/"+c.Param("id"))
}

func UnassignVehicle(c *gin.Context) {
	vehicle, err := storage.GetVehicleByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Транспорт не найден")
		return
	}
	if !isAdmin(c) && vehicle.AssignedUserID != c.GetString("userID") {
		c.String(http.StatusForbidden, "Доступ запрещен")
		return
	}
	if err := storage.AssignVehicle(vehicle.ID, ""); err != nil {
		c.String(http.StatusInternalServerError, "Не удалось снять привязку: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/vehicles")
}

func VehiclePage(c *gin.Context) {
	vehicle, err := storage.GetVehicleByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Транспорт не найден")
		return
	}
	if !canAccessVehicle(c, vehicle) {
		c.String(http.StatusForbidden, "Доступ запрещен")
		return
	}
	records, _ := storage.GetVehicleRecords(vehicle.ID)
	userNames, users, _ := vehicleUserNames()
	owner := "Не закреплен"
	if vehicle.AssignedUserID != "" {
		owner = userNames[vehicle.AssignedUserID]
		if owner == "" {
			owner = "Неизвестный пользователь"
		}
	}

	lastMileage := 0
	var recordsHTML strings.Builder
	for _, record := range records {
		if record.Mileage > lastMileage {
			lastMileage = record.Mileage
		}
		details := make([]string, 0, 3)
		if record.Mileage > 0 {
			details = append(details, fmt.Sprintf("%d км", record.Mileage))
		}
		if record.Liters > 0 {
			details = append(details, fmt.Sprintf("%.2f л", record.Liters))
		}
		if record.Amount > 0 {
			details = append(details, fmt.Sprintf("%.2f руб.", record.Amount))
		}
		summary := strings.Join(details, " · ")
		if summary == "" {
			summary = vehicleRecordTypeLabel(record.Type)
		}
		recordsHTML.WriteString(`<article class="vehicle-record"><div class="vehicle-record-date"><strong>` + template.HTMLEscapeString(vehicleDateLabel(record.Date)) + `</strong><span class="status-badge">` + template.HTMLEscapeString(vehicleRecordTypeLabel(record.Type)) + `</span></div><div><strong>` + template.HTMLEscapeString(summary) + `</strong><p>` + template.HTMLEscapeString(record.Notes) + `</p><small>` + template.HTMLEscapeString(record.CreatedByName) + `</small></div></article>`)
	}
	if recordsHTML.Len() == 0 {
		recordsHTML.WriteString(`<div class="empty-state-inline"><strong>Записей пока нет</strong><p>Добавьте первый пробег, заправку или техническое обслуживание.</p></div>`)
	}

	adminPanel := ""
	if isAdmin(c) {
		var options strings.Builder
		options.WriteString(`<option value="">Без владельца</option>`)
		for _, user := range users {
			selected := ""
			if user.ID == vehicle.AssignedUserID {
				selected = " selected"
			}
			options.WriteString(`<option value="` + template.HTMLEscapeString(user.ID) + `"` + selected + `>` + template.HTMLEscapeString(user.Name) + `</option>`)
		}
		adminPanel = `<section class="card vehicle-owner-panel"><div><h2>Владелец ТС</h2><p class="text-muted">Администратор может переназначить или освободить транспорт.</p></div><form method="POST" action="/vehicles/` + template.HTMLEscapeString(vehicle.ID) + `/assign">` + CSRFHiddenInput(c) + `<select name="assigned_user_id">` + options.String() + `</select><button class="btn btn-secondary" type="submit">Сохранить владельца</button></form></section>`
	}
	unassignButton := ""
	if vehicle.AssignedUserID != "" {
		unassignButton = `<form method="POST" action="/vehicles/` + template.HTMLEscapeString(vehicle.ID) + `/unassign">` + CSRFHiddenInput(c) + `<button class="btn btn-secondary btn-compact" type="submit">Снять привязку</button></form>`
	}
	alert := ""
	if vehicle.AssignedUserID != "" && !storage.HasMileageRecordForMonth(vehicle.ID, time.Now().Format("2006-01")) {
		alert = `<div class="dashboard-alert-item is-warning vehicle-mileage-alert"><strong>Нет пробега за текущий месяц</strong><p>Добавьте показание одометра. В начале месяца напоминание также приходит в Telegram.</p></div>`
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover"><title>{{NAME}}</title><link rel="stylesheet" href="/static/css/style.css?v=9"></head><body>
{{SIDEBAR_HTML}}<main class="main-content vehicle-detail-page"><a class="back-link" href="/vehicles">← Все авто</a><header class="vehicle-detail-hero"><div><span class="vehicle-card-kicker">Транспортное средство</span><h1>{{NAME}}</h1><div class="vehicle-registration">{{REGISTRATION}}</div></div><div class="vehicle-owner-summary"><span>Владелец</span><strong>{{OWNER}}</strong>{{UNASSIGN}}</div></header>
{{ALERT}}{{ADMIN_PANEL}}<div class="vehicle-detail-grid"><section class="card vehicle-record-form-panel"><div class="section-heading"><div><h2>Новая запись</h2><p>Пробег, заправка или техническое обслуживание.</p></div></div><form method="POST" action="/vehicles/{{ID}}/records" class="vehicle-record-form">{{CSRF_FIELD}}<div class="form-group-edit"><label for="record_type">Тип</label><select id="record_type" name="type" required><option value="mileage">Пробег</option><option value="fuel">Заправка</option><option value="maintenance">Техническое обслуживание</option></select></div><div class="form-group-edit"><label for="record_date">Дата</label><input id="record_date" name="date" type="date" value="{{TODAY}}" required></div><div class="form-group-edit" data-record-field="mileage"><label for="mileage">Пробег, км</label><input id="mileage" name="mileage" type="number" min="0" value="{{LAST_MILEAGE}}"></div><div class="form-group-edit" data-record-field="fuel"><label for="liters">Топливо, л</label><input id="liters" name="liters" type="number" min="0" step="0.01"></div><div class="form-group-edit" data-record-field="amount"><label for="amount">Сумма, руб.</label><input id="amount" name="amount" type="number" min="0" step="0.01"></div><div class="form-group-edit vehicle-record-notes" data-record-field="notes"><label for="notes">Описание / комментарий</label><textarea id="notes" name="notes" rows="3" placeholder="Что сделано, где заправились или другое уточнение"></textarea></div><button class="btn btn-primary" type="submit">Добавить запись</button></form></section><section class="card vehicle-history-panel"><div class="section-heading"><div><h2>История</h2><p>{{RECORD_COUNT}} записей</p></div></div><div class="vehicle-record-list">{{RECORDS}}</div></section></div>
</main><script>(function(){const type=document.getElementById('record_type');if(!type)return;const mileage=document.getElementById('mileage');const liters=document.getElementById('liters');const amount=document.getElementById('amount');const notes=document.getElementById('notes');function field(name){return document.querySelector('[data-record-field="'+name+'"]');}function toggle(name,visible){const el=field(name);if(el)el.hidden=!visible;}function sync(){const value=type.value;toggle('mileage',true);toggle('fuel',value==='fuel');toggle('amount',value!=='mileage');toggle('notes',true);mileage.required=value==='mileage';liters.required=value==='fuel';notes.required=value==='maintenance';if(value!=='fuel')liters.value='';if(value==='mileage')amount.value='';}type.addEventListener('change',sync);sync();})();</script></body></html>`
	replacements := map[string]string{
		"{{SIDEBAR_HTML}}": RenderSidebar(c, "vehicles"), "{{NAME}}": template.HTMLEscapeString(vehicle.Name), "{{REGISTRATION}}": template.HTMLEscapeString(vehicle.RegistrationNumber), "{{OWNER}}": template.HTMLEscapeString(owner),
		"{{UNASSIGN}}": unassignButton, "{{ALERT}}": alert, "{{ADMIN_PANEL}}": adminPanel, "{{ID}}": template.HTMLEscapeString(vehicle.ID), "{{CSRF_FIELD}}": CSRFHiddenInput(c),
		"{{TODAY}}": time.Now().Format("2006-01-02"), "{{LAST_MILEAGE}}": strconv.Itoa(lastMileage), "{{RECORD_COUNT}}": strconv.Itoa(len(records)), "{{RECORDS}}": recordsHTML.String(),
	}
	for token, value := range replacements {
		page = strings.ReplaceAll(page, token, value)
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}

func CreateVehicleRecord(c *gin.Context) {
	vehicle, err := storage.GetVehicleByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Транспорт не найден")
		return
	}
	if !canAccessVehicle(c, vehicle) {
		c.String(http.StatusForbidden, "Доступ запрещен")
		return
	}
	mileage, _ := strconv.Atoi(strings.TrimSpace(c.PostForm("mileage")))
	liters, _ := strconv.ParseFloat(strings.TrimSpace(c.PostForm("liters")), 64)
	amount, _ := strconv.ParseFloat(strings.TrimSpace(c.PostForm("amount")), 64)
	record := models.VehicleRecord{
		VehicleID: vehicle.ID, Type: c.PostForm("type"), Date: c.PostForm("date"), Mileage: mileage, Liters: liters, Amount: amount, Notes: c.PostForm("notes"),
		CreatedByID: c.GetString("userID"), CreatedByName: c.GetString("userName"),
	}
	if _, err := storage.CreateVehicleRecord(record); err != nil {
		c.String(http.StatusBadRequest, "Не удалось добавить запись: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/vehicles/"+vehicle.ID)
}
