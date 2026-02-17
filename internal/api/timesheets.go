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

func joinMappedLinks(ids []string, valuesMap map[string]string, pathPrefix string) string {
	items := make([]string, 0, len(ids))
	for _, id := range ids {
		name, exists := valuesMap[id]
		if !exists {
			continue
		}
		items = append(items, fmt.Sprintf(`<a class="entity-link" href="%s/%s">%s</a>`, template.HTMLEscapeString(pathPrefix), template.HTMLEscapeString(id), template.HTMLEscapeString(name)))
	}
	if len(items) == 0 {
		return "—"
	}
	return strings.Join(items, ", ")
}

// SchedulePage shows assignment entries list (old "Табель" page renamed to "Расписание").

func cleanIDList(ids []string) []string {
	clean := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		clean = append(clean, id)
	}
	return clean
}

func getScopedEntries(c *gin.Context, entries []models.TimesheetEntry) ([]models.TimesheetEntry, error) {
	if c.GetString("userStatus") == "admin" {
		return entries, nil
	}
	worker, err := storage.GetWorkerByUserID(c.GetString("userID"))
	if err != nil {
		return []models.TimesheetEntry{}, nil
	}
	filtered := make([]models.TimesheetEntry, 0)
	for _, entry := range entries {
		for _, wid := range entry.WorkerIDs {
			if wid == worker.ID {
				filtered = append(filtered, entry)
				break
			}
		}
	}
	return filtered, nil
}

func monthOptionsHTML(selectedMonth string) string {
	monthNames := []string{"Январь", "Февраль", "Март", "Апрель", "Май", "Июнь", "Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь"}
	now := time.Now()
	var b strings.Builder
	for i := -12; i <= 12; i++ {
		m := now.AddDate(0, i, 0)
		value := m.Format("2006-01")
		selected := ""
		if value == selectedMonth {
			selected = " selected"
		}
		label := fmt.Sprintf("%s %d", monthNames[int(m.Month())-1], m.Year())
		b.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, template.HTMLEscapeString(value), selected, template.HTMLEscapeString(label)))
	}
	return b.String()
}
func formatScheduleDateLabel(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return template.HTMLEscapeString(date)
	}
	weekdays := []string{"воскресенье", "понедельник", "вторник", "среда", "четверг", "пятница", "суббота"}
	months := []string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}
	return fmt.Sprintf("%s, %d %s %d", weekdays[int(t.Weekday())], t.Day(), months[int(t.Month())-1], t.Year())
}

func SchedulePage(c *gin.Context) {
	entries, err := storage.GetTimesheets()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load schedule entries: %v", err)
		return
	}

	entries, err = getScopedEntries(c, entries)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to scope schedule entries: %v", err)
		return
	}

	selectedMonth := c.Query("month")
	if selectedMonth == "" {
		selectedMonth = time.Now().Format("2006-01")
	}

	filteredEntries := make([]models.TimesheetEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Date, selectedMonth+"-") {
			filteredEntries = append(filteredEntries, entry)
		}
	}
	entries = filteredEntries

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

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date == entries[j].Date {
			return entries[i].StartTime < entries[j].StartTime
		}
		return entries[i].Date > entries[j].Date
	})

	var scheduleRows strings.Builder
	if len(entries) == 0 {
		scheduleRows.WriteString(`<div class="info-card"><p>Записей за выбранный месяц нет.</p></div>`)
	} else {
		currentDate := ""
		for _, entry := range entries {
			if entry.Date != currentDate {
				if currentDate != "" {
					scheduleRows.WriteString(`</div></div>`)
				}
				currentDate = entry.Date
				scheduleRows.WriteString(fmt.Sprintf(`<div class="schedule-day-group"><h3>%s</h3><div class="schedule-day-list">`, template.HTMLEscapeString(formatScheduleDateLabel(entry.Date))))
			}
			commentHTML := ""
			if strings.TrimSpace(entry.Notes) != "" {
				commentHTML = `<div class="assignment-note"><span>Комментарий</span><p>` + template.HTMLEscapeString(entry.Notes) + `</p></div>`
			}
			creatorHTML := ""
			if strings.TrimSpace(entry.CreatedByName) != "" {
				creatorHTML = `<div class="assignment-meta"><span>Создал</span><p>` + template.HTMLEscapeString(entry.CreatedByName) + `</p></div>`
			}
			scheduleRows.WriteString(fmt.Sprintf(`<article class="schedule-entry-vertical assignment-card"><div class="assignment-head"><strong>%s — %s</strong><span>%s ч</span></div><div class="assignment-body"><div class="assignment-meta"><span>Объекты</span><p>%s</p></div><div class="assignment-meta"><span>Работники</span><p>%s</p></div>%s%s</div><div class="info-card-actions"><a href="/schedule/edit/%s" class="btn btn-secondary" data-modal-url="/schedule/edit/%s" data-modal-title="Редактирование назначения" data-modal-return="/schedule">Редактировать</a></div></article>`,
				template.HTMLEscapeString(entry.StartTime),
				template.HTMLEscapeString(entry.EndTime),
				template.HTMLEscapeString(formatWorkHours(entry.StartTime, entry.EndTime, entry.LunchBreakMinutes)),
				joinMappedLinks(entry.ObjectIDs, objectsMap, "/object"),
				joinMappedLinks(entry.WorkerIDs, workersMap, "/worker"),
				creatorHTML,
				commentHTML,
				template.HTMLEscapeString(entry.ID),
				template.HTMLEscapeString(entry.ID),
			))
		}
		scheduleRows.WriteString(`</div></div>`)
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><title>Расписание</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{SIDEBAR_HTML}}
<div class="main-content">
<div class="page-header"><h1>Расписание</h1><form method="GET" action="/schedule" class="month-selector"><select id="month" name="month" onchange="this.form.submit()">{{MONTH_OPTIONS}}</select></form><a class="btn btn-primary" href="/schedule/new" data-modal-url="/schedule/new" data-modal-title="Новое назначение" data-modal-return="/schedule">Добавить назначение</a></div>
<div class="card"><div class="schedule-vertical">{{SCHEDULE_ROWS}}</div></div>
</div>
</body></html>`

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "schedule"), 1)
	final = strings.Replace(final, "{{MONTH_OPTIONS}}", monthOptionsHTML(selectedMonth), 1)
	final = strings.Replace(final, "{{SCHEDULE_ROWS}}", scheduleRows.String(), 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}

func buildSelectAndSelectedList(items [][2]string, selectedIDs []string, selectID, inputName string) (string, string) {
	_ = selectID
	var options strings.Builder
	options.WriteString(`<option value="">Выберите...</option>`)
	for _, item := range items {
		options.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, template.HTMLEscapeString(item[0]), template.HTMLEscapeString(item[1])))
	}

	var rows strings.Builder
	for _, selectedID := range selectedIDs {
		rows.WriteString(`<div class="dynamic-select-row"><select name="` + template.HTMLEscapeString(inputName) + `" class="dynamic-select">`)
		rows.WriteString(`<option value="">Выберите...</option>`)
		for _, item := range items {
			selected := ""
			if item[0] == selectedID {
				selected = " selected"
			}
			rows.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, template.HTMLEscapeString(item[0]), selected, template.HTMLEscapeString(item[1])))
		}
		rows.WriteString(`</select><button type="button" class="btn btn-secondary btn-mini" data-remove-select>✕</button></div>`)
	}
	rows.WriteString(`<div class="dynamic-select-row"><select name="` + template.HTMLEscapeString(inputName) + `" class="dynamic-select">` + options.String() + `</select><button type="button" class="btn btn-secondary btn-mini" data-remove-select>✕</button></div>`)

	return options.String(), rows.String()
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

	if c.GetString("userStatus") != "admin" {
		if ownWorker, err := storage.GetWorkerByUserID(c.GetString("userID")); err == nil && !ownWorker.IsFired {
			workers = []models.Worker{ownWorker}
		} else {
			workers = []models.Worker{}
		}
	}

	workerItems := make([][2]string, 0, len(workers))
	for _, worker := range workers {
		if worker.IsFired {
			continue
		}
		workerItems = append(workerItems, [2]string{worker.ID, worker.Name})
	}
	objectItems := make([][2]string, 0, len(objects))
	for _, object := range objects {
		if object.Status != "in_progress" {
			continue
		}
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

	l0, l30, l60, l90 := "", "", "", ""
	switch entry.LunchBreakMinutes {
	case 0:
		l0 = " selected"
	case 30:
		l30 = " selected"
	case 90:
		l90 = " selected"
	default:
		l60 = " selected"
	}

	deleteBtn := ""
	isModal := IsModalRequest(c)
	if isEdit {
		deleteBtn = `<button type="button" class="btn btn-danger" onclick="showDeleteModal()">Удалить</button>`
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><title>{{TITLE}}</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{LAYOUT_START}}
<div class="main-content{{MAIN_CONTENT_CLASS}}">
{{BACK_LINK}}
<div class="page-header"><h1>{{TITLE}}</h1></div>
<div class="card{{CARD_CLASS}}">
<form action="{{ACTION_URL}}" method="POST" class="form-grid-edit timesheet-form">
{{CSRF_FIELD}}
<input type="hidden" name="return_to" value="{{RETURN_TO}}">
<div class="form-group-edit"><label for="date">Дата</label><input id="date" name="date" type="date" value="{{DATE}}" required></div>
<div class="form-group-edit"><label for="start_time">Начало смены</label><input id="start_time" name="start_time" type="time" value="{{START_TIME}}" required></div>
<div class="form-group-edit"><label for="end_time">Окончание смены</label><input id="end_time" name="end_time" type="time" value="{{END_TIME}}" required></div>
<div class="form-group-edit"><label for="lunch_break_minutes">Обед</label><select id="lunch_break_minutes" name="lunch_break_minutes" required><option value="0"{{L0}}>Без обеда</option><option value="30"{{L30}}>30 минут</option><option value="60"{{L60}}>60 минут</option><option value="90"{{L90}}>90 минут</option></select></div>

<div class="form-group-edit timesheet-span-2">
  <label>Работники</label>
  <div class="dynamic-select-group" data-dynamic-select-group>
    {{WORKER_SELECTED}}
  </div>
</div>

<div class="form-group-edit timesheet-span-2">
  <label>Объекты</label>
  <div class="dynamic-select-group" data-dynamic-select-group>
    {{OBJECT_SELECTED}}
  </div>
</div>

<div class="form-group-edit timesheet-span-2"><label for="notes">Комментарий</label><input id="notes" name="notes" type="text" value="{{NOTES}}" placeholder="Комментарий к смене"></div>
<div class="form-actions-edit"><button class="btn btn-primary" type="submit">{{SUBMIT}}</button><a href="/schedule" class="btn btn-secondary">Отмена</a>{{DELETE_BUTTON}}</div>
</form>
</div>
</div>
{{LAYOUT_END}}

<div id="deleteModal" class="modal" style="display:none;"><div class="modal-content"><span class="close-button" onclick="closeDeleteModal()">&times;</span><h2>Удалить назначение?</h2><p>Действие нельзя отменить.</p><form action="/schedule/delete/{{ID}}" method="POST">{{CSRF_FIELD}}<div class="form-actions"><button class="btn btn-danger" type="submit">Удалить</button><button class="btn btn-secondary" type="button" onclick="closeDeleteModal()">Отмена</button></div></form></div></div>
<script>
function makeSelectRow(name, optionsHTML){
  const row=document.createElement('div');
  row.className='dynamic-select-row';
  row.innerHTML='<select name="'+name+'" class="dynamic-select">'+optionsHTML+'</select><button type="button" class="btn btn-secondary btn-mini" data-remove-select>✕</button>';
  return row;
}
function normalizeDynamicGroup(group){
  const rows=Array.from(group.querySelectorAll('.dynamic-select-row'));
  rows.forEach(function(row){
    const btn=row.querySelector('[data-remove-select]');
    if(btn){
      btn.onclick=function(){
        if(group.querySelectorAll('.dynamic-select-row').length===1){
          row.querySelector('select').value='';
          return;
        }
        row.remove();
        normalizeDynamicGroup(group);
      };
    }
  });
}
function ensureDynamicSelectRows(group){
  const rows=Array.from(group.querySelectorAll('.dynamic-select-row'));
  if(!rows.length) return;
  const last=rows[rows.length-1];
  const select=last.querySelector('select');
  if(select && select.value){
    group.appendChild(makeSelectRow(select.name, select.innerHTML));
  }
  normalizeDynamicGroup(group);
}
document.querySelectorAll('[data-dynamic-select-group]').forEach(function(group){
  group.addEventListener('change', function(e){
    if(e.target.matches('select')) ensureDynamicSelectRows(group);
  });
  ensureDynamicSelectRows(group);
});
function showDeleteModal(){document.getElementById('deleteModal').style.display='block';}
function closeDeleteModal(){document.getElementById('deleteModal').style.display='none';}
</script>
</body></html>`

	layoutStart := RenderSidebar(c, "schedule")
	layoutEnd := ""
	mainClass := ""
	backLink := `<a href="/schedule" class="back-link">← К расписанию</a>`
	cardClass := ""
	returnTo := c.DefaultQuery("return", "/schedule")
	if !strings.HasPrefix(returnTo, "/") {
		returnTo = "/schedule"
	}
	if isModal {
		layoutStart = `<div class="modal-form-layout">`
		layoutEnd = `</div>`
		mainClass = " modal-form-content"
		backLink = ""
		cardClass = " modal-form-card"
	}

	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "schedule"), 1)
	final = strings.Replace(final, "{{LAYOUT_START}}", layoutStart, 1)
	final = strings.Replace(final, "{{LAYOUT_END}}", layoutEnd, 1)
	final = strings.Replace(final, "{{MAIN_CONTENT_CLASS}}", mainClass, 1)
	final = strings.Replace(final, "{{BACK_LINK}}", backLink, 1)
	final = strings.Replace(final, "{{CARD_CLASS}}", cardClass, 1)
	final = strings.Replace(final, "{{TITLE}}", template.HTMLEscapeString(title), -1)
	final = strings.Replace(final, "{{ACTION_URL}}", template.HTMLEscapeString(actionURL), 1)
	final = strings.Replace(final, "{{CSRF_FIELD}}", CSRFHiddenInput(c), -1)
	final = strings.Replace(final, "{{RETURN_TO}}", template.HTMLEscapeString(returnTo), 1)
	final = strings.Replace(final, "{{DATE}}", template.HTMLEscapeString(entry.Date), 1)
	final = strings.Replace(final, "{{START_TIME}}", template.HTMLEscapeString(entry.StartTime), 1)
	final = strings.Replace(final, "{{END_TIME}}", template.HTMLEscapeString(entry.EndTime), 1)
	final = strings.Replace(final, "{{L0}}", l0, 1)
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

func validateScheduleLinks(workerIDs, objectIDs []string) error {
	workers, err := storage.GetWorkers()
	if err != nil {
		return err
	}
	workerMap := map[string]models.Worker{}
	for _, w := range workers {
		workerMap[w.ID] = w
	}
	for _, wid := range workerIDs {
		w, ok := workerMap[wid]
		if !ok || w.IsFired {
			return fmt.Errorf("нельзя назначить уволенного или несуществующего работника")
		}
	}

	objects, err := storage.GetObjects()
	if err != nil {
		return err
	}
	objMap := map[string]models.Object{}
	for _, o := range objects {
		objMap[o.ID] = o
	}
	for _, oid := range objectIDs {
		o, ok := objMap[oid]
		if !ok || o.Status != "in_progress" {
			return fmt.Errorf("нельзя назначить объект, который не в работе")
		}
	}
	return nil
}

func CreateScheduleEntry(c *gin.Context) {
	lunch, _ := strconv.Atoi(c.PostForm("lunch_break_minutes"))
	entry := models.TimesheetEntry{
		Date:              c.PostForm("date"),
		StartTime:         c.PostForm("start_time"),
		EndTime:           c.PostForm("end_time"),
		LunchBreakMinutes: lunch,
		WorkerIDs:         cleanIDList(c.PostFormArray("worker_ids")),
		ObjectIDs:         cleanIDList(c.PostFormArray("object_ids")),
		Notes:             c.PostForm("notes"),
		CreatedByID:       c.GetString("userID"),
		CreatedByName:     c.GetString("userName"),
	}

	if c.GetString("userStatus") != "admin" {
		if worker, err := storage.GetWorkerByUserID(c.GetString("userID")); err == nil {
			entry.WorkerIDs = []string{worker.ID}
		}
		entry.UserMark = "Создано пользователем " + c.GetString("userName")
	}
	if err := validateScheduleLinks(entry.WorkerIDs, entry.ObjectIDs); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	if _, err := storage.CreateTimesheet(entry); err != nil {
		c.String(http.StatusBadRequest, "Failed to create schedule entry: %v", err)
		return
	}
	returnTo := c.PostForm("return_to")
	if !strings.HasPrefix(returnTo, "/") {
		returnTo = "/schedule"
	}
	c.Redirect(http.StatusFound, returnTo)
}

func EditSchedulePage(c *gin.Context) {
	entry, err := storage.GetTimesheetByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Schedule entry not found")
		return
	}
	if c.GetString("userStatus") != "admin" {
		worker, err := storage.GetWorkerByUserID(c.GetString("userID"))
		if err != nil {
			c.String(http.StatusForbidden, "Нет привязанного работника")
			return
		}
		allowed := false
		for _, wid := range entry.WorkerIDs {
			if wid == worker.ID {
				allowed = true
				break
			}
		}
		if !allowed {
			c.String(http.StatusForbidden, "Доступ запрещен")
			return
		}
	}
	renderScheduleForm(c, entry, "/schedule/edit/"+entry.ID, "Редактирование назначения", "Сохранить изменения", true)
}

func UpdateScheduleEntry(c *gin.Context) {
	entry, err := storage.GetTimesheetByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Schedule entry not found")
		return
	}
	if c.GetString("userStatus") != "admin" {
		worker, err := storage.GetWorkerByUserID(c.GetString("userID"))
		if err != nil {
			c.String(http.StatusForbidden, "Нет привязанного работника")
			return
		}
		allowed := false
		for _, wid := range entry.WorkerIDs {
			if wid == worker.ID {
				allowed = true
				break
			}
		}
		if !allowed {
			c.String(http.StatusForbidden, "Доступ запрещен")
			return
		}
	}
	lunch, _ := strconv.Atoi(c.PostForm("lunch_break_minutes"))
	entry.Date = c.PostForm("date")
	entry.StartTime = c.PostForm("start_time")
	entry.EndTime = c.PostForm("end_time")
	entry.LunchBreakMinutes = lunch
	entry.WorkerIDs = cleanIDList(c.PostFormArray("worker_ids"))
	entry.ObjectIDs = cleanIDList(c.PostFormArray("object_ids"))
	entry.Notes = c.PostForm("notes")
	if c.GetString("userStatus") != "admin" {
		if worker, err := storage.GetWorkerByUserID(c.GetString("userID")); err == nil {
			entry.WorkerIDs = []string{worker.ID}
		}
		entry.UserMark = "Обновлено пользователем " + c.GetString("userName")
	}

	if err := validateScheduleLinks(entry.WorkerIDs, entry.ObjectIDs); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	if err := storage.UpdateTimesheet(entry); err != nil {
		c.String(http.StatusBadRequest, "Failed to update schedule entry: %v", err)
		return
	}
	returnTo := c.PostForm("return_to")
	if !strings.HasPrefix(returnTo, "/") {
		returnTo = "/schedule"
	}
	c.Redirect(http.StatusFound, returnTo)
}

func DeleteScheduleEntry(c *gin.Context) {
	entry, err := storage.GetTimesheetByID(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Schedule entry not found")
		return
	}
	if c.GetString("userStatus") != "admin" {
		worker, err := storage.GetWorkerByUserID(c.GetString("userID"))
		if err != nil {
			c.String(http.StatusForbidden, "Нет привязанного работника")
			return
		}
		allowed := false
		for _, wid := range entry.WorkerIDs {
			if wid == worker.ID {
				allowed = true
				break
			}
		}
		if !allowed {
			c.String(http.StatusForbidden, "Доступ запрещен")
			return
		}
	}
	if err := storage.DeleteTimesheet(c.Param("id")); err != nil {
		c.String(http.StatusBadRequest, "Failed to delete schedule entry: %v", err)
		return
	}
	returnTo := c.PostForm("return_to")
	if !strings.HasPrefix(returnTo, "/") {
		returnTo = "/schedule"
	}
	c.Redirect(http.StatusFound, returnTo)
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
				objects := joinMappedLinks(entry.ObjectIDs, objectsMap, "/object")
				creator := strings.TrimSpace(entry.CreatedByName)
				if creator == "" {
					creator = "—"
				}
				details = append(details, fmt.Sprintf("%s-%s · %s ч · %s · %s · создал: %s", entry.StartTime, entry.EndTime, hoursStr, objects, template.HTMLEscapeString(entry.Notes), template.HTMLEscapeString(creator)))
			}
			if len(details) == 0 {
				cells.WriteString(`<td class="hours-cell empty">—</td>`)
				continue
			}
			cells.WriteString(fmt.Sprintf(`<td class="hours-cell"><span>%.1f</span><div class="hours-tooltip">%s</div></td>`, total, strings.Join(details, "<br>")))
		}
		workerRows = append(workerRows, fmt.Sprintf(`<tr><th><a class="entity-link" href="/worker/%s">%s</a></th>%s</tr>`, template.HTMLEscapeString(worker.ID), template.HTMLEscapeString(worker.Name), cells.String()))
	}

	rows := strings.Join(workerRows, "")
	if rows == "" {
		rows = `<tr><td colspan="100%">Нет работников.</td></tr>`
	}

	page := `<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><title>Табель</title><link rel="stylesheet" href="/static/css/style.css"></head><body>
{{SIDEBAR_HTML}}
<div class="main-content">
<div class="page-header"><h1>Табель</h1><a class="btn btn-secondary" href="/schedule">К расписанию</a></div>
<div class="card timesheet-card">
  <form method="GET" action="/timesheets" class="month-selector">
    <label for="month">Месяц:</label>
    <select id="month" name="month" onchange="this.form.submit()">{{MONTH_OPTIONS}}</select>
  </form>
  <div class="table-scroll timesheet-table-wrap"><table class="table timesheet-matrix"><thead><tr><th>Работник</th>{{HEADERS}}</tr></thead><tbody>{{ROWS}}</tbody></table></div>
</div>
</div></body></html>`

	monthOptions := monthOptionsHTML(selectedMonth)
	final := strings.Replace(page, "{{SIDEBAR_HTML}}", RenderSidebar(c, "timesheets"), 1)
	final = strings.Replace(final, "{{MONTH_OPTIONS}}", monthOptions, 1)
	final = strings.Replace(final, "{{HEADERS}}", headers.String(), 1)
	final = strings.Replace(final, "{{ROWS}}", rows, 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(final))
}
