package api

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

type dashboardDayRollup struct {
	Date         time.Time
	Assignments  int
	SpecialMarks int
	Workers      map[string]struct{}
}

type dashboardObjectRollup struct {
	Name        string
	Assignments int
	Workers     map[string]struct{}
}

// DashboardPage renders the main dashboard page using manual HTML string building.
func DashboardPage(c *gin.Context) {
	if c.GetString("userStatus") != "admin" {
		c.Redirect(http.StatusFound, "/schedule")
		return
	}

	now := time.Now()
	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	today := todayDate.Format("2006-01-02")
	weekStart := todayDate.AddDate(0, 0, -((int(todayDate.Weekday()) + 6) % 7))
	weekEnd := weekStart.AddDate(0, 0, 6)
	weekStartKey := weekStart.Format("2006-01-02")
	weekEndKey := weekEnd.Format("2006-01-02")

	SetTopNavActions(c, `<div class="top-nav-toolbar"><span class="status-badge">`+template.HTMLEscapeString(todayDate.Format("02.01.2006"))+`</span><a class="btn btn-primary" href="/schedule/new" data-modal-url="/schedule/new" data-modal-title="Новое назначение" data-modal-return="/dashboard">Новая смена</a><a class="btn btn-secondary" href="/workers/new" data-modal-url="/workers/new" data-modal-title="Добавить работника" data-modal-return="/dashboard">Работник</a><a class="btn btn-secondary" href="/objects/new" data-modal-url="/objects/new" data-modal-title="Новый объект" data-modal-return="/dashboard">Объект</a></div>`)

	userName := strings.TrimSpace(c.GetString("userName"))
	if userName == "" {
		userName = "команда"
	}

	workers, _ := storage.GetWorkers()
	objects, _ := storage.GetObjects()
	entries, _ := storage.GetTimesheets()
	improvements, _ := storage.GetImprovements()
	workersMap, _ := buildWorkersMap()
	objectsMap, _ := buildObjectsMap()

	activeWorkers := 0
	firedWorkers := 0
	workersWithoutPhone := 0
	for _, worker := range workers {
		if worker.IsFired {
			firedWorkers++
			continue
		}
		activeWorkers++
		if strings.TrimSpace(worker.Phone) == "" {
			workersWithoutPhone++
		}
	}

	activeObjects := 0
	pausedObjects := 0
	completedObjects := 0
	for _, object := range objects {
		switch object.Status {
		case "completed":
			completedObjects++
		case "paused":
			pausedObjects++
		default:
			activeObjects++
		}
	}

	todayAssignments := 0
	weekAssignments := 0
	todayHours := 0.0
	todayWorkersSet := make(map[string]struct{})
	upcomingEntries := make([]string, 0, 5)
	todayObjects := make(map[string]*dashboardObjectRollup)
	nextSevenDays := make(map[string]*dashboardDayRollup, 7)
	dayOrder := make([]string, 0, 7)

	for i := 0; i < 7; i++ {
		day := todayDate.AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		nextSevenDays[key] = &dashboardDayRollup{
			Date:    day,
			Workers: make(map[string]struct{}),
		}
		dayOrder = append(dayOrder, key)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date == entries[j].Date {
			return entries[i].StartTime < entries[j].StartTime
		}
		return entries[i].Date < entries[j].Date
	})

	for _, entry := range entries {
		if entry.Date >= weekStartKey && entry.Date <= weekEndKey {
			weekAssignments++
		}

		if rollup, ok := nextSevenDays[entry.Date]; ok {
			rollup.Assignments++
			if isSpecialMark(entry.UserMark) {
				rollup.SpecialMarks++
			}
			for _, workerID := range entry.WorkerIDs {
				rollup.Workers[workerID] = struct{}{}
			}
		}

		if entry.Date == today {
			todayAssignments++
			for _, workerID := range entry.WorkerIDs {
				todayWorkersSet[workerID] = struct{}{}
			}
			if !isSpecialMark(entry.UserMark) {
				if hoursFloat, err := parseDashboardHours(formatWorkHours(entry.StartTime, entry.EndTime, entry.LunchBreakMinutes)); err == nil {
					todayHours += hoursFloat
				}
				for _, objectID := range entry.ObjectIDs {
					objectName := strings.TrimSpace(objectsMap[objectID])
					if objectName == "" {
						objectName = "Объект без названия"
					}
					rollup := todayObjects[objectID]
					if rollup == nil {
						rollup = &dashboardObjectRollup{
							Name:    objectName,
							Workers: make(map[string]struct{}),
						}
						todayObjects[objectID] = rollup
					}
					rollup.Assignments++
					for _, workerID := range entry.WorkerIDs {
						rollup.Workers[workerID] = struct{}{}
					}
				}
			}
		}

		if entry.Date < today || len(upcomingEntries) >= 5 {
			continue
		}

		bodyHTML := template.HTMLEscapeString(specialMarkTitle(entry.UserMark)) + ` · ` + joinMappedValues(entry.WorkerIDs, workersMap)
		if !isSpecialMark(entry.UserMark) {
			bodyHTML = template.HTMLEscapeString(entry.StartTime+"-"+entry.EndTime) + ` · ` + joinMappedValues(entry.WorkerIDs, workersMap) + ` · ` + joinMappedValues(entry.ObjectIDs, objectsMap)
		}

		upcomingEntries = append(
			upcomingEntries,
			renderDashboardListItem(formatScheduleDateLabel(entry.Date), bodyHTML),
		)
	}

	freeWorkersToday := activeWorkers - len(todayWorkersSet)
	if freeWorkersToday < 0 {
		freeWorkersToday = 0
	}

	openImprovements := 0
	doneToday := 0
	improvementsShown := 0
	sort.Slice(improvements, func(i, j int) bool {
		return improvements[i].CreatedAt.After(improvements[j].CreatedAt)
	})

	var improvementsHTML strings.Builder
	for _, item := range improvements {
		if item.Status != "done" {
			openImprovements++
		}
		if !item.DoneAt.IsZero() && item.DoneAt.Format("2006-01-02") == today {
			doneToday++
		}
		if improvementsShown >= 4 {
			continue
		}

		typeLabel := "Улучшение"
		if item.Type == "bug" {
			typeLabel = "Ошибка"
		}

		statusLabel := "Открыто"
		if item.Status == "done" {
			statusLabel = "Закрыто"
		}

		improvementsHTML.WriteString(
			renderDashboardListItem(
				item.Title,
				template.HTMLEscapeString(typeLabel+" · "+statusLabel),
			),
		)
		improvementsShown++
	}
	if improvementsHTML.Len() == 0 {
		improvementsHTML.WriteString(renderDashboardListItem("Пока пусто", "Сообщений об ошибках и улучшениях еще нет."))
	}

	var attentionHTML strings.Builder
	if todayAssignments == 0 {
		attentionHTML.WriteString(renderDashboardAlertItem("is-warning", "Сегодня нет назначений", "Проверьте расписание и добавьте смены, чтобы табель не оставался пустым."))
	}
	if pausedObjects > 0 {
		attentionHTML.WriteString(renderDashboardAlertItem("is-warning", fmt.Sprintf("Объекты на паузе: %d", pausedObjects), "Откройте объекты и решите, что возвращать в работу, а что завершать окончательно."))
	}
	if workersWithoutPhone > 0 {
		attentionHTML.WriteString(renderDashboardAlertItem("", fmt.Sprintf("Не хватает контактов: %d", workersWithoutPhone), "У части работников не указан телефон. Это неудобно для связи и быстрых перестроений."))
	}
	if freeWorkersToday > 0 && todayAssignments > 0 {
		attentionHTML.WriteString(renderDashboardAlertItem("", fmt.Sprintf("Свободный резерв сегодня: %d", freeWorkersToday), "Есть люди без загрузки на сегодня. Их можно быстро перекинуть на приоритетные задачи."))
	}
	if openImprovements > 0 {
		attentionHTML.WriteString(renderDashboardAlertItem("", fmt.Sprintf("Открытых обращений: %d", openImprovements), "В разделе улучшений ждут обработки новые идеи и баги от команды."))
	}
	if attentionHTML.Len() == 0 {
		attentionHTML.WriteString(renderDashboardAlertItem("is-success", "Критичных сигналов нет", "Сводка выглядит ровно: назначения есть, паузы под контролем, карточки заполнены."))
	}

	upcomingHTML := renderDashboardListItem("Пока пусто", "Ближайших назначений не найдено. Можно начать с нового назначения.")
	if len(upcomingEntries) > 0 {
		upcomingHTML = strings.Join(upcomingEntries, "")
	}

	type todayObjectRow struct {
		Name        string
		Assignments int
		Workers     int
	}

	objectRows := make([]todayObjectRow, 0, len(todayObjects))
	for _, object := range todayObjects {
		objectRows = append(objectRows, todayObjectRow{
			Name:        object.Name,
			Assignments: object.Assignments,
			Workers:     len(object.Workers),
		})
	}

	sort.Slice(objectRows, func(i, j int) bool {
		if objectRows[i].Assignments == objectRows[j].Assignments {
			if objectRows[i].Workers == objectRows[j].Workers {
				return objectRows[i].Name < objectRows[j].Name
			}
			return objectRows[i].Workers > objectRows[j].Workers
		}
		return objectRows[i].Assignments > objectRows[j].Assignments
	})

	var todayObjectsHTML strings.Builder
	if len(objectRows) == 0 {
		todayObjectsHTML.WriteString(renderDashboardListItem("Сегодня без объектов", "В графике на сегодня нет обычных смен по объектам."))
	} else {
		for i, row := range objectRows {
			if i >= 5 {
				break
			}
			todayObjectsHTML.WriteString(
				renderDashboardListItem(
					row.Name,
					template.HTMLEscapeString(fmt.Sprintf("Назначений: %d · Работников: %d", row.Assignments, row.Workers)),
				),
			)
		}
	}

	var weekPulseHTML strings.Builder
	for _, dayKey := range dayOrder {
		rollup := nextSevenDays[dayKey]
		weekPulseHTML.WriteString(renderDashboardDayChip(rollup.Date, rollup.Assignments, len(rollup.Workers), rollup.SpecialMarks, dayKey == today))
	}

	heroDescription := fmt.Sprintf(
		"Сегодня в сменах %d из %d работников, свободно %d. На паузе %d объектов, открыто %d обращений, а в плане на неделю уже %d назначений.",
		len(todayWorkersSet),
		activeWorkers,
		freeWorkersToday,
		pausedObjects,
		openImprovements,
		weekAssignments,
	)

	pageTemplate := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
    <title>Панель управления</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    {{SIDEBAR_HTML}}
    <div class="main-content">
        <div class="page-header">
            <div class="dashboard-hero">
                <div class="dashboard-copy">
                    <span class="text-muted">оперативная сводка</span>
                    <h1>Добро пожаловать, {{USER_NAME}}</h1>
                    <p>{{HERO_DESCRIPTION}}</p>
                </div>
                <div class="dashboard-cta dashboard-quick-actions">
                    <span class="status-badge">сегодня · {{TODAY_LABEL}}</span>
                    <a class="btn btn-secondary" href="/timesheets">Табель</a>
                    <a class="btn btn-secondary" href="/objects">Объекты</a>
                    <a class="btn btn-secondary" href="/improvements">Обратная связь</a>
                </div>
            </div>
        </div>

        <div class="workers-grid dashboard-stats">
            <a class="metric metric-link" href="/schedule"><div class="label">Назначений сегодня</div><div class="value">{{TODAY_ASSIGNMENTS}}</div><p>Сводка по текущему дню</p></a>
            <a class="metric metric-link" href="/schedule"><div class="label">Людей в сменах</div><div class="value">{{TODAY_WORKERS}}</div><p>Из {{ACTIVE_WORKERS}} работников в штате</p></a>
            <a class="metric metric-link" href="/workers"><div class="label">Свободный резерв</div><div class="value">{{FREE_WORKERS}}</div><p>Людей можно быстро подключить к задачам</p></a>
            <a class="metric metric-link" href="/timesheets"><div class="label">Часов на сегодня</div><div class="value">{{TODAY_HOURS}}</div><p>Плановая загрузка на день</p></a>
            <a class="metric metric-link" href="/schedule"><div class="label">Назначений за неделю</div><div class="value">{{WEEK_ASSIGNMENTS}}</div><p>{{WEEK_RANGE}}</p></a>
            <a class="metric metric-link" href="/objects"><div class="label">Объектов в работе</div><div class="value">{{ACTIVE_OBJECTS}}</div><p>{{PAUSED_OBJECTS}} на паузе</p></a>
            <a class="metric metric-link" href="/objects?tab=completed"><div class="label">Завершено объектов</div><div class="value">{{COMPLETED_OBJECTS}}</div><p>Архив выполненных площадок</p></a>
            <a class="metric metric-link" href="/improvements"><div class="label">Открытых замечаний</div><div class="value">{{OPEN_IMPROVEMENTS}}</div><p>Закрыто сегодня: {{DONE_TODAY}}</p></a>
        </div>

        <div class="profile-grid profile-grid-split">
            <div class="placeholder-card dashboard-panel">
                <div class="history-header">
                    <h2>Ближайшие назначения</h2>
                    <a class="btn btn-secondary btn-compact" href="/schedule">Открыть расписание</a>
                </div>
                <div class="dashboard-list">{{UPCOMING_ENTRIES}}</div>
            </div>
            <div class="placeholder-card dashboard-panel">
                <div class="history-header">
                    <h2>Требует внимания</h2>
                    <span class="status-badge">оперативный контроль</span>
                </div>
                <div class="dashboard-alerts">{{ATTENTION_ITEMS}}</div>
            </div>
        </div>

        <div class="compact-grid dashboard-panels">
            <div class="info-card">
                <div class="info-card-header">
                    <h2>Пульс на 7 дней</h2>
                    <span class="status-badge">неделя вперед</span>
                </div>
                <div class="dashboard-day-strip">{{DAY_PULSE}}</div>
            </div>
            <div class="info-card">
                <div class="info-card-header">
                    <h2>Сегодня по объектам</h2>
                    <a class="btn btn-secondary btn-compact" href="/schedule">К расписанию</a>
                </div>
                <div class="dashboard-list">{{TODAY_OBJECTS}}</div>
            </div>
        </div>

        <div class="compact-grid dashboard-panels">
            <div class="info-card">
                <div class="info-card-header">
                    <h2>Статус по объектам и команде</h2>
                    <a class="btn btn-secondary btn-compact" href="/workers">К работникам</a>
                </div>
                <div class="details-list">
                    <div class="detail-row"><span>Работников в штате</span><strong>{{ACTIVE_WORKERS}}</strong></div>
                    <div class="detail-row"><span>Уволено</span><strong>{{FIRED_WORKERS}}</strong></div>
                    <div class="detail-row"><span>Без телефона</span><strong>{{MISSING_PHONES}}</strong></div>
                    <div class="detail-row"><span>Свободно сегодня</span><strong>{{FREE_WORKERS}}</strong></div>
                    <div class="detail-row"><span>Активных объектов</span><strong>{{ACTIVE_OBJECTS}}</strong></div>
                    <div class="detail-row"><span>На паузе / завершено</span><strong>{{PAUSED_OBJECTS}} / {{COMPLETED_OBJECTS}}</strong></div>
                </div>
            </div>
            <div class="info-card">
                <div class="info-card-header">
                    <h2>Последние улучшения и баги</h2>
                    <a class="btn btn-secondary btn-compact" href="/improvements">Весь список</a>
                </div>
                <div class="dashboard-list">{{IMPROVEMENTS_FEED}}</div>
            </div>
        </div>
    </div>
</body>
</html>`

	sidebar := RenderSidebar(c, "dashboard")
	finalHTML := strings.Replace(pageTemplate, "{{SIDEBAR_HTML}}", sidebar, 1)
	finalHTML = strings.Replace(finalHTML, "{{USER_NAME}}", template.HTMLEscapeString(userName), 1)
	finalHTML = strings.Replace(finalHTML, "{{HERO_DESCRIPTION}}", template.HTMLEscapeString(heroDescription), 1)
	finalHTML = strings.Replace(finalHTML, "{{TODAY_LABEL}}", template.HTMLEscapeString(todayDate.Format("02.01.2006")), 1)
	finalHTML = strings.Replace(finalHTML, "{{TODAY_ASSIGNMENTS}}", fmt.Sprintf("%d", todayAssignments), 1)
	finalHTML = strings.Replace(finalHTML, "{{TODAY_WORKERS}}", fmt.Sprintf("%d", len(todayWorkersSet)), 1)
	finalHTML = strings.Replace(finalHTML, "{{FREE_WORKERS}}", fmt.Sprintf("%d", freeWorkersToday), -1)
	finalHTML = strings.Replace(finalHTML, "{{TODAY_HOURS}}", fmt.Sprintf("%.1f", todayHours), 1)
	finalHTML = strings.Replace(finalHTML, "{{ACTIVE_WORKERS}}", fmt.Sprintf("%d", activeWorkers), -1)
	finalHTML = strings.Replace(finalHTML, "{{FIRED_WORKERS}}", fmt.Sprintf("%d", firedWorkers), 1)
	finalHTML = strings.Replace(finalHTML, "{{ACTIVE_OBJECTS}}", fmt.Sprintf("%d", activeObjects), -1)
	finalHTML = strings.Replace(finalHTML, "{{PAUSED_OBJECTS}}", fmt.Sprintf("%d", pausedObjects), -1)
	finalHTML = strings.Replace(finalHTML, "{{COMPLETED_OBJECTS}}", fmt.Sprintf("%d", completedObjects), -1)
	finalHTML = strings.Replace(finalHTML, "{{WEEK_ASSIGNMENTS}}", fmt.Sprintf("%d", weekAssignments), 1)
	finalHTML = strings.Replace(finalHTML, "{{WEEK_RANGE}}", template.HTMLEscapeString(weekStart.Format("02.01")+" - "+weekEnd.Format("02.01")), 1)
	finalHTML = strings.Replace(finalHTML, "{{OPEN_IMPROVEMENTS}}", fmt.Sprintf("%d", openImprovements), 1)
	finalHTML = strings.Replace(finalHTML, "{{DONE_TODAY}}", fmt.Sprintf("%d", doneToday), 1)
	finalHTML = strings.Replace(finalHTML, "{{UPCOMING_ENTRIES}}", upcomingHTML, 1)
	finalHTML = strings.Replace(finalHTML, "{{ATTENTION_ITEMS}}", attentionHTML.String(), 1)
	finalHTML = strings.Replace(finalHTML, "{{DAY_PULSE}}", weekPulseHTML.String(), 1)
	finalHTML = strings.Replace(finalHTML, "{{TODAY_OBJECTS}}", todayObjectsHTML.String(), 1)
	finalHTML = strings.Replace(finalHTML, "{{MISSING_PHONES}}", fmt.Sprintf("%d", workersWithoutPhone), 1)
	finalHTML = strings.Replace(finalHTML, "{{IMPROVEMENTS_FEED}}", improvementsHTML.String(), 1)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(finalHTML))
}

func renderDashboardListItem(title, bodyHTML string) string {
	return `<div class="dashboard-list-item"><strong>` + template.HTMLEscapeString(title) + `</strong><p>` + bodyHTML + `</p></div>`
}

func renderDashboardAlertItem(modifierClass, title, body string) string {
	className := "dashboard-alert-item"
	if strings.TrimSpace(modifierClass) != "" {
		className += " " + strings.TrimSpace(modifierClass)
	}
	return `<div class="` + className + `"><strong>` + template.HTMLEscapeString(title) + `</strong><p>` + template.HTMLEscapeString(body) + `</p></div>`
}

func renderDashboardDayChip(day time.Time, assignments, workers, specialMarks int, isToday bool) string {
	className := "dashboard-day-chip"
	if isToday {
		className += " is-today"
	}

	weekdayNames := []string{"Вс", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб"}
	monthNames := []string{"янв", "фев", "мар", "апр", "мая", "июн", "июл", "авг", "сен", "окт", "ноя", "дек"}

	var body strings.Builder
	body.WriteString(`<div class="` + className + `">`)
	body.WriteString(`<small>` + template.HTMLEscapeString(weekdayNames[int(day.Weekday())]) + `</small>`)
	body.WriteString(`<strong>` + template.HTMLEscapeString(fmt.Sprintf("%d %s", day.Day(), monthNames[int(day.Month())-1])) + `</strong>`)
	body.WriteString(`<p>Назначений: ` + template.HTMLEscapeString(fmt.Sprintf("%d", assignments)) + `</p>`)
	body.WriteString(`<p>Работников: ` + template.HTMLEscapeString(fmt.Sprintf("%d", workers)) + `</p>`)
	if specialMarks > 0 {
		body.WriteString(`<p>Отметок: ` + template.HTMLEscapeString(fmt.Sprintf("%d", specialMarks)) + `</p>`)
	}
	body.WriteString(`</div>`)
	return body.String()
}

func parseDashboardHours(value string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(value), 64)
}
