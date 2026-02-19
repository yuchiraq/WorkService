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

// WorkersPage renders the list of workers with clickable cards.

func filterWorkers(workers []models.Worker, searchQuery, positionFilter string) []models.Worker {
	search := strings.ToLower(strings.TrimSpace(searchQuery))
	position := strings.TrimSpace(positionFilter)

	if search == "" && position == "" {
		return workers
	}

	filtered := make([]models.Worker, 0, len(workers))
	for _, worker := range workers {
		nameMatch := search == "" || strings.Contains(strings.ToLower(worker.Name), search)
		positionMatch := position == "" || worker.Position == position
		if nameMatch && positionMatch {
			filtered = append(filtered, worker)
		}
	}

	return filtered
}

func uniquePositions(workers []models.Worker) []string {
	positionsSet := make(map[string]struct{})
	positions := make([]string, 0)

	for _, worker := range workers {
		position := strings.TrimSpace(worker.Position)
		if position == "" {
			continue
		}
		if _, exists := positionsSet[position]; exists {
			continue
		}
		positionsSet[position] = struct{}{}
		positions = append(positions, position)
	}

	sort.Strings(positions)
	return positions
}

func WorkersPage(c *gin.Context) {
	workers, err := storage.GetWorkers()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load workers: %v", err)
		return
	}

	searchQuery := c.Query("q")
	selectedPosition := c.Query("position")
	selectedTab := c.DefaultQuery("tab", "active")

	scopedWorkers := make([]models.Worker, 0, len(workers))
	for _, worker := range workers {
		if selectedTab == "fired" {
			if worker.IsFired {
				scopedWorkers = append(scopedWorkers, worker)
			}
			continue
		}
		if !worker.IsFired {
			scopedWorkers = append(scopedWorkers, worker)
		}
	}

	filteredWorkers := filterWorkers(scopedWorkers, searchQuery, selectedPosition)
	positions := uniquePositions(scopedWorkers)

	var positionOptionsHTML strings.Builder
	for _, position := range positions {
		selectedAttr := ""
		if position == selectedPosition {
			selectedAttr = " selected"
		}
		positionOptionsHTML.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, template.HTMLEscapeString(position), selectedAttr, template.HTMLEscapeString(position)))
	}

	var workersGridHTML strings.Builder
	for _, worker := range filteredWorkers {
		runes := []rune(worker.Name)
		initials := ""
		if len(runes) > 1 {
			initials = string(runes[0:2])
		} else if len(runes) > 0 {
			initials = string(runes[0])
		}

		cardHTML := fmt.Sprintf(`
            <a href="/worker/%s" class="worker-card-link-wrapper">
                <div class="worker-card">
                    <div class="worker-card-header">
                        <div class="worker-avatar">%s</div>
                        <div class="worker-info">
                            <h3>%s</h3>
                            <p>%s</p>
                        </div>
                    </div>
                    <div class="worker-card-footer">
                        <span class="btn btn-secondary">Просмотр</span>
                    </div>
                </div>
            </a>`,
			template.HTMLEscapeString(worker.ID),
			strings.ToUpper(initials),
			template.HTMLEscapeString(worker.Name),
			template.HTMLEscapeString(worker.Position),
		)
		workersGridHTML.WriteString(cardHTML)
	}

	pageTemplate := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Работники</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    {{SIDEBAR_HTML}}
    <div class="main-content">
        <div class="page-header">
            <h1>Работники</h1>
            <a href="/workers/new" class="btn btn-primary" data-modal-url="/workers/new" data-modal-title="Добавить работника" data-modal-return="/workers">Добавить работника</a>
        </div>
        <div class="card">
            <p>Просмотр, добавление, редактирование или увольнение работников.</p>
            <div class="tab-switcher" style="margin-bottom:10px;display:flex;gap:8px;">
                <a class="btn btn-secondary{{TAB_ACTIVE_CLASS}}" href="/workers?tab=active">Текущие</a>
                <a class="btn btn-secondary{{TAB_FIRED_CLASS}}" href="/workers?tab=fired">Уволенные</a>
            </div>
            <button type="button" class="btn btn-secondary workers-search-toggle{{FILTER_TOGGLE_ACTIVE}}" data-search-toggle aria-expanded="{{FILTER_EXPANDED}}">Поиск и фильтры</button>
            <form action="/workers" method="GET" class="workers-filters{{FILTERS_CLASS}}" data-search-panel>
                <input type="hidden" name="tab" value="{{TAB}}">
                <div class="form-group">
                    <label for="q">Поиск по Ф.И.О.</label>
                    <input type="text" id="q" name="q" value="{{SEARCH_QUERY}}" placeholder="Например: Иванов">
                </div>
                <div class="form-group">
                    <label for="position">Фильтр по должности</label>
                    <select id="position" name="position">
                        <option value="">Все должности</option>
                        {{POSITION_OPTIONS}}
                    </select>
                </div>
                <div class="filter-actions">
                    <button type="submit" class="btn btn-primary">Применить</button>
                    <a href="/workers?tab={{TAB}}" class="btn btn-secondary">Сбросить</a>
                </div>
            </form>
            <p class="workers-summary">Найдено: <strong>{{FILTERED_COUNT}}</strong> из <strong>{{TOTAL_COUNT}}</strong>.</p>
            <div class="workers-grid">%s</div>
        </div>
    </div>

    <script>
      (function(){
        const toggle=document.querySelector('[data-search-toggle]');
        const panel=document.querySelector('[data-search-panel]');
        if(!toggle||!panel) return;
        toggle.addEventListener('click', function(){
          panel.classList.toggle('is-open');
          const expanded=panel.classList.contains('is-open');
          toggle.setAttribute('aria-expanded', expanded ? 'true' : 'false');
          toggle.classList.toggle('active', expanded);
        });
      })();
    </script>
</body>
</html>`

	// Build the final HTML by replacing placeholders
	sidebar := RenderSidebar(c, "workers")
	finalHTML := fmt.Sprintf(pageTemplate, workersGridHTML.String())
	finalHTML = strings.Replace(finalHTML, "{{SIDEBAR_HTML}}", sidebar, 1)
	finalHTML = strings.Replace(finalHTML, "{{SEARCH_QUERY}}", template.HTMLEscapeString(searchQuery), 1)
	finalHTML = strings.Replace(finalHTML, "{{POSITION_OPTIONS}}", positionOptionsHTML.String(), 1)
	filtersOpen := strings.TrimSpace(searchQuery) != "" || strings.TrimSpace(selectedPosition) != ""
	filtersClass := ""
	filterExpanded := "false"
	filterToggleActive := ""
	if filtersOpen {
		filtersClass = " is-open"
		filterExpanded = "true"
		filterToggleActive = " active"
	}
	finalHTML = strings.Replace(finalHTML, "{{FILTERS_CLASS}}", filtersClass, 1)
	finalHTML = strings.Replace(finalHTML, "{{FILTER_EXPANDED}}", filterExpanded, 1)
	finalHTML = strings.Replace(finalHTML, "{{FILTER_TOGGLE_ACTIVE}}", filterToggleActive, 1)
	finalHTML = strings.Replace(finalHTML, "{{FILTERED_COUNT}}", strconv.Itoa(len(filteredWorkers)), 1)
	finalHTML = strings.Replace(finalHTML, "{{TOTAL_COUNT}}", strconv.Itoa(len(scopedWorkers)), 1)
	finalHTML = strings.Replace(finalHTML, "{{TAB}}", template.HTMLEscapeString(selectedTab), -1)
	tabActiveClass := ""
	tabFiredClass := ""
	if selectedTab == "fired" {
		tabFiredClass = " active"
	} else {
		tabActiveClass = " active"
	}
	finalHTML = strings.Replace(finalHTML, "{{TAB_ACTIVE_CLASS}}", tabActiveClass, 1)
	finalHTML = strings.Replace(finalHTML, "{{TAB_FIRED_CLASS}}", tabFiredClass, 1)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(finalHTML))
}

// WorkerProfilePage displays a single worker's profile.
func WorkerProfilePage(c *gin.Context) {
	workerID := c.Param("id")
	worker, err := storage.GetWorkerByID(workerID)
	if err != nil {
		c.String(http.StatusNotFound, "Worker not found: %v", err)
		return
	}

	runes := []rune(worker.Name)
	initials := ""
	if len(runes) > 1 {
		initials = string(runes[0:2])
	} else if len(runes) > 0 {
		initials = string(runes[0])
	}

	selectedMonth := c.Query("month")
	if selectedMonth == "" {
		selectedMonth = time.Now().Format("2006-01")
	}

	objectsMap, _ := buildObjectsMap()
	entries, _ := storage.GetTimesheets()
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date == entries[j].Date {
			return entries[i].StartTime < entries[j].StartTime
		}
		return entries[i].Date > entries[j].Date
	})

	totalHours := 0.0
	var workerAssignments strings.Builder
	currentDate := ""
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Date, selectedMonth+"-") {
			continue
		}
		matched := false
		for _, wid := range entry.WorkerIDs {
			if wid == worker.ID {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		hoursVal, _ := strconv.ParseFloat(formatWorkHours(entry.StartTime, entry.EndTime, entry.LunchBreakMinutes), 64)
		totalHours += hoursVal
		if entry.Date != currentDate {
			if currentDate != "" {
				workerAssignments.WriteString(`</div></div>`)
			}
			currentDate = entry.Date
			workerAssignments.WriteString(fmt.Sprintf(`<div class="schedule-day-group"><h3>%s</h3><div class="schedule-day-list">`, template.HTMLEscapeString(formatScheduleDateLabel(entry.Date))))
		}
		commentHTML := ""
		if strings.TrimSpace(entry.Notes) != "" {
			commentHTML = `<div class="assignment-note"><span>Комментарий</span><p>` + template.HTMLEscapeString(entry.Notes) + `</p></div>`
		}
		creatorHTML := ""
		if strings.TrimSpace(entry.CreatedByName) != "" {
			creatorHTML = `<div class="assignment-meta"><span>Создал</span><p>` + template.HTMLEscapeString(entry.CreatedByName) + `</p></div>`
		}
		workerAssignments.WriteString(fmt.Sprintf(`<div class="schedule-entry-vertical structured-assignment"><div class="assignment-head"><strong>%s — %s</strong><span>%.2f ч</span></div><div class="assignment-body"><div class="assignment-meta"><span>Объекты</span><p>%s</p></div>%s%s</div></div>`, template.HTMLEscapeString(entry.StartTime), template.HTMLEscapeString(entry.EndTime), hoursVal, joinMappedLinks(entry.ObjectIDs, objectsMap, "/object"), creatorHTML, commentHTML))
	}
	if workerAssignments.Len() == 0 {
		workerAssignments.WriteString(`<p>Назначений за выбранный месяц нет.</p>`)
	} else {
		workerAssignments.WriteString(`</div></div>`)
	}

	monthNames := []string{"Январь", "Февраль", "Март", "Апрель", "Май", "Июнь", "Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь"}
	var workerMonthOptions strings.Builder
	for i := -12; i <= 12; i++ {
		m := time.Now().AddDate(0, i, 0)
		val := m.Format("2006-01")
		sel := ""
		if val == selectedMonth {
			sel = " selected"
		}
		label := fmt.Sprintf("%s %d", monthNames[int(m.Month())-1], m.Year())
		workerMonthOptions.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, template.HTMLEscapeString(val), sel, template.HTMLEscapeString(label)))
	}

	formattedBirthDate := "Не указана"
	if worker.BirthDate != "" {
		parts := strings.Split(worker.BirthDate, "-")
		if len(parts) == 3 {
			formattedBirthDate = fmt.Sprintf("%s.%s.%s", parts[2], parts[1], parts[0])
		}
	}

	pageTemplate := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Профиль: {{WORKER_NAME}}</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    {{SIDEBAR_HTML}}
    <div class="main-content"> 
        <a href="/workers" class="back-link"><svg viewBox="0 0 16 16"><path d="M11.9,8.5H4.1l3.3,3.3c0.2,0.2,0.2,0.5,0,0.7s-0.5,0.2-0.7,0l-4-4C2.6,8.4,2.5,8.2,2.5,8s0.1-0.4,0.2-0.5l4-4c0.2-0.2,0.5-0.2,0.7,0s0.2,0.5,0,0.7L4.1,7.5H11.9c0.3,0,0.5,0.2,0.5,0.5S12.2,8.5,11.9,8.5z"/></svg>К списку работников</a>

        <div class="profile-header-container">
            <div class="profile-header">
                <div class="worker-avatar">{{INITIALS}}</div>
                <div class="profile-header-info">
                    <h1>Профиль: {{WORKER_NAME}}</h1>
                    <p>{{POSITION}}</p>
                </div>
            </div>
            <div class="profile-actions">
                {{STATUS_BADGE}}
                <a href="/workers/edit/{{WORKER_ID}}" class="btn btn-secondary" data-modal-url="/workers/edit/{{WORKER_ID}}" data-modal-title="Редактировать работника" data-modal-return="/worker/{{WORKER_ID}}">Редактировать</a>
            </div>
        </div>

        <ul class="profile-details">
            <li><svg fill="currentColor" viewBox="0 0 20 20"><path d="M6 8V7a4 4 0 118 0v1h2V7a6 6 0 10-12 0v1h2zm6 2H8v6h4v-6z"/></svg>Дата рождения: {{BIRTH_DATE}}</li>
            <li><svg fill="currentColor" viewBox="0 0 20 20"><path d="M2 3a1 1 0 011-1h2.153a1 1 0 01.986.836l.74 4.435a1 1 0 01-.54 1.06l-1.548.773a11.037 11.037 0 006.105 6.105l.774-1.548a1 1 0 011.059-.54l4.435.74a1 1 0 01.836.986V17a1 1 0 01-1 1h-2C7.82 18 2 12.18 2 5V3z"/></svg>{{PHONE}}</li>
            <li><svg fill="currentColor" viewBox="0 0 20 20"><path d="M10 2a8 8 0 100 16 8 8 0 000-16zm1 11a1 1 0 11-2 0v-2a1 1 0 112 0v2zm-1-4a1 1 0 01-1-1V7a1 1 0 112 0v1a1 1 0 01-1 1z"/></svg>Ставка: {{RATE}} руб/час</li>
        </ul>


        <div class="profile-grid">
            <div class="placeholder-card">
                 <div class="history-header"><h2>История назначений</h2></div>
                 <form method="GET" action="/worker/{{WORKER_ID}}" class="month-selector"><label for="month">Месяц:</label><select id="month" name="month" onchange="this.form.submit()">{{MONTH_OPTIONS}}</select><span><strong>Итого часов:</strong> {{TOTAL_HOURS}}</span></form>
                 <div class="schedule-vertical">{{ASSIGNMENTS_BY_DAY}}</div>
            </div>
        </div>
    </div>
</body>
</html>`

	// Build the final HTML by replacing placeholders
	sidebar := RenderSidebar(c, "workers")
	finalHTML := pageTemplate
	finalHTML = strings.Replace(finalHTML, "{{SIDEBAR_HTML}}", sidebar, -1)
	finalHTML = strings.Replace(finalHTML, "{{WORKER_NAME}}", template.HTMLEscapeString(worker.Name), -1)
	finalHTML = strings.Replace(finalHTML, "{{INITIALS}}", template.HTMLEscapeString(strings.ToUpper(initials)), -1)
	finalHTML = strings.Replace(finalHTML, "{{POSITION}}", template.HTMLEscapeString(worker.Position), -1)
	finalHTML = strings.Replace(finalHTML, "{{WORKER_ID}}", template.HTMLEscapeString(worker.ID), -1)
	finalHTML = strings.Replace(finalHTML, "{{CSRF_FIELD}}", CSRFHiddenInput(c), -1)
	finalHTML = strings.Replace(finalHTML, "{{BIRTH_DATE}}", template.HTMLEscapeString(formattedBirthDate), -1)
	finalHTML = strings.Replace(finalHTML, "{{PHONE}}", template.HTMLEscapeString(worker.Phone), -1)
	finalHTML = strings.Replace(finalHTML, "{{RATE}}", fmt.Sprintf("%.2f", worker.HourlyRate), -1)
	statusBadge := `<div class="status-badge active"><svg viewBox="0 0 16 16"><path d="M8,0C3.6,0,0,3.6,0,8s3.6,8,8,8s8-3.6,8-8S12.4,0,8,0z M7,11.4L3.6,8L5,6.6l2,2l4-4L12.4,6L7,11.4z"/></svg>Активен</div>`
	if worker.IsFired {
		statusBadge = `<div class="status-badge" style="background:#ffe9e9;color:#b42318;">Уволен</div>`
	}
	finalHTML = strings.Replace(finalHTML, "{{STATUS_BADGE}}", statusBadge, -1)
	finalHTML = strings.Replace(finalHTML, "{{MONTH_OPTIONS}}", workerMonthOptions.String(), -1)
	finalHTML = strings.Replace(finalHTML, "{{TOTAL_HOURS}}", fmt.Sprintf("%.2f", totalHours), -1)
	finalHTML = strings.Replace(finalHTML, "{{ASSIGNMENTS_BY_DAY}}", workerAssignments.String(), -1)
	finalHTML = strings.Replace(finalHTML, "{{ASSIGNMENTS_SECTION}}", "", -1)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(finalHTML))
}

// AddWorkerPage renders the page with a form to add a new worker.
func AddWorkerPage(c *gin.Context) {
	pageTemplate := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Добавить работника</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    {{SIDEBAR_HTML}}
    <div class="main-content">
        <a href="/workers" class="back-link"><svg viewBox="0 0 16 16"><path d="M11.9,8.5H4.1l3.3,3.3c0.2,0.2,0.2,0.5,0,0.7s-0.5,0.2-0.7,0l-4-4C2.6,8.4,2.5,8.2,2.5,8s0.1-0.4,0.2-0.5l4-4c0.2-0.2,0.5-0.2,0.7,0s0.2,0.5,0,0.7L4.1,7.5H11.9c0.3,0,0.5,0.2,0.5,0.5S12.2,8.5,11.9,8.5z"/></svg>К списку работников</a>
        <div class="page-header">
            <h1>Новый работник</h1>
        </div>
        <div class="card">
            <p>Заполните все поля для регистрации нового работника в системе.</p>
            <form action="/workers/new" method="POST">
                {{CSRF_FIELD}}
                <div class="form-grid">
                    <div class="form-group">
                        <label for="name">Ф.И.О.</label>
                        <input type="text" id="name" name="name" required>
                    </div>
                    <div class="form-group">
                        <label for="position">Должность</label>
                        <input type="text" id="position" name="position" required>
                    </div>
                    <div class="form-group">
                        <label for="phone">Телефон</label>
                        <input type="tel" id="phone" name="phone">
                    </div>
                    <div class="form-group">
                        <label for="birth_date">Дата рождения</label>
                        <input type="date" id="birth_date" name="birth_date">
                    </div>
                    <div class="form-group">
                        <label for="hourly_rate">Ставка (руб/час)</label>
                        <input type="number" id="hourly_rate" name="hourly_rate" step="0.01">
                    </div>
                </div>
                <div class="form-actions">
                    <button type="submit" class="btn btn-primary">Сохранить</button>
                    <a href="/workers" class="btn btn-secondary">Отмена</a>
                </div>
            </form>
        </div>
    </div>
</body>
</html>`

	sidebar := RenderSidebar(c, "workers")
	finalHTML := strings.Replace(pageTemplate, "{{SIDEBAR_HTML}}", sidebar, 1)
	finalHTML = strings.Replace(finalHTML, "{{CSRF_FIELD}}", CSRFHiddenInput(c), -1)
	if IsModalRequest(c) {
		finalHTML = strings.Replace(finalHTML, sidebar, "", 1)
		finalHTML = strings.Replace(finalHTML, `<body>`, `<body><div class="modal-form-layout">`, 1)
		finalHTML = strings.Replace(finalHTML, `<a href="/workers" class="back-link"><svg viewBox="0 0 16 16"><path d="M11.9,8.5H4.1l3.3,3.3c0.2,0.2,0.2,0.5,0,0.7s-0.5,0.2-0.7,0l-4-4C2.6,8.4,2.5,8.2,2.5,8s0.1-0.4,0.2-0.5l4-4c0.2-0.2,0.5-0.2,0.7,0s0.2,0.5,0,0.7L4.1,7.5H11.9c0.3,0,0.5,0.2,0.5,0.5S12.2,8.5,11.9,8.5z"/></svg>К списку работников</a>`, "", 1)
		finalHTML = strings.Replace(finalHTML, `<div class="main-content">`, `<div class="main-content modal-form-content">`, 1)
		finalHTML = strings.Replace(finalHTML, `<div class="card">`, `<div class="card modal-form-card">`, 1)
		finalHTML = strings.Replace(finalHTML, `</body>`, `</div></body>`, 1)
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(finalHTML))
}

// CreateWorker handles the creation of a new worker.
func CreateWorker(c *gin.Context) {
	rate := 0.0
	rateRaw := strings.TrimSpace(c.PostForm("hourly_rate"))
	if rateRaw != "" {
		parsedRate, err := strconv.ParseFloat(rateRaw, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid hourly rate: %v", err)
			return
		}
		rate = parsedRate
	}

	userID, _ := c.Get("userID")
	userName, _ := c.Get("userName")

	newWorker := models.Worker{
		Name:          c.PostForm("name"),
		Position:      c.PostForm("position"),
		Phone:         c.PostForm("phone"),
		BirthDate:     c.PostForm("birth_date"),
		HourlyRate:    rate,
		CreatedBy:     userID.(string),
		CreatedByName: userName.(string),
	}

	_, err := storage.CreateWorker(newWorker)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to create worker: %v", err)
		return
	}

	c.Redirect(http.StatusFound, "/workers")
}

// EditWorkerPage renders the page for editing an existing worker.
func EditWorkerPage(c *gin.Context) {
	workerID := c.Param("id")
	worker, err := storage.GetWorkerByID(workerID)
	if err != nil {
		c.String(http.StatusNotFound, "Worker not found: %v", err)
		return
	}

	pageTemplate := `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Редактировать профиль</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    {{SIDEBAR_HTML}}
    <div class="main-content">
         <a href="/worker/{{WORKER_ID}}" class="back-link"><svg viewBox="0 0 16 16"><path d="M11.9,8.5H4.1l3.3,3.3c0.2,0.2,0.2,0.5,0,0.7s-0.5,0.2-0.7,0l-4-4C2.6,8.4,2.5,8.2,2.5,8s0.1-0.4,0.2-0.5l4-4c0.2-0.2,0.5-0.2,0.7,0s0.2,0.5,0,0.7L4.1,7.5H11.9c0.3,0,0.5,0.2,0.5,0.5S12.2,8.5,11.9,8.5z"/></svg>Назад к профилю</a>
        <div class="page-header">
            <h1>Редактировать профиль</h1>
        </div>
        <div class="card">
            <p>Здесь можно обновить информацию о работнике.</p>
            <form action="/workers/edit/{{WORKER_ID}}" method="POST" class="form-grid-edit">
{{CSRF_FIELD}}

                <div class="form-group-edit form-group-name">
                    <label for="name">Ф.И.О.</label>
                    <input type="text" id="name" name="name" value="{{WORKER_NAME}}" required>
                </div>

                <div class="form-group-edit form-group-position">
                    <label for="position">Должность</label>
                    <input type="text" id="position" name="position" value="{{POSITION}}" required>
                </div>

                <div class="form-group-edit form-group-phone">
                    <label for="phone">Телефон</label>
                    <input type="tel" id="phone" name="phone" value="{{PHONE}}">
                </div>

                <div class="form-group-edit form-group-birthdate">
                    <label for="birth_date">Дата рождения</label>
                    <input type="date" id="birth_date" name="birth_date" value="{{BIRTH_DATE}}">
                </div>

                <div class="form-group-edit form-group-rate">
                    <label for="hourly_rate">Ставка (руб/час)</label>
                    <input type="number" id="hourly_rate" name="hourly_rate" value="{{RATE}}" step="0.01">
                </div>

                <div class="form-actions-edit">
                    <button type="submit" class="btn btn-primary">Сохранить изменения</button>
                    <a href="/worker/{{WORKER_ID}}" class="btn btn-secondary">Отмена</a>
                    <button type="button" class="btn btn-danger" onclick="showDeleteModal()">Уволить</button>
                </div>
            </form>
        </div>
    </div>

    <!-- Modal for delete confirmation -->
    <div id="deleteModal" class="modal">
        <div class="modal-content">
            <span class="close-button" onclick="closeDeleteModal()">&times;</span>
            <h2>Подтверждение увольнения</h2>
            <p>Вы уверены, что хотите уволить этого работника? Это действие нельзя будет отменить.</p>
            <form action="/workers/delete/{{WORKER_ID}}" method="POST">
                {{CSRF_FIELD}}
                <div class="form-actions">
                    <button type="submit" class="btn btn-danger">Да, уволить</button>
                    <button type="button" class="btn btn-secondary" onclick="closeDeleteModal()">Отмена</button>
                </div>
            </form>
        </div>
    </div>

    <script>
        function showDeleteModal() {
            document.getElementById('deleteModal').style.display = 'block';
        }
        function closeDeleteModal() {
            document.getElementById('deleteModal').style.display = 'none';
        }
        // Close modal if user clicks outside of it
        window.onclick = function(event) {
            if (event.target == document.getElementById('deleteModal')) {
                closeDeleteModal();
            }
        }
    </script>

</body>
</html>`

	sidebar := RenderSidebar(c, "workers")
	finalHTML := pageTemplate
	finalHTML = strings.Replace(finalHTML, "{{SIDEBAR_HTML}}", sidebar, -1)
	finalHTML = strings.Replace(finalHTML, "{{WORKER_ID}}", template.HTMLEscapeString(worker.ID), -1)
	finalHTML = strings.Replace(finalHTML, "{{CSRF_FIELD}}", CSRFHiddenInput(c), -1)
	finalHTML = strings.Replace(finalHTML, "{{WORKER_NAME}}", template.HTMLEscapeString(worker.Name), -1)
	finalHTML = strings.Replace(finalHTML, "{{POSITION}}", template.HTMLEscapeString(worker.Position), -1)
	finalHTML = strings.Replace(finalHTML, "{{PHONE}}", template.HTMLEscapeString(worker.Phone), -1)
	finalHTML = strings.Replace(finalHTML, "{{BIRTH_DATE}}", template.HTMLEscapeString(worker.BirthDate), -1)
	finalHTML = strings.Replace(finalHTML, "{{RATE}}", fmt.Sprintf("%.2f", worker.HourlyRate), -1)
	statusBadge := `<div class="status-badge active"><svg viewBox="0 0 16 16"><path d="M8,0C3.6,0,0,3.6,0,8s3.6,8,8,8s8-3.6,8-8S12.4,0,8,0z M7,11.4L3.6,8L5,6.6l2,2l4-4L12.4,6L7,11.4z"/></svg>Активен</div>`
	if worker.IsFired {
		statusBadge = `<div class="status-badge" style="background:#ffe9e9;color:#b42318;">Уволен</div>`
	}
	finalHTML = strings.Replace(finalHTML, "{{STATUS_BADGE}}", statusBadge, -1)
	if IsModalRequest(c) {
		finalHTML = strings.Replace(finalHTML, sidebar, "", 1)
		finalHTML = strings.Replace(finalHTML, `<body>`, `<body><div class="modal-form-layout">`, 1)
		finalHTML = strings.Replace(finalHTML, `<a href="/worker/{{WORKER_ID}}" class="back-link"><svg viewBox="0 0 16 16"><path d="M11.9,8.5H4.1l3.3,3.3c0.2,0.2,0.2,0.5,0,0.7s-0.5,0.2-0.7,0l-4-4C2.6,8.4,2.5,8.2,2.5,8s0.1-0.4,0.2-0.5l4-4c0.2-0.2,0.5-0.2,0.7,0s0.2,0.5,0,0.7L4.1,7.5H11.9c0.3,0,0.5,0.2,0.5,0.5S12.2,8.5,11.9,8.5z"/></svg>Назад к профилю</a>`, "", 1)
		finalHTML = strings.Replace(finalHTML, `<div class="main-content">`, `<div class="main-content modal-form-content">`, 1)
		finalHTML = strings.Replace(finalHTML, `<div class="card">`, `<div class="card modal-form-card">`, 1)
		finalHTML = strings.Replace(finalHTML, `</body>`, `</div></body>`, 1)
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(finalHTML))
}

// UpdateWorker handles the update of an existing worker's details.
func UpdateWorker(c *gin.Context) {
	workerID := c.Param("id")

	worker, err := storage.GetWorkerByID(workerID)
	if err != nil {
		c.String(http.StatusNotFound, "Worker not found: %v", err)
		return
	}

	rate := 0.0
	rateRaw := strings.TrimSpace(c.PostForm("hourly_rate"))
	if rateRaw != "" {
		parsedRate, err := strconv.ParseFloat(rateRaw, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid hourly rate: %v", err)
			return
		}
		rate = parsedRate
	}

	// Update fields from the form
	worker.Name = c.PostForm("name")
	worker.Position = c.PostForm("position")
	worker.Phone = c.PostForm("phone")
	worker.BirthDate = c.PostForm("birth_date")
	worker.HourlyRate = rate

	if err := storage.UpdateWorker(worker); err != nil {
		c.String(http.StatusInternalServerError, "Failed to save updated worker data: %v", err)
		return
	}

	c.Redirect(http.StatusFound, "/worker/"+workerID)
}

// DeleteWorker handles the deletion of a worker.
func DeleteWorker(c *gin.Context) {
	workerID := c.Param("id")

	if err := storage.DeleteWorker(workerID); err != nil {
		c.String(http.StatusInternalServerError, "Failed to delete worker: %v", err)
		return
	}

	c.Redirect(http.StatusFound, "/workers?tab=fired")
}
