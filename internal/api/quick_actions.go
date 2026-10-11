package api

import (
	"bytes"
	"html/template"
	"sort"
	"strings"

	"project/internal/models"
)

// The dial lists owned vehicles, not the wider fleet an administrator can access.
func renderFloatingActions(userID, page string, vehicles []models.Vehicle) string {
	owned := make([]models.Vehicle, 0)
	for _, vehicle := range vehicles {
		if userID != "" && vehicle.AssignedUserID == userID {
			owned = append(owned, vehicle)
		}
	}
	sort.Slice(owned, func(i, j int) bool {
		a, b := strings.ToLower(owned[i].Name), strings.ToLower(owned[j].Name)
		if a != b {
			return a < b
		}
		return owned[i].RegistrationNumber < owned[j].RegistrationNumber
	})
	var output bytes.Buffer
	if err := floatingActionsTemplate.Execute(&output, struct {
		Vehicles []models.Vehicle
		Cloud    bool
	}{owned, page == "cloud"}); err != nil {
		return ""
	}
	return output.String()
}

var floatingActionsTemplate = template.Must(template.New("floating-actions").Parse(`
<div class="floating-create-wrap" {{if .Vehicles}}data-quick-actions{{end}}>
  {{if .Cloud}}<button type="button" class="floating-create-btn" id="cloud-floating-upload" data-quick-fallback aria-label="Загрузить файлы" title="Загрузить файлы">+</button>
  {{else}}<a class="floating-create-btn" data-quick-fallback href="/schedule/new" data-modal-url="/schedule/new" data-modal-title="Новое назначение" aria-label="Создать назначение" title="Создать назначение">+</a>{{end}}
  {{if .Vehicles}}
  <button type="button" id="quick-create-toggle" class="floating-create-btn" aria-label="Добавить" title="Добавить" aria-haspopup="menu" aria-expanded="false" aria-controls="quick-create-menu" hidden><span class="quick-icon quick-plus" aria-hidden="true"></span></button>
  <div id="quick-create-menu" class="quick-create-menu" role="menu" aria-label="Добавить" hidden>
    <a class="quick-action" role="menuitem" tabindex="-1" href="/schedule/new" data-modal-url="/schedule/new" data-modal-title="Новое назначение" data-modal-focus="#quick-create-toggle" title="Новое назначение"><span class="quick-action-label">Назначение</span><span class="quick-action-symbol"><span class="quick-icon quick-work" aria-hidden="true"></span></span></a>
    {{if eq (len .Vehicles) 1}}{{range .Vehicles}}<a class="quick-action" role="menuitem" tabindex="-1" href="/vehicles/{{.ID}}#vehicle-record-form" title="Новая запись: {{.Name}} · {{.RegistrationNumber}}"><span class="quick-action-label">Запись по авто</span><span class="quick-action-symbol"><span class="quick-icon quick-car" aria-hidden="true"></span></span></a>{{end}}
    {{else}}<button type="button" class="quick-action" role="menuitem" tabindex="-1" data-quick-vehicles aria-haspopup="dialog" aria-controls="quick-vehicle-picker" title="Выбрать авто"><span class="quick-action-label">Запись по авто</span><span class="quick-action-symbol"><span class="quick-icon quick-car" aria-hidden="true"></span></span></button>{{end}}
    {{if .Cloud}}<button type="button" class="quick-action" role="menuitem" tabindex="-1" data-quick-upload title="Загрузить файлы"><span class="quick-action-label">Загрузить файлы</span><span class="quick-action-symbol"><span class="quick-icon quick-upload" aria-hidden="true"></span></span></button>{{end}}
  </div>
  {{end}}
</div>
{{if gt (len .Vehicles) 1}}
<dialog id="quick-vehicle-picker" class="confirmation-dialog quick-vehicle-picker" aria-labelledby="quick-vehicle-title">
  <div class="modal-content"><header class="quick-picker-heading"><h2 id="quick-vehicle-title">Выберите авто</h2><button type="button" class="quick-picker-close" data-quick-close aria-label="Закрыть" title="Закрыть"><span class="quick-icon quick-close" aria-hidden="true"></span></button></header>
    <div class="quick-vehicle-list">{{range .Vehicles}}<a href="/vehicles/{{.ID}}#vehicle-record-form" class="quick-vehicle-choice"><span class="quick-icon quick-car" aria-hidden="true"></span><span><strong>{{.Name}}</strong><small>{{.RegistrationNumber}}</small></span><span class="quick-icon quick-next" aria-hidden="true"></span></a>{{end}}</div>
  </div>
</dialog>
{{end}}`))
