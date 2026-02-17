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

// SchedulePage shows assignment entries list (old "Табель" page renamed to "Расписание").
func SchedulePage(c *gin.Context) {
	entries, err := storage.GetTimesheets()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load schedule entries: %v", err)
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

	var cards strings.Builder
	for _, entry := range entries {
		cards.WriteString(fmt.Sprintf(`<div class="info-card">
            <div class="info-card-header">
                <h3>%s</h3>
                <span class="status-badge active">%s–%s</span>
            </div>
            <p><strong>Обед:</strong> %d мин</p>
            <p><strong>Часы:</strong> %s</p>
            <p><strong>Работники:</strong> %s</p>
            <p><strong>Объекты:</strong> %s</p>
            <p><strong>Комментарий:</strong> %s</p>
            <div class="info-card-actions"><a href="/schedule/edit/%s" class="btn btn-secondary">Редактировать</a></div>
        </div>`,
			template.HTMLEscapeString(entry.Date),
			template.HTMLEscapeString(entry.StartTime),
			template.HTMLEscapeString(entry.EndTime),
			entry.LunchBreakMinutes,
			template.HTMLEscapeString(formatWorkHours(entry.StartTime, entry.EndTime, entry.LunchBreakMinutes)),
			joinMappedValues(entry.WorkerIDs, workersMap),
			joinMappedValues(entry.ObjectIDs, objectsMap),
			template.HTMLEscapeString(entry.Notes),
			template.HTMLEscapeString(entry.ID),
		))
	}
	if cards.Len() == 0 {
		cards.WriteString(`<div class="info-card"><p>Записей расписания пока нет.</p></div>`)
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><title>Расписание</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{SIDEBAR_HTML}}
<div class="main-content">
<div class="page-header"><h1>Расписание</h1><a class="btn btn-primary" href="/schedule/new">Добавить назначение</a></div>
<div class="card"><div class="compact-grid">{{CARDS}}</div></div>
</div>
</body></html>`

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "schedule"), 1)
	final = strings.Replace(final, "{{CARDS}}", cards.String(), 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

func buildSelectAndSelectedList(items [][2]string, selectedIDs []string, selectID, inputName string) (string, string) {
	selectedSet := make(map[string]struct{}, len(selectedIDs))
	for _, id := range selectedIDs {
		selectedSet[id] = struct{}{}
	}

	var options strings.Builder
	for _, item := range items {
		options.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, template.HTMLEscapeString(item[0]), template.HTMLEscapeString(item[1])))
	}

	var selectedList strings.Builder
	for _, item := range items {
		if _, ok := selectedSet[item[0]]; !ok {
			continue
		}
		selectedList.WriteString(fmt.Sprintf(`<li data-id="%s"><span>%s</span><input type="hidden" name="%s" value="%s"><button type="button" class="btn btn-secondary btn-mini" onclick="removeSelection(this)">Удалить</button></li>`,
			template.HTMLEscapeString(item[0]),
			template.HTMLEscapeString(item[1]),
			template.HTMLEscapeString(inputName),
			template.HTMLEscapeString(item[0]),
		))
	}

	if selectedList.Len() == 0 {
		selectedList.WriteString(`<li class="empty">Ничего не выбрано</li>`)
	}

	return options.String(), selectedList.String()
}

func renderScheduleForm(c *gin.Context, entry models.TimesheetEntry, actionURL, title, submit string, isEdit bool) {
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

	workerItems := make([][2]string, 0, len(workers))
	for _, worker := range workers {
		workerItems = append(workerItems, [2]string{worker.ID, worker.Name})
	}
	objectItems := make([][2]string, 0, len(objects))
	for _, object := range objects {
		objectItems = append(objectItems, [2]string{object.ID, object.Name})
	}

	workerOptions, workerSelected := buildSelectAndSelectedList(workerItems, entry.WorkerIDs, "worker_select", "worker_ids")
	objectOptions, objectSelected := buildSelectAndSelectedList(objectItems, entry.ObjectIDs, "object_select", "object_ids")

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
<a href="/schedule" class="back-link">← К расписанию</a>
<div class="page-header"><h1>{{TITLE}}</h1></div>
<div class="card">
<form action="{{ACTION_URL}}" method="POST" class="form-grid-edit timesheet-form">
<div class="form-group-edit"><label for="date">Дата</label><input id="date" name="date" type="date" value="{{DATE}}" required></div>
<div class="form-group-edit"><label for="start_time">Начало смены</label><input id="start_time" name="start_time" type="time" value="{{START_TIME}}" required></div>
<div class="form-group-edit"><label for="end_time">Окончание смены</label><input id="end_time" name="end_time" type="time" value="{{END_TIME}}" required></div>
<div class="form-group-edit"><label for="lunch_break_minutes">Обед</label><select id="lunch_break_minutes" name="lunch_break_minutes" required><option value="30"{{L30}}>30 минут</option><option value="60"{{L60}}>60 минут</option><option value="90"{{L90}}>90 минут</option></select></div>

<div class="form-group-edit timesheet-span-2">
  <label>Работники</label>
  <div class="picker-row"><select id="worker_select">{{WORKER_OPTIONS}}</select><button class="btn btn-primary" type="button" onclick="addSelection('worker_select','worker_selected','worker_ids')">Добавить работника</button></div>
  <ul id="worker_selected" class="selected-list">{{WORKER_SELECTED}}</ul>
</div>

<div class="form-group-edit timesheet-span-2">
  <label>Объекты</label>
  <div class="picker-row"><select id="object_select">{{OBJECT_OPTIONS}}</select><button class="btn btn-primary" type="button" onclick="addSelection('object_select','object_selected','object_ids')">Добавить объект</button></div>
  <ul id="object_selected" class="selected-list">{{OBJECT_SELECTED}}</ul>
</div>

<div class="form-group-edit timesheet-span-2"><label for="notes">Комментарий</label><input id="notes" name="notes" type="text" value="{{NOTES}}" placeholder="Комментарий к смене"></div>
<div class="form-actions-edit"><button class="btn btn-primary" type="submit">{{SUBMIT}}</button><a href="/schedule" class="btn btn-secondary">Отмена</a>{{DELETE_BUTTON}}</div>
</form>
</div>
</div>

<div id="deleteModal" class="modal" style="display:none;"><div class="modal-content"><span class="close-button" onclick="closeDeleteModal()">&times;</span><h2>Удалить назначение?</h2><p>Действие нельзя отменить.</p><form action="/schedule/delete/{{ID}}" method="POST"><div class="form-actions"><button class="btn btn-danger" type="submit">Удалить</button><button class="btn btn-secondary" type="button" onclick="closeDeleteModal()">Отмена</button></div></form></div></div>
<script>
function removeSelection(button){
  const li = button.closest('li');
  const ul = li.parentElement;
  li.remove();
  if (!ul.querySelector('li')) { ul.innerHTML='<li class="empty">Ничего не выбрано</li>'; }
}
function addSelection(selectId, listId, inputName){
  const sel = document.getElementById(selectId);
  const value = sel.value;
  const text = sel.options[sel.selectedIndex].text;
  const list = document.getElementById(listId);
  if (!value) return;
  if (list.querySelector('li[data-id="'+value+'"]')) return;
  const empty = list.querySelector('li.empty');
  if (empty) empty.remove();
  const li = document.createElement('li');
  li.setAttribute('data-id', value);
  li.innerHTML = '<span>'+text+'</span><input type="hidden" name="'+inputName+'" value="'+value+'"><button type="button" class="btn btn-secondary btn-mini" onclick="removeSelection(this)">Удалить</button>';
  list.appendChild(li);
}
function showDeleteModal(){document.getElementById('deleteModal').style.display='block';}
function closeDeleteModal(){document.getElementById('deleteModal').style.display='none';}
</script>
</body></html>`

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "schedule"), 1)
	final = strings.Replace(final, "{{TITLE}}", template.HTMLEscapeString(title), -1)
	final = strings.Replace(final, "{{ACTION_URL}}", template.HTMLEscapeString(actionURL), 1)
	final = strings.Replace(final, "{{DATE}}", template.HTMLEscapeString(entry.Date), 1)
	final = strings.Replace(final, "{{START_TIME}}", template.HTMLEscapeString(entry.StartTime), 1)
	final = strings.Replace(final, "{{END_TIME}}", template.HTMLEscapeString(entry.EndTime), 1)
	final = strings.Replace(final, "{{L30}}", l30, 1)
	final = strings.Replace(final, "{{L60}}", l60, 1)
	final = strings.Replace(final, "{{L90}}", l90, 1)
	final = strings.Replace(final, "{{WORKER_OPTIONS}}", workerOptions, 1)
	final = strings.Replace(final, "{{WORKER_SELECTED}}", workerSelected, 1)
	final = strings.Replace(final, "{{OBJECT_OPTIONS}}", objectOptions, 1)
	final = strings.Replace(final, "{{OBJECT_SELECTED}}", objectSelected, 1)
	final = strings.Replace(final, "{{NOTES}}", template.HTMLEscapeString(entry.Notes), 1)
	final = strings.Replace(final, "{{SUBMIT}}", template.HTMLEscapeString(submit), 1)
	final = strings.Replace(final, "{{DELETE_BUTTON}}", deleteBtn, 1)
	final = strings.Replace(final, "{{ID}}", template.HTMLEscapeString(entry.ID), 1)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

func AddSchedulePage(c *gin.Context) {
	renderScheduleForm(c, models.TimesheetEntry{Date: time.Now().Format("2006-01-02"), StartTime: "08:00", EndTime: "17:00", LunchBreakMinutes: 60}, "/schedule/new", "Новое назначение", "Сохранить", false)
}

func CreateScheduleEntry(c *gin.Context) {
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
		c.String(http.StatusBadRequest, "Failed to create schedule entry: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/schedule")
}

func EditSchedulePage(c *gin.Context) {
	entry, err := storage.GetTimesheetByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Schedule entry not found")
		return
	}
	renderScheduleForm(c, entry, "/schedule/edit/"+entry.ID, "Редактирование назначения", "Сохранить изменения", true)
}

func UpdateScheduleEntry(c *gin.Context) {
	entry, err := storage.GetTimesheetByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Schedule entry not found")
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
		c.String(http.StatusBadRequest, "Failed to update schedule entry: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/schedule")
}

func DeleteScheduleEntry(c *gin.Context) {
	if err := storage.DeleteTimesheet(c.Param("id")); err != nil {
		c.String(http.StatusBadRequest, "Failed to delete schedule entry: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/schedule")
}

// TimesheetsPage is new табель matrix by workers/dates with per-cell hover details.
func TimesheetsPage(c *gin.Context) {
	entries, err := storage.GetTimesheets()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load timesheets: %v", err)
		return
	}
	workers, err := storage.GetWorkers()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load workers: %v", err)
		return
	}
	objectsMap, err := buildObjectsMap()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load objects: %v", err)
		return
	}

	selectedMonth := c.Query("month")
	if selectedMonth == "" {
		selectedMonth = time.Now().Format("2006-01")
	}
	monthStart, err := time.Parse("2006-01", selectedMonth)
	if err != nil {
		monthStart = time.Now()
		selectedMonth = monthStart.Format("2006-01")
	}
	monthStart = time.Date(monthStart.Year(), monthStart.Month(), 1, 0, 0, 0, 0, time.UTC)
	nextMonth := monthStart.AddDate(0, 1, 0)
	daysInMonth := int(nextMonth.Sub(monthStart).Hours() / 24)

	monthDates := make([]string, 0, daysInMonth)
	for d := 1; d <= daysInMonth; d++ {
		monthDates = append(monthDates, fmt.Sprintf("%s-%02d", selectedMonth, d))
	}

	var headers strings.Builder
	for d := 1; d <= daysInMonth; d++ {
		headers.WriteString(fmt.Sprintf(`<th>%d</th>`, d))
	}

	workerRows := make([]string, 0, len(workers))
	for _, worker := range workers {
		var cells strings.Builder
		for _, date := range monthDates {
			total := 0.0
			details := make([]string, 0)
			for _, entry := range entries {
				if entry.Date != date {
					continue
				}
				contains := false
				for _, wid := range entry.WorkerIDs {
					if wid == worker.ID {
						contains = true
						break
					}
				}
				if !contains {
					continue
				}
				hoursStr := formatWorkHours(entry.StartTime, entry.EndTime, entry.LunchBreakMinutes)
				hours, _ := strconv.ParseFloat(hoursStr, 64)
				total += hours
				objects := joinMappedValues(entry.ObjectIDs, objectsMap)
				details = append(details, fmt.Sprintf("%s-%s · %s ч · %s · %s", entry.StartTime, entry.EndTime, hoursStr, objects, template.HTMLEscapeString(entry.Notes)))
			}
			if len(details) == 0 {
				cells.WriteString(`<td class="hours-cell empty">—</td>`)
				continue
			}
			cells.WriteString(fmt.Sprintf(`<td class="hours-cell"><span>%.1f</span><div class="hours-tooltip">%s</div></td>`, total, strings.Join(details, "<br>")))
		}
		workerRows = append(workerRows, fmt.Sprintf(`<tr><th>%s</th>%s</tr>`, template.HTMLEscapeString(worker.Name), cells.String()))
	}

	rows := strings.Join(workerRows, "")
	if rows == "" {
		rows = `<tr><td colspan="100%">Нет работников.</td></tr>`
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><title>Табель</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{SIDEBAR_HTML}}
<div class="main-content">
<div class="page-header"><h1>Табель</h1><a class="btn btn-secondary" href="/schedule">К расписанию</a></div>
<div class="card">
  <form method="GET" action="/timesheets" class="month-selector">
    <label for="month">Месяц:</label>
    <input type="month" id="month" name="month" value="{{MONTH}}">
    <button type="submit" class="btn btn-primary">Показать</button>
  </form>
  <div class="table-scroll"><table class="table timesheet-matrix"><thead><tr><th>Работник</th>{{HEADERS}}</tr></thead><tbody>{{ROWS}}</tbody></table></div>
</div>
</div></body></html>`

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "timesheets"), 1)
	final = strings.Replace(final, "{{MONTH}}", template.HTMLEscapeString(selectedMonth), 1)
	final = strings.Replace(final, "{{HEADERS}}", headers.String(), 1)
	final = strings.Replace(final, "{{ROWS}}", rows, 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}
