package api

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"project/internal/models"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

func formatWorkHours(startTime, endTime string, lunch int) string {
	start, err := time.Parse("15:04", startTime)
	if err != nil {
		return "-"
	}
	end, err := time.Parse("15:04", endTime)
	if err != nil {
		return "-"
	}
	minutes := int(end.Sub(start).Minutes()) - lunch
	if minutes < 0 {
		minutes = 0
	}
	return fmt.Sprintf("%.2f", float64(minutes)/60.0)
}

func buildWorkersMap() (map[string]string, error) {
	workers, err := storage.GetWorkers()
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(workers))
	for _, worker := range workers {
		result[worker.ID] = worker.Name
	}
	return result, nil
}

func buildObjectsMap() (map[string]string, error) {
	objects, err := storage.GetObjects()
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(objects))
	for _, object := range objects {
		result[object.ID] = object.Name
	}
	return result, nil
}

func joinMappedValues(ids []string, valuesMap map[string]string) string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, exists := valuesMap[id]; exists {
			names = append(names, template.HTMLEscapeString(name))
		}
	}
	if len(names) == 0 {
		return "—"
	}
	return strings.Join(names, ", ")
}

func TimesheetsPage(c *gin.Context) {
	entries, err := storage.GetTimesheets()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load timesheets: %v", err)
		return
	}

	workersMap, err := buildWorkersMap()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load workers: %v", err)
		return
	}
	objectsMap, err := buildObjectsMap()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load objects: %v", err)
		return
	}

	var rows strings.Builder
	for _, entry := range entries {
		rows.WriteString(fmt.Sprintf(`<tr>
<td>%s</td>
<td>%s–%s</td>
<td>%d мин</td>
<td>%s</td>
<td>%s</td>
<td>%s</td>
<td><a href="/timesheets/edit/%s" class="btn btn-secondary">Редактировать</a></td>
</tr>`,
			template.HTMLEscapeString(entry.Date),
			template.HTMLEscapeString(entry.StartTime),
			template.HTMLEscapeString(entry.EndTime),
			entry.LunchBreakMinutes,
			template.HTMLEscapeString(formatWorkHours(entry.StartTime, entry.EndTime, entry.LunchBreakMinutes)),
			joinMappedValues(entry.WorkerIDs, workersMap),
			joinMappedValues(entry.ObjectIDs, objectsMap),
			template.HTMLEscapeString(entry.ID),
		))
	}

	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="7">Записей табеля пока нет.</td></tr>`)
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><title>Табель работ</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{SIDEBAR_HTML}}
<div class="main-content">
<div class="page-header"><h1>Табель работ</h1><a class="btn btn-primary" href="/timesheets/new">Добавить назначение</a></div>
<div class="card"><table class="table"><thead><tr><th>Дата</th><th>Смена</th><th>Обед</th><th>Часы</th><th>Работники</th><th>Объекты</th><th>Действия</th></tr></thead><tbody>{{ROWS}}</tbody></table></div>
</div>
</body></html>`

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "timesheets"), 1)
	final = strings.Replace(final, "{{ROWS}}", rows.String(), 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

func renderTimesheetForm(c *gin.Context, entry models.TimesheetEntry, actionURL, title, submit string, isEdit bool) {
	workers, err := storage.GetWorkers()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load workers: %v", err)
		return
	}
	objects, err := storage.GetObjects()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load objects: %v", err)
		return
	}

	selectedWorkers := make(map[string]struct{}, len(entry.WorkerIDs))
	for _, id := range entry.WorkerIDs {
		selectedWorkers[id] = struct{}{}
	}
	selectedObjects := make(map[string]struct{}, len(entry.ObjectIDs))
	for _, id := range entry.ObjectIDs {
		selectedObjects[id] = struct{}{}
	}

	var workerChecks strings.Builder
	for _, worker := range workers {
		checked := ""
		if _, exists := selectedWorkers[worker.ID]; exists {
			checked = " checked"
		}
		workerChecks.WriteString(fmt.Sprintf(`<label class="check-chip"><input type="checkbox" name="worker_ids" value="%s"%s><span>%s</span></label>`, template.HTMLEscapeString(worker.ID), checked, template.HTMLEscapeString(worker.Name)))
	}

	var objectChecks strings.Builder
	for _, object := range objects {
		checked := ""
		if _, exists := selectedObjects[object.ID]; exists {
			checked = " checked"
		}
		objectChecks.WriteString(fmt.Sprintf(`<label class="check-chip"><input type="checkbox" name="object_ids" value="%s"%s><span>%s</span></label>`, template.HTMLEscapeString(object.ID), checked, template.HTMLEscapeString(object.Name)))
	}

	if entry.Date == "" {
		entry.Date = time.Now().Format("2006-01-02")
	}
	if entry.StartTime == "" {
		entry.StartTime = "08:00"
	}
	if entry.EndTime == "" {
		entry.EndTime = "17:00"
	}
	if entry.LunchBreakMinutes == 0 {
		entry.LunchBreakMinutes = 60
	}

	l30, l60, l90 := "", "", ""
	switch entry.LunchBreakMinutes {
	case 30:
		l30 = " selected"
	case 90:
		l90 = " selected"
	default:
		l60 = " selected"
	}

	deleteBtn := ""
	if isEdit {
		deleteBtn = `<button type="button" class="btn btn-danger" onclick="showDeleteModal()">Удалить</button>`
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><title>{{TITLE}}</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{SIDEBAR_HTML}}
<div class="main-content">
<a href="/timesheets" class="back-link">← К табелю</a>
<div class="page-header"><h1>{{TITLE}}</h1></div>
<div class="card">
<form action="{{ACTION_URL}}" method="POST" class="form-grid-edit timesheet-form">
<div class="form-group-edit form-group-name"><label for="date">Дата</label><input id="date" name="date" type="date" value="{{DATE}}" required></div>
<div class="form-group-edit form-group-position"><label for="start_time">Начало смены</label><input id="start_time" name="start_time" type="time" value="{{START_TIME}}" required></div>
<div class="form-group-edit form-group-phone"><label for="end_time">Окончание смены</label><input id="end_time" name="end_time" type="time" value="{{END_TIME}}" required></div>
<div class="form-group-edit form-group-rate"><label for="lunch_break_minutes">Обед</label><select id="lunch_break_minutes" name="lunch_break_minutes" required><option value="30"{{L30}}>30 минут</option><option value="60"{{L60}}>60 минут</option><option value="90"{{L90}}>90 минут</option></select></div>
<div class="form-group-edit timesheet-span-2"><label>Работники</label><div class="chips-grid">{{WORKER_CHECKS}}</div></div>
<div class="form-group-edit timesheet-span-2"><label>Объекты</label><div class="chips-grid">{{OBJECT_CHECKS}}</div></div>
<div class="form-group-edit timesheet-span-2"><label for="notes">Комментарий</label><input id="notes" name="notes" type="text" value="{{NOTES}}" placeholder="Комментарий к смене"></div>
<div class="form-actions-edit"><button class="btn btn-primary" type="submit">{{SUBMIT}}</button><a href="/timesheets" class="btn btn-secondary">Отмена</a>{{DELETE_BUTTON}}</div>
</form>
</div>
</div>

<div id="deleteModal" class="modal" style="display:none;"><div class="modal-content"><span class="close-button" onclick="closeDeleteModal()">&times;</span><h2>Удалить назначение?</h2><p>Действие нельзя отменить.</p><form action="/timesheets/delete/{{ID}}" method="POST"><div class="form-actions"><button class="btn btn-danger" type="submit">Удалить</button><button class="btn btn-secondary" type="button" onclick="closeDeleteModal()">Отмена</button></div></form></div></div>
<script>function showDeleteModal(){document.getElementById('deleteModal').style.display='block';}function closeDeleteModal(){document.getElementById('deleteModal').style.display='none';}</script>
</body></html>`

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "timesheets"), 1)
	final = strings.Replace(final, "{{TITLE}}", template.HTMLEscapeString(title), -1)
	final = strings.Replace(final, "{{ACTION_URL}}", template.HTMLEscapeString(actionURL), 1)
	final = strings.Replace(final, "{{DATE}}", template.HTMLEscapeString(entry.Date), 1)
	final = strings.Replace(final, "{{START_TIME}}", template.HTMLEscapeString(entry.StartTime), 1)
	final = strings.Replace(final, "{{END_TIME}}", template.HTMLEscapeString(entry.EndTime), 1)
	final = strings.Replace(final, "{{L30}}", l30, 1)
	final = strings.Replace(final, "{{L60}}", l60, 1)
	final = strings.Replace(final, "{{L90}}", l90, 1)
	final = strings.Replace(final, "{{WORKER_CHECKS}}", workerChecks.String(), 1)
	final = strings.Replace(final, "{{OBJECT_CHECKS}}", objectChecks.String(), 1)
	final = strings.Replace(final, "{{NOTES}}", template.HTMLEscapeString(entry.Notes), 1)
	final = strings.Replace(final, "{{SUBMIT}}", template.HTMLEscapeString(submit), 1)
	final = strings.Replace(final, "{{DELETE_BUTTON}}", deleteBtn, 1)
	final = strings.Replace(final, "{{ID}}", template.HTMLEscapeString(entry.ID), 1)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

func AddTimesheetPage(c *gin.Context) {
	renderTimesheetForm(c, models.TimesheetEntry{Date: time.Now().Format("2006-01-02"), StartTime: "08:00", EndTime: "17:00", LunchBreakMinutes: 60}, "/timesheets/new", "Новое назначение", "Сохранить", false)
}

func CreateTimesheet(c *gin.Context) {
	lunch, _ := strconv.Atoi(c.PostForm("lunch_break_minutes"))
	entry := models.TimesheetEntry{
		Date:              c.PostForm("date"),
		StartTime:         c.PostForm("start_time"),
		EndTime:           c.PostForm("end_time"),
		LunchBreakMinutes: lunch,
		WorkerIDs:         c.PostFormArray("worker_ids"),
		ObjectIDs:         c.PostFormArray("object_ids"),
		Notes:             c.PostForm("notes"),
	}
	if _, err := storage.CreateTimesheet(entry); err != nil {
		c.String(http.StatusBadRequest, "Failed to create timesheet entry: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/timesheets")
}

func EditTimesheetPage(c *gin.Context) {
	entry, err := storage.GetTimesheetByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Timesheet entry not found")
		return
	}
	renderTimesheetForm(c, entry, "/timesheets/edit/"+entry.ID, "Редактирование назначения", "Сохранить изменения", true)
}

func UpdateTimesheet(c *gin.Context) {
	entry, err := storage.GetTimesheetByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Timesheet entry not found")
		return
	}
	lunch, _ := strconv.Atoi(c.PostForm("lunch_break_minutes"))
	entry.Date = c.PostForm("date")
	entry.StartTime = c.PostForm("start_time")
	entry.EndTime = c.PostForm("end_time")
	entry.LunchBreakMinutes = lunch
	entry.WorkerIDs = c.PostFormArray("worker_ids")
	entry.ObjectIDs = c.PostFormArray("object_ids")
	entry.Notes = c.PostForm("notes")

	if err := storage.UpdateTimesheet(entry); err != nil {
		c.String(http.StatusBadRequest, "Failed to update timesheet entry: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/timesheets")
}

func DeleteTimesheet(c *gin.Context) {
	if err := storage.DeleteTimesheet(c.Param("id")); err != nil {
		c.String(http.StatusBadRequest, "Failed to delete timesheet entry: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/timesheets")
}
